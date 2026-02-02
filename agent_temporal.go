package sdk

import (
	"log"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/runtime/temporal_runtime"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func (c *SDK) NewTemporalAgent(options *AgentOptions) *agents.Agent {
	agent := agents.NewAgent(&agents.AgentOptions{
		Name:        options.Name,
		LLM:         options.LLM,
		History:     options.History,
		Parameters:  options.Parameters,
		Output:      options.Output,
		Tools:       options.Tools,
		Instruction: options.Instruction,
		McpServers:  options.McpServers,
		Runtime:     temporal_runtime.NewTemporalRuntime(c.temporalConfig.Endpoint, c.redisBroker),
		MaxLoops:    options.MaxLoops,
	})

	c.agents[options.Name] = agent
	c.temporalAgentConfigs[options.Name] = &agents.AgentOptions{
		Name:        options.Name,
		LLM:         options.LLM,
		History:     options.History,
		Parameters:  options.Parameters,
		Output:      options.Output,
		Tools:       options.Tools,
		Instruction: options.Instruction,
		McpServers:  options.McpServers,
	}

	return agent
}

func (c *SDK) StartTemporalService() {
	cli, err := client.Dial(client.Options{
		HostPort: c.temporalConfig.Endpoint,
	})
	if err != nil {
		panic("unable to create temporal client")
	}

	go func() {
		w := worker.New(cli, "AgentWorkflowTaskQueue", worker.Options{})

		// Register workflows and activities based on the agents available in the SDK
		for agentName, agentOptions := range c.temporalAgentConfigs {
			temporalAgentProxy := temporal_runtime.NewTemporalAgent(agentOptions, c.redisBroker)
			for name, fn := range temporalAgentProxy.GetActivities() {
				w.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: name})
			}
			w.RegisterWorkflowWithOptions(temporalAgentProxy.Execute, workflow.RegisterOptions{
				Name: agentName + "_AgentWorkflow",
			})
		}

		err = w.Run(worker.InterruptCh())
		if err != nil {
			log.Fatal(err)
		}
	}()
}
