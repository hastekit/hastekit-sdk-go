package temporal_runtime

import (
	"context"
	"time"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.temporal.io/sdk/workflow"
)

type TemporalAgentV2 struct {
	agentConfigs map[string]*agents.AgentOptions
	options      *agents.AgentOptions
	broker       agents.StreamBroker
}

func NewTemporalAgent(configs map[string]*agents.AgentOptions, options *agents.AgentOptions, broker agents.StreamBroker) *TemporalAgentV2 {
	return &TemporalAgentV2{
		agentConfigs: configs,
		options:      options,
		broker:       broker,
	}
}

func (a *TemporalAgentV2) GetActivities() map[string]interface{} {
	activities := map[string]interface{}{}

	temporalPrompt := NewTemporalPrompt(a.options.Instruction)
	activities[a.options.Name+"_GetPromptActivity"] = temporalPrompt.GetPrompt

	temporalLLM := NewTemporalLLM(a.options.LLM, a.broker)
	activities[a.options.Name+"_NewStreamingResponsesActivity"] = temporalLLM.NewStreamingResponsesActivity

	temporalConversationPersistence := NewTemporalConversationPersistence(a.options.History.ConversationPersistenceAdapter)
	activities[a.options.Name+"_LoadMessagesActivity"] = temporalConversationPersistence.LoadMessages
	activities[a.options.Name+"_SaveMessagesActivity"] = temporalConversationPersistence.SaveMessages
	activities[a.options.Name+"_SaveSummaryActivity"] = temporalConversationPersistence.SaveSummary

	if a.options.History.Summarizer != nil {
		temporalSummarizer := NewTemporalConversationSummarizer(a.options.History.Summarizer)
		activities[a.options.Name+"_SummarizerActivity"] = temporalSummarizer
	}

	for _, tool := range a.options.Tools {
		temporalTool := NewTemporalTool(tool)
		activities[getToolName(a.options.Name, tool)+"_ExecuteToolActivity"] = temporalTool.Execute
	}

	for _, mcpClient := range a.options.McpServers {
		temporalMCP := NewTemporalMCPServer(mcpClient)
		activities[mcpClient.GetName()+"_ListMCPToolsActivity"] = temporalMCP.ListTools
		activities[mcpClient.GetName()+"_ExecuteMCPToolActivity"] = temporalMCP.ExecuteTool
	}

	return activities
}

func (a *TemporalAgentV2) Execute(ctx workflow.Context, in *agents.AgentInput) (*agents.AgentOutput, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	})

	workflowId := workflow.GetInfo(ctx).WorkflowExecution.ID
	cb := func(chunk *responses.ResponseChunk) {
		a.broker.Publish(context.Background(), workflowId, chunk)
	}
	defer a.broker.Close(context.Background(), workflowId)
	in.Callback = cb

	agent := a.newTemporalProxyAgent(ctx)

	return agent.ExecuteWithoutTrace(context.Background(), in)
}

func (a *TemporalAgentV2) newTemporalProxyAgent(ctx workflow.Context) *agents.Agent {
	promptProxy := NewTemporalPromptProxy(ctx, a.options.Name)

	llmProxy := NewTemporalLLMProxy(ctx, a.options.Name, a.broker)

	conversationPersistenceProxy := NewTemporalConversationPersistenceProxy(ctx, a.options.Name)
	var options []history.ConversationManagerOptions
	if a.options.History.Summarizer != nil {
		conversationSummarizerProxy := NewTemporalConversationSummarizerProxy(ctx, a.options.Name)
		options = append(options, history.WithSummarizer(conversationSummarizerProxy))
	}
	conversationHistory := history.NewConversationManager(conversationPersistenceProxy, options...)

	var toolProxies []agents.Tool
	for _, tool := range a.options.Tools {
		toolProxy := NewTemporalToolProxy(ctx, getToolName(a.options.Name, tool), tool)
		toolProxies = append(toolProxies, toolProxy)
	}

	var mcpProxies []agents.MCPToolset
	for _, mcpClient := range a.options.McpServers {
		mcpProxy := NewTemporalMCPProxy(ctx, mcpClient.GetName())
		mcpProxies = append(mcpProxies, mcpProxy)
	}

	opts := &agents.AgentOptions{
		Name:       a.options.Name,
		Output:     a.options.Output,
		Parameters: a.options.Parameters,
		MaxLoops:   a.options.MaxLoops,

		History:     conversationHistory,
		Instruction: promptProxy,
		Tools:       toolProxies,
		McpServers:  mcpProxies,
	}

	for _, h := range a.options.Handoffs {
		agentOptions := a.agentConfigs[h.Agent.Name]
		opts.Handoffs = append(opts.Handoffs, agents.NewHandoff(
			NewTemporalAgent(a.agentConfigs, agentOptions, a.broker).newTemporalProxyAgent(ctx), h.Description),
		)
	}

	return agents.NewAgent(opts).WithLLM(llmProxy)
}

func getToolName(prefix string, tool agents.Tool) string {
	toolName := ""
	if t := tool.Tool(context.Background()); t != nil {
		if t.OfFunction != nil {
			toolName = t.OfFunction.Name
		}

		if t.OfWebSearch != nil {
			toolName = "web_search"
		}

		if t.OfImageGeneration != nil {
			toolName = "image_generation"
		}
	}

	return prefix + "_" + toolName
}
