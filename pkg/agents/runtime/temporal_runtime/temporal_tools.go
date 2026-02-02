package temporal_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.temporal.io/sdk/workflow"
)

type TemporalTool struct {
	wrappedTool agents.Tool
}

func NewTemporalTool(wrappedTool agents.Tool) *TemporalTool {
	return &TemporalTool{
		wrappedTool: wrappedTool,
	}
}

func (t *TemporalTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	return t.wrappedTool.Execute(ctx, params)
}

type TemporalToolProxy struct {
	workflowCtx workflow.Context
	prefix      string
	wrappedTool agents.Tool
}

func NewTemporalToolProxy(workflowCtx workflow.Context, prefix string, wrappedTool agents.Tool) agents.Tool {
	return &TemporalToolProxy{
		workflowCtx: workflowCtx,
		prefix:      prefix,
		wrappedTool: wrappedTool,
	}
}

func (t *TemporalToolProxy) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	var output *responses.FunctionCallOutputMessage
	err := workflow.ExecuteActivity(t.workflowCtx, t.prefix+"_ExecuteToolActivity", params).Get(t.workflowCtx, &output)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (t *TemporalToolProxy) Tool(ctx context.Context) *responses.ToolUnion {
	return t.wrappedTool.Tool(ctx)
}

func (t *TemporalToolProxy) NeedApproval() bool {
	return t.wrappedTool.NeedApproval()
}
