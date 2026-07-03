package temporal_runtime

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.temporal.io/sdk/workflow"
)

var tracer = otel.Tracer("Agent")

type TemporalTool struct {
	wrappedTool agents.Tool
}

func NewTemporalTool(wrappedTool agents.Tool) *TemporalTool {
	return &TemporalTool{
		wrappedTool: wrappedTool,
	}
}

func (t *TemporalTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	// GenAI execute_tool span. TemporalTool runs inside the _ExecuteToolActivity
	// activity, which executes exactly once per real tool call (activity results
	// are cached in workflow history), so the span here is replay-safe.
	toolCtx, span := tracer.Start(ctx, genai.OpExecuteTool+" "+params.Name)
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

func (t *TemporalToolProxy) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	var output *agents.ToolCallResponse
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

func (t *TemporalToolProxy) IsDeferred() bool {
	return t.wrappedTool.IsDeferred()
}
