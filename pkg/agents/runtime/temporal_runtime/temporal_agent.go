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
	options *agents.AgentOptions
	broker  agents.StreamBroker
}

func NewTemporalAgent(options *agents.AgentOptions, broker agents.StreamBroker) *TemporalAgentV2 {
	return &TemporalAgentV2{
		options: options,
		broker:  broker,
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

	agent := agents.NewAgent(&agents.AgentOptions{
		Name:       a.options.Name,
		Output:     a.options.Output,
		Parameters: a.options.Parameters,
		MaxLoops:   a.options.MaxLoops,

		History:     conversationHistory,
		Instruction: promptProxy,
		Tools:       toolProxies,
		McpServers:  mcpProxies,
	})
	agent = agent.WithLLM(llmProxy)

	return agent.ExecuteWithExecutor(context.Background(), in, cb)
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
