package restate_runtime

import (
	"context"
	"fmt"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/agentstate"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	restate "github.com/restatedev/sdk-go"
)

// AgentWorkflow is the Restate workflow that executes agents with durability.
type AgentWorkflow struct {
	agentConfigs map[string]*agents.AgentOptions
	broker       agents.StreamBroker
}

func NewRestateWorkflow(agentConfigs map[string]*agents.AgentOptions, broker agents.StreamBroker) *AgentWorkflow {
	return &AgentWorkflow{
		agentConfigs: agentConfigs,
		broker:       broker,
	}
}

// Run executes the agent inside a Restate workflow context.
func (w *AgentWorkflow) Run(restateCtx restate.WorkflowContext, input *WorkflowInput) (*agents.AgentOutput, error) {
	agentOptions, ok := w.agentConfigs[input.AgentName]
	if !ok {
		return &agents.AgentOutput{Status: agentstate.RunStatusError}, fmt.Errorf("agent not found: %s", input.AgentName)
	}

	workflowId := restate.Key(restateCtx)
	cb := func(chunk *responses.ResponseChunk) {
		w.broker.Publish(context.Background(), workflowId, chunk)
	}
	defer w.broker.Close(context.Background(), workflowId)

	agent := w.newRestateAgentProxy(restateCtx, agentOptions)

	// Execute using the SAME agent instance with durability
	return agent.Execute(restateCtx, &agents.AgentInput{
		Namespace:         input.Namespace,
		PreviousMessageID: input.PreviousMessageID,
		Messages:          input.Messages,
		RunContext:        input.RunContext,
		Callback:          cb,
	})
}

func (w *AgentWorkflow) newRestateAgentProxy(restateCtx restate.WorkflowContext, agentOptions *agents.AgentOptions) *agents.Agent {
	promptProxy := NewRestatePrompt(restateCtx, agentOptions.Instruction)

	llmProxy := NewRestateLLM(restateCtx, agentOptions.LLM)

	conversationPersistenceProxy := NewRestateConversationPersistence(restateCtx, agentOptions.History.ConversationPersistenceAdapter)
	var options []history.ConversationManagerOptions
	if agentOptions.History.Summarizer != nil {
		conversationSummarizerProxy := NewRestateConversationSummarizer(restateCtx, agentOptions.History.Summarizer)
		options = append(options, history.WithSummarizer(conversationSummarizerProxy))
	}
	conversationHistory := history.NewConversationManager(conversationPersistenceProxy, options...)

	var restateTools []agents.Tool
	for _, tool := range agentOptions.Tools {
		restateTools = append(restateTools, NewRestateTool(restateCtx, tool))
	}

	var mcpClients []agents.MCPToolset
	for _, mcpClient := range agentOptions.McpServers {
		mcpClients = append(mcpClients, NewRestateMCPServer(restateCtx, mcpClient))
	}

	opts := &agents.AgentOptions{
		Name:       agentOptions.Name,
		Output:     agentOptions.Output,
		Parameters: agentOptions.Parameters,
		MaxLoops:   agentOptions.MaxLoops,

		Instruction: promptProxy,
		History:     conversationHistory,
		Tools:       restateTools,
		McpServers:  mcpClients,
	}

	for _, h := range agentOptions.Handoffs {
		agentOption := w.agentConfigs[h.Name]
		opts.Handoffs = append(opts.Handoffs, agents.NewHandoff(
			h.Name, h.Description, w.newRestateAgentProxy(restateCtx, agentOption),
		))
	}

	return agents.NewAgent(opts).WithLLM(llmProxy)
}
