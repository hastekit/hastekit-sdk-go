package streambroker

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/redis/go-redis/v9"
)

// RedisStreamBroker implements StreamBroker using Redis Streams.
// Each channel (typically a stream id) maps to one Redis Stream that
// persists every chunk emitted for it. Subscribers always receive the
// full transcript from the first event and then tail any remaining
// events until Close is called. This makes streams rejoinable: a
// client that drops mid-stream can reconnect to the same channel and
// receive the complete transcript.
//
// Redis Streams give us:
//   - Durable event log with MAXLEN cap to bound memory
//   - Multiple independent readers per channel (no coordination needed)
//   - TTL-based automatic cleanup after a stream terminates
type RedisStreamBroker struct {
	client    *redis.Client
	prefix    string
	activeTTL time.Duration
	replayTTL time.Duration
	maxLen    int64
	readCount int64
	blockTime time.Duration
}

// RedisStreamBrokerOptions configures the Redis stream broker.
type RedisStreamBrokerOptions struct {
	// Addr is the Redis server address (e.g., "localhost:6379").
	Addr string

	// Password is the Redis password (optional).
	Password string

	// DB is the Redis database number (default 0).
	DB int

	// Prefix is prepended to all channel names (default "uno:stream:").
	// This allows multiple applications to share the same Redis instance.
	Prefix string

	// Client is an existing Redis client to use instead of creating a new one.
	// If provided, Addr/Password/DB are ignored.
	Client *redis.Client

	// ActiveTTL bounds the lifetime of a channel's stream while it is
	// still active. Default 30 minutes.
	ActiveTTL time.Duration

	// ReplayTTL is the rejoin window applied after Close. Default 10 minutes.
	ReplayTTL time.Duration

	// MaxLen caps the approximate number of entries retained per stream
	// (XADD MAXLEN ~). Default 2000.
	MaxLen int64
}

const (
	defaultActiveTTL = 30 * time.Minute
	defaultReplayTTL = 10 * time.Minute
	defaultMaxLen    = int64(2000)
	defaultReadCount = int64(500)
	defaultBlock     = 5 * time.Second

	// streamEndType is the sentinel event type written by Close so
	// Subscribe loops can terminate without relying on a status key.
	streamEndType = "__stream_end"
)

// NewRedisStreamBroker creates a new Redis-backed stream broker.
func NewRedisStreamBroker(opts RedisStreamBrokerOptions) (*RedisStreamBroker, error) {
	var client *redis.Client

	if opts.Client != nil {
		client = opts.Client
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:     opts.Addr,
			Password: opts.Password,
			DB:       opts.DB,
		})
	}

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	prefix := opts.Prefix
	if prefix == "" {
		prefix = "uno:stream:"
	}

	activeTTL := opts.ActiveTTL
	if activeTTL <= 0 {
		activeTTL = defaultActiveTTL
	}
	replayTTL := opts.ReplayTTL
	if replayTTL <= 0 {
		replayTTL = defaultReplayTTL
	}
	maxLen := opts.MaxLen
	if maxLen <= 0 {
		maxLen = defaultMaxLen
	}

	return &RedisStreamBroker{
		client:    client,
		prefix:    prefix,
		activeTTL: activeTTL,
		replayTTL: replayTTL,
		maxLen:    maxLen,
		readCount: defaultReadCount,
		blockTime: defaultBlock,
	}, nil
}

// streamKey returns the Redis Stream key for a channel.
func (b *RedisStreamBroker) streamKey(channel string) string {
	return b.prefix + channel
}

// stopKey returns the Redis key holding the stop flag for a channel.
func (b *RedisStreamBroker) stopKey(channel string) string {
	return b.prefix + "stop:" + channel
}

// Publish appends a chunk to the channel's Redis Stream. The stream
// key has its active TTL refreshed on first write so a long-lived
// stream doesn't silently expire.
func (b *RedisStreamBroker) Publish(ctx context.Context, channel string, chunk *responses.ResponseChunk) error {
	data, err := sonic.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to serialize chunk: %w", err)
	}

	key := b.streamKey(channel)
	id, err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		MaxLen: b.maxLen,
		Approx: true,
		Values: map[string]any{
			"type":    chunk.ChunkType(),
			"payload": data,
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to publish chunk: %w", err)
	}

	// Refresh TTL on first write (id ends in "-0" for the very first
	// entry of a millisecond — cheap best-effort; re-EXPIRE is idempotent).
	if id != "" {
		_ = b.client.Expire(ctx, key, b.activeTTL).Err()
	}

	return nil
}

// Subscribe returns a channel that delivers every chunk of `channel`
// from the first event onward. It first drains the existing stream via
// XRANGE and then tails live entries with XREAD BLOCK. The output
// channel closes when Close has been called (the __stream_end sentinel
// is seen) or the context is cancelled.
//
// Multiple subscribers may be active for the same channel concurrently;
// each receives the full transcript independently.
func (b *RedisStreamBroker) Subscribe(ctx context.Context, channel string) (<-chan *responses.ResponseChunk, error) {
	out := make(chan *responses.ResponseChunk, 100)

	go func() {
		defer close(out)

		key := b.streamKey(channel)

		// Phase 1 — replay everything currently in the stream using
		// paginated XRANGE. lastID tracks the id of the last entry we
		// emitted so Phase 2's XREAD picks up from the right place.
		lastID := "0"
		cursor := "-"
		for {
			entries, err := b.client.XRangeN(ctx, key, cursor, "+", b.readCount).Result()
			if err != nil || len(entries) == 0 {
				break
			}
			endSeen := false
			for _, entry := range entries {
				lastID = entry.ID
				chunk, isEnd, ok := decodeEntry(entry)
				if isEnd {
					endSeen = true
					break
				}
				if !ok {
					continue
				}
				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}
			}
			if endSeen {
				return
			}
			// XRANGE's start is inclusive, so bump past the last id.
			cursor = incrementID(lastID)
		}

		// Phase 2 — live tail. XREAD is exclusive of the passed id.
		for {
			if ctx.Err() != nil {
				return
			}
			res, err := b.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{key, lastID},
				Count:   b.readCount,
				Block:   b.blockTime,
			}).Result()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				return
			}
			for _, stream := range res {
				for _, entry := range stream.Messages {
					lastID = entry.ID
					chunk, isEnd, ok := decodeEntry(entry)
					if isEnd {
						return
					}
					if !ok {
						continue
					}
					select {
					case out <- chunk:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return out, nil
}

// Close writes a stream-end sentinel so active subscribers terminate
// their XREAD BLOCK loops, then shortens the stream's TTL to the
// replay window. Idempotent.
func (b *RedisStreamBroker) Close(ctx context.Context, channel string) error {
	key := b.streamKey(channel)

	if _, err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: key,
		MaxLen: b.maxLen,
		Approx: true,
		Values: map[string]any{
			"type":    streamEndType,
			"payload": "{}",
		},
	}).Result(); err != nil {
		return fmt.Errorf("failed to write stream-end sentinel: %w", err)
	}

	_ = b.client.Expire(ctx, key, b.replayTTL).Err()
	return nil
}

// Stop records a stop request for the channel by setting a flag key
// with the active TTL. The agent loop polls this via IsStopped.
func (b *RedisStreamBroker) Stop(ctx context.Context, channel string) error {
	if err := b.client.Set(ctx, b.stopKey(channel), "1", b.activeTTL).Err(); err != nil {
		return fmt.Errorf("failed to set stop flag: %w", err)
	}
	return nil
}

// IsStopped reports whether Stop has been called for the channel.
func (b *RedisStreamBroker) IsStopped(ctx context.Context, channel string) (bool, error) {
	n, err := b.client.Exists(ctx, b.stopKey(channel)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to read stop flag: %w", err)
	}
	return n > 0, nil
}

// GetClient returns the underlying Redis client.
func (b *RedisStreamBroker) GetClient() *redis.Client {
	return b.client
}

// decodeEntry parses a Redis stream entry into a ResponseChunk. The
// second return is true when the entry is the stream-end sentinel. The
// third return is false when the entry's payload is malformed.
func decodeEntry(entry redis.XMessage) (*responses.ResponseChunk, bool, bool) {
	evType, _ := entry.Values["type"].(string)
	if evType == streamEndType {
		return nil, true, false
	}
	payload, _ := entry.Values["payload"].(string)
	var chunk responses.ResponseChunk
	if err := sonic.Unmarshal([]byte(payload), &chunk); err != nil {
		return nil, false, false
	}
	return &chunk, false, true
}

// incrementID returns the smallest Redis stream id strictly greater
// than id. Redis ids are "<ms>-<seq>"; incrementing the seq suffices.
func incrementID(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			seq, err := strconv.ParseUint(id[i+1:], 10, 64)
			if err != nil {
				return id
			}
			return id[:i+1] + strconv.FormatUint(seq+1, 10)
		}
	}
	return id
}
