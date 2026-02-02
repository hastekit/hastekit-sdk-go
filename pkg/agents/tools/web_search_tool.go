package tools

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type WebSearchTool struct {
	*agents.BaseTool
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{}
}

func (t *WebSearchTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	return nil, nil
}

func (t *WebSearchTool) Tool(ctx context.Context) *responses.ToolUnion {
	return &responses.ToolUnion{OfWebSearch: &responses.WebSearchTool{}}
}
