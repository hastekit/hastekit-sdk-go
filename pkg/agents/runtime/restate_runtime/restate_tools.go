package restate_runtime

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	restate "github.com/restatedev/sdk-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("Agent")

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

func (t *RestateTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	return restate.Run(t.restateCtx, func(runCtx restate.RunContext) (*agents.ToolCallResponse, error) {
		// GenAI execute_tool span, created inside restate.Run so it fires
		// exactly once (on real execution) and never on replay.
		toolCtx, span := tracer.Start(runCtx, genai.OpExecuteTool+" "+params.Name)
		defer span.End()
		span.SetAttributes(
			attribute.String(genai.AttrOperationName, genai.OpExecuteTool),
			attribute.String(genai.AttrToolName, params.Name),
			attribute.String(genai.AttrToolCallID, params.CallID),
			attribute.String(genai.AttrToolArguments, params.Arguments),
		)
		if def := t.wrappedTool.Tool(toolCtx); def != nil && def.OfFunction != nil && def.OfFunction.Description != nil {
			span.SetAttributes(attribute.String(genai.AttrToolDescription, *def.OfFunction.Description))
		}

		resp, err := t.wrappedTool.Execute(toolCtx, params)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if resp != nil && resp.FunctionCallOutputMessage != nil {
			if out, mErr := sonic.Marshal(resp.Output); mErr == nil {
				span.SetAttributes(attribute.String(genai.AttrToolResult, string(out)))
			}
		}

		return resp, err
	}, restate.WithName(params.Name+"_ToolCall"))
}

func (t *RestateTool) Tool(ctx context.Context) *responses.ToolUnion {
	return t.wrappedTool.Tool(ctx)
}

func (t *RestateTool) NeedApproval() bool {
	return t.wrappedTool.NeedApproval()
}

func (t *RestateTool) IsDeferred() bool {
	return t.wrappedTool.IsDeferred()
}
