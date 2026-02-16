package sdk

import (
	"context"
	"fmt"
	"log"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/runtime/restate_runtime"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/server"
)

func (c *SDK) NewRestateAgent(options *AgentOptions) *agents.Agent {
	agent := agents.NewAgent(&agents.AgentOptions{
		Name:        options.Name,
		LLM:         options.LLM,
		History:     options.History,
		Parameters:  options.Parameters,
		Output:      options.Output,
		Tools:       options.Tools,
		Instruction: options.Instruction,
		McpServers:  options.McpServers,
		Runtime:     restate_runtime.NewRestateRuntime(c.restateConfig.Endpoint, c.redisBroker),
		MaxLoops:    options.MaxLoops,
		Handoffs:    options.Handoffs,
	})

	c.agents[options.Name] = agent
	c.restateAgentConfigs[options.Name] = &agents.AgentOptions{
		Name:        options.Name,
		LLM:         options.LLM,
		History:     options.History,
		Parameters:  options.Parameters,
		Output:      options.Output,
		Tools:       options.Tools,
		Instruction: options.Instruction,
		McpServers:  options.McpServers,
		MaxLoops:    options.MaxLoops,
		Handoffs:    options.Handoffs,
	}

	return agent
}

func (c *SDK) StartRestateService(host, port string) {
	wf := restate_runtime.NewRestateWorkflow(c.restateAgentConfigs, c.redisBroker)

	go func() {
		if err := server.NewRestate().
			Bind(restate.Reflect(wf)).
			Start(context.Background(), fmt.Sprintf("%s:%s", host, port)); err != nil {
			log.Fatal(err)
		}
	}()
}
