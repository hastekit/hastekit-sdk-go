package sdk

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/streambroker"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway"
	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/client"
)

var (
	tracer = otel.Tracer("hastekit-sdk-go")
)

type SDK struct {
	*gateway.LLMClient

	endpoint       string
	orgName        string
	projectName    string
	virtualKey     string
	directMode     bool
	llmConfigs     gateway.ConfigStore
	restateConfig  RestateConfig
	temporalConfig TemporalConfig
	redisConfig    RedisConfig
	httpClient     *http.Client

	agents               map[string]*agents.Agent
	restateAgentConfigs  map[string]*agents.AgentOptions
	temporalAgentConfigs map[string]*agents.AgentOptions
	redisBroker          agents.StreamBroker

	temporalClient client.Client
}

type ServerConfig struct {
	// Endpoint of the Uno Server
	Endpoint string

	// For LLM calls
	VirtualKey string

	// Org name — the org segment of the GitHub-style API path
	OrgName string

	// Project name — the project segment of the API path.
	ProjectName string
}

type RestateConfig struct {
	Endpoint string
}

type TemporalConfig struct {
	Endpoint string
}

type RedisConfig struct {
	Endpoint string
}

// ClientOptions holds the configuration for the SDK client
type ClientOptions struct {
	serverConfig    ServerConfig
	providerConfigs []gateway.ProviderConfig
	restateConfig   RestateConfig
	temporalConfig  TemporalConfig
	redisConfig     RedisConfig
	httpClient      *http.Client
	timeout         time.Duration
}

// ClientOption is a function that configures the SDK client
type ClientOption func(*ClientOptions)

// WithServerConfig configures the SDK to use a HasteKit Gateway server
func WithServerConfig(endpoint, virtualKey, orgName, projectName string) ClientOption {
	return func(opts *ClientOptions) {
		opts.serverConfig = ServerConfig{
			Endpoint:    endpoint,
			VirtualKey:  virtualKey,
			OrgName:     orgName,
			ProjectName: projectName,
		}
	}
}

// WithProviderConfigs configures the SDK to use direct provider connections
func WithProviderConfigs(configs ...gateway.ProviderConfig) ClientOption {
	return func(opts *ClientOptions) {
		opts.providerConfigs = configs
	}
}

// WithRestateConfig configures Restate for durable execution
func WithRestateConfig(endpoint string) ClientOption {
	return func(opts *ClientOptions) {
		opts.restateConfig = RestateConfig{Endpoint: endpoint}
	}
}

// WithTemporalConfig configures Temporal for durable execution
func WithTemporalConfig(endpoint string) ClientOption {
	return func(opts *ClientOptions) {
		opts.temporalConfig = TemporalConfig{Endpoint: endpoint}
	}
}

// WithRedisConfig configures Redis for stream brokering
func WithRedisConfig(endpoint string) ClientOption {
	return func(opts *ClientOptions) {
		opts.redisConfig = RedisConfig{Endpoint: endpoint}
	}
}

// WithHTTPClient configures a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(opts *ClientOptions) {
		opts.httpClient = client
	}
}

// WithTimeout configures a default timeout for HTTP requests
func WithTimeout(timeout time.Duration) ClientOption {
	return func(opts *ClientOptions) {
		opts.timeout = timeout
	}
}

// Deprecated: Use functional options instead
type LegacyClientOptions struct {
	ServerConfig ServerConfig

	// Set this if you are using the SDK without the LLM Gateway server.
	ProviderConfigs []gateway.ProviderConfig

	RestateConfig  RestateConfig
	TemporalConfig TemporalConfig
	RedisConfig    RedisConfig
}

// New creates a new SDK instance with the struct-based options (legacy)
// Deprecated: Use NewWithOptions with functional options instead for better flexibility
func New(legacyOpts *LegacyClientOptions) (*SDK, error) {
	opts := &ClientOptions{
		serverConfig:    legacyOpts.ServerConfig,
		providerConfigs: legacyOpts.ProviderConfigs,
		restateConfig:   legacyOpts.RestateConfig,
		temporalConfig:  legacyOpts.TemporalConfig,
		redisConfig:     legacyOpts.RedisConfig,
		timeout:         30 * time.Second,
	}
	return newSDK(opts)
}

// NewWithOptions creates a new SDK instance with functional options (recommended)
func NewWithOptions(options ...ClientOption) (*SDK, error) {
	opts := &ClientOptions{
		timeout: 30 * time.Second, // Default timeout
	}

	// Apply all options
	for _, option := range options {
		option(opts)
	}

	return newSDK(opts)
}

// NewWithLegacyOptions creates a new SDK instance with the old struct-based options
// Deprecated: Use NewWithOptions with functional options instead
func NewWithLegacyOptions(legacyOpts *LegacyClientOptions) (*SDK, error) {
	opts := &ClientOptions{
		serverConfig:    legacyOpts.ServerConfig,
		providerConfigs: legacyOpts.ProviderConfigs,
		restateConfig:   legacyOpts.RestateConfig,
		temporalConfig:  legacyOpts.TemporalConfig,
		redisConfig:     legacyOpts.RedisConfig,
		timeout:         30 * time.Second,
	}
	return newSDK(opts)
}

func newSDK(opts *ClientOptions) (*SDK, error) {
	if opts.providerConfigs == nil && opts.serverConfig.Endpoint == "" {
		return nil, fmt.Errorf("must provide either ServerConfig.Endpoint or ProviderConfigs")
	}

	var configStore gateway.ConfigStore
	if opts.providerConfigs != nil {
		configStore = gateway.NewInMemoryConfigStore(opts.providerConfigs)
	}

	var broker agents.StreamBroker
	if opts.redisConfig.Endpoint != "" {
		var err error
		broker, err = streambroker.NewRedisStreamBroker(streambroker.RedisStreamBrokerOptions{
			Addr: opts.redisConfig.Endpoint,
		})
		if err != nil {
			return nil, fmt.Errorf("error creating redis stream broker: %w", err)
		}
	} else {
		// Execute requires a broker. Fall back to an in-memory broker so
		// agents work out-of-the-box; production deployments should
		// configure Redis for cross-process streaming.
		broker = streambroker.NewMemoryStreamBroker()
	}

	httpClient := opts.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: opts.timeout,
		}
	}

	sdk := &SDK{
		llmConfigs:     configStore,
		directMode:     configStore != nil,
		endpoint:       opts.serverConfig.Endpoint,
		orgName:        opts.serverConfig.OrgName,
		projectName:    opts.serverConfig.ProjectName,
		virtualKey:     opts.serverConfig.VirtualKey,
		restateConfig:  opts.restateConfig,
		temporalConfig: opts.temporalConfig,
		redisConfig:    opts.redisConfig,
		httpClient:     httpClient,

		agents:               map[string]*agents.Agent{},
		restateAgentConfigs:  map[string]*agents.AgentOptions{},
		temporalAgentConfigs: map[string]*agents.AgentOptions{},
		redisBroker:          broker,
	}

	sdk.setTemporalClient()
	sdk.setLLMClient()

	return sdk, nil
}
