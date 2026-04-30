package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
)

type AgentOptions agents.AgentOptions

func (c *SDK) NewAgent(options *AgentOptions) *agents.Agent {
	opts := (*agents.AgentOptions)(options)
	if opts.StreamBroker == nil {
		opts.StreamBroker = c.redisBroker
	}
	agent := agents.NewAgent(opts)

	c.agents[options.Name] = agent

	return agent
}
