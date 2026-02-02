package restate_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	restate "github.com/restatedev/sdk-go"
)

type RestateTool struct {
	restateCtx  restate.WorkflowContext
	wrappedTool agents.Tool
}

func NewRestateTool(restateCtx restate.WorkflowContext, wrappedTool agents.Tool) *RestateTool {
	return &RestateTool{
		restateCtx:  restateCtx,
		wrappedTool: wrappedTool,
	}
}

func (t *RestateTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	return restate.Run(t.restateCtx, func(ctx restate.RunContext) (*responses.FunctionCallOutputMessage, error) {
		return t.wrappedTool.Execute(ctx, params)
	}, restate.WithName(params.Name+"_ToolCall"))
}

func (t *RestateTool) Tool(ctx context.Context) *responses.ToolUnion {
	return t.wrappedTool.Tool(ctx)
}

func (t *RestateTool) NeedApproval() bool {
	return t.wrappedTool.NeedApproval()
}
