package sdk

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/streambroker"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway"
	"github.com/hastekit/hastekit-sdk-go/pkg/hastekitgateway"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("hastekit-sdk-go")
)

type SDK struct {
	*gateway.LLMClient

	endpoint       string
	projectId      uuid.UUID
	virtualKey     string
	directMode     bool
	llmConfigs     gateway.ConfigStore
	restateConfig  RestateConfig
	temporalConfig TemporalConfig
	redisConfig    RedisConfig

	agents               map[string]*agents.Agent
	restateAgentConfigs  map[string]*agents.AgentOptions
	temporalAgentConfigs map[string]*agents.AgentOptions
	redisBroker          agents.StreamBroker
}

type ServerConfig struct {
	// Endpoint of the Uno Server
	Endpoint string

	// For LLM calls
	VirtualKey string

	// For conversations
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

type ClientOptions struct {
	ServerConfig ServerConfig

	// Set this if you are using the SDK without the LLM Gateway server.
	ProviderConfigs []gateway.ProviderConfig

	RestateConfig  RestateConfig
	TemporalConfig TemporalConfig
	RedisConfig    RedisConfig
}

func New(opts *ClientOptions) (*SDK, error) {
	if opts.ProviderConfigs == nil && opts.ServerConfig.Endpoint == "" {
		return nil, fmt.Errorf("must provide either ServerConfig.Endpoint or LLMConfigs")
	}

	var configStore gateway.ConfigStore
	if opts.ProviderConfigs != nil {
		configStore = gateway.NewInMemoryConfigStore(opts.ProviderConfigs)
	}

	var broker agents.StreamBroker
	var err error
	if opts.RedisConfig.Endpoint != "" {
		broker, err = streambroker.NewRedisStreamBroker(streambroker.RedisStreamBrokerOptions{
			Addr: opts.RedisConfig.Endpoint,
		})
		if err != nil {
			return nil, fmt.Errorf("error creating redis stream broker: %w", err)
		}
	}

	sdk := &SDK{
		llmConfigs:     configStore,
		directMode:     configStore != nil,
		endpoint:       opts.ServerConfig.Endpoint,
		virtualKey:     opts.ServerConfig.VirtualKey,
		restateConfig:  opts.RestateConfig,
		temporalConfig: opts.TemporalConfig,
		redisConfig:    opts.RedisConfig,

		agents:               map[string]*agents.Agent{},
		restateAgentConfigs:  map[string]*agents.AgentOptions{},
		temporalAgentConfigs: map[string]*agents.AgentOptions{},
		redisBroker:          broker,
	}

	sdk.setLLMClient()

	if opts.ServerConfig.ProjectName == "" {
		return sdk, nil
	}

	// Convert project name to ID
	resp, err := http.DefaultClient.Get(fmt.Sprintf("%s/api/agent-server/projects", opts.ServerConfig.Endpoint))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	projectsRes := hastekitgateway.Response[[]Project]{}
	if err := utils.DecodeJSON(resp.Body, &projectsRes); err != nil {
		return nil, err
	}

	for _, proj := range projectsRes.Data {
		if proj.Name == opts.ServerConfig.ProjectName {
			sdk.projectId = proj.ID
			return sdk, nil
		}
	}

	return nil, fmt.Errorf("project %s not found", opts.ServerConfig.ProjectName)
}

// Project represents a workspace or collection an agent can belong to
type Project struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	DefaultKey *string   `json:"default_key,omitempty" db:"default_key"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
