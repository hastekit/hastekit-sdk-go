package agents

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type ToolCall struct {
	*responses.FunctionCallMessage
	AgentName      string `json:"agent_name"`
	AgentVersion   string `json:"agent_version"`
	Namespace      string `json:"namespace"`
	ConversationID string `json:"conversation_id"`
}

type Tool interface {
	Execute(ctx context.Context, params *ToolCall) (*responses.FunctionCallOutputMessage, error)
	Tool(ctx context.Context) *responses.ToolUnion
	NeedApproval() bool
}

type BaseTool struct {
	ToolUnion        responses.ToolUnion
	RequiresApproval bool
}

func (t *BaseTool) NeedApproval() bool {
	return t.RequiresApproval
}

func (t *BaseTool) Tool(ctx context.Context) *responses.ToolUnion {
	return &t.ToolUnion
}

// partitionByApproval splits tool calls into those needing approval and those that can execute immediately
func partitionByApproval(ctx context.Context, tools []Tool, toolCalls []responses.FunctionCallMessage) (needsApproval []responses.FunctionCallMessage, immediate []responses.FunctionCallMessage) {
	for _, toolCall := range toolCalls {
		tool := findTool(ctx, tools, toolCall.Name)
		if tool != nil && tool.NeedApproval() {
			needsApproval = append(needsApproval, toolCall)
		} else {
			immediate = append(immediate, toolCall)
		}
	}
	return needsApproval, immediate
}

// findTool finds a tool by name
func findTool(ctx context.Context, tools []Tool, toolName string) Tool {
	for _, tool := range tools {
		if t := tool.Tool(ctx); t != nil && t.OfFunction != nil && t.OfFunction.Name == toolName {
			return tool
		}
	}
	return nil
}
