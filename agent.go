package sdk

import (
	"sort"

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

// Agent returns a registered agent by name.
func (c *SDK) Agent(name string) (*agents.Agent, bool) {
	agent, ok := c.agents[name]
	return agent, ok
}

// AgentNames returns the names of all registered agents, sorted.
func (c *SDK) AgentNames() []string {
	names := make([]string, 0, len(c.agents))
	for name := range c.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
