package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
)

type AgentOptions agents.AgentOptions

func (c *SDK) NewAgent(options *AgentOptions) *agents.Agent {
	agent := agents.NewAgent((*agents.AgentOptions)(options))

	c.agents[options.Name] = agent

	return agent
}
