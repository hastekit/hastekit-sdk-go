package tools

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type CodeExecutionTool struct {
	*agents.BaseTool
}

func NewCodeExecutionTool() *CodeExecutionTool {
	return &CodeExecutionTool{}
}

func (t *CodeExecutionTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	return nil, nil
}

func (t *CodeExecutionTool) Tool(ctx context.Context) *responses.ToolUnion {
	return &responses.ToolUnion{OfCodeExecution: &responses.CodeExecutionTool{
		Container: &responses.CodeExecutionToolContainerUnion{
			ContainerConfig: &responses.CodeExecutionToolContainerConfig{
				Type:        "auto",
				MemoryLimit: "4g",
			},
		},
	}}
}
