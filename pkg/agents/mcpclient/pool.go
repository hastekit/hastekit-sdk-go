package mcpclient

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	// globalPool is the package-level connection pool shared across all MCPClient instances.
	// Since temporal/restate workers are long-lived processes, this pool is shared across
	// all activity/handler executions on the same worker.
	globalPool = newConnectionPool(5 * time.Minute)
)

// SchemaCache defines the interface for caching MCP tool schemas.
// Implementations can use Redis, in-memory stores, or any other backing store.
type SchemaCache interface {
	// Get retrieves cached tool schemas by key. Returns nil, false on cache miss.
	Get(ctx context.Context, key string) (*CachedToolEntry, bool)
	// Set stores tool schemas with the given key.
	Set(ctx context.Context, key string, entry *CachedToolEntry)
	// Delete removes a cached entry by key.
	Delete(ctx context.Context, key string)
	// Clear removes all cached entries.
	Clear(ctx context.Context)
}

// CachedToolEntry stores cached MCP tool schemas.
type CachedToolEntry struct {
	Tools []mcp.Tool `json:"tools"`
	Meta  *mcp.Meta  `json:"meta,omitempty"`
}

// poolEntry holds a live MCP connection.
type poolEntry struct {
	client   *client.Client
	lastUsed time.Time
	mu       sync.Mutex
}

// connectionPool manages reusable MCP connections keyed by endpoint+transport+headers.
type connectionPool struct {
	mu          sync.RWMutex
	connections map[string]*poolEntry
	idleTimeout time.Duration
	stopCleanup chan struct{}
	stopOnce    sync.Once
}

func newConnectionPool(idleTimeout time.Duration) *connectionPool {
	p := &connectionPool{
		connections: make(map[string]*poolEntry),
		idleTimeout: idleTimeout,
		stopCleanup: make(chan struct{}),
	}
	go p.cleanupLoop()
	return p
}

// poolKey generates a key for the connection pool.
// Uses endpoint + transport + sorted headers (without tool filters, since connections are server-level).
func poolKey(endpoint, transportType string, headers map[string]string) string {
	return fmt.Sprintf("%s|%s|%s", endpoint, transportType, sortedHeadersString(headers))
}

// Checkout returns an existing healthy connection or creates a new one.
// The mcp-go SSE client supports concurrent CallTool calls via JSON-RPC request IDs,
// so a single connection per server is sufficient.
func (p *connectionPool) Checkout(ctx context.Context, endpoint, transportType string, headers map[string]string) (*client.Client, error) {
	key := poolKey(endpoint, transportType, headers)

	p.mu.RLock()
	entry, exists := p.connections[key]
	p.mu.RUnlock()

	if exists {
		entry.mu.Lock()
		entry.lastUsed = time.Now()
		cli := entry.client
		entry.mu.Unlock()

		if cli != nil {
			return cli, nil
		}
	}

	// Create new connection
	cli, err := createConnection(ctx, endpoint, transportType, headers)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.connections[key] = &poolEntry{
		client:   cli,
		lastUsed: time.Now(),
	}
	p.mu.Unlock()

	return cli, nil
}

// Remove removes a connection from the pool (e.g., when it's known to be dead).
func (p *connectionPool) Remove(endpoint, transportType string, headers map[string]string) {
	key := poolKey(endpoint, transportType, headers)
	p.mu.Lock()
	if entry, ok := p.connections[key]; ok {
		entry.mu.Lock()
		if entry.client != nil {
			entry.client.Close()
		}
		entry.mu.Unlock()
		delete(p.connections, key)
	}
	p.mu.Unlock()
}

func (p *connectionPool) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdle()
		case <-p.stopCleanup:
			return
		}
	}
}

func (p *connectionPool) cleanupIdle() {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, entry := range p.connections {
		entry.mu.Lock()
		if now.Sub(entry.lastUsed) > p.idleTimeout {
			if entry.client != nil {
				entry.client.Close()
			}
			delete(p.connections, key)
			slog.Debug("MCP connection pool: closed idle connection", slog.String("key", key))
		}
		entry.mu.Unlock()
	}
}

// Close closes all connections and stops the cleanup goroutine.
func (p *connectionPool) Close() {
	p.stopOnce.Do(func() {
		close(p.stopCleanup)
		p.mu.Lock()
		defer p.mu.Unlock()
		for key, entry := range p.connections {
			entry.mu.Lock()
			if entry.client != nil {
				entry.client.Close()
			}
			entry.mu.Unlock()
			delete(p.connections, key)
		}
	})
}

// createConnection establishes a new MCP connection (Start + Initialize, no ListTools).
func createConnection(ctx context.Context, endpoint, transportType string, headers map[string]string) (*client.Client, error) {
	var cli *client.Client
	var err error

	switch transportType {
	case "sse":
		cli, err = client.NewSSEMCPClient(endpoint, client.WithHeaders(headers))
	case "streamable-http":
		cli, err = client.NewStreamableHttpClient(endpoint, transport.WithHTTPHeaders(headers))
	default:
		cli, err = client.NewSSEMCPClient(endpoint, client.WithHeaders(headers))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	if err = cli.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	if _, err = cli.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{},
		Params: mcp.InitializeParams{
			ProtocolVersion: "2025-06-18",
		},
	}); err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return cli, nil
}

// sortedHeadersString produces a deterministic string from headers for use as cache/pool key.
func sortedHeadersString(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	result := ""
	for _, k := range keys {
		result += k + "=" + headers[k] + ";"
	}
	return result
}
