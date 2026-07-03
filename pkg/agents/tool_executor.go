package agents

import (
	"context"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ToolExecution represents a single tool execution to be run.
type ToolExecution struct {
	ExecutableToolCall ExecutableToolCall
	Fn                 func(ctx context.Context) (*ToolCallResponse, error)
}

type ExecutableToolCall struct {
	Index    int
	ToolName string
	Tool     Tool
	ToolCall *ToolCall
}

// ToolExecutionResult holds the result of a single tool execution.
type ToolExecutionResult struct {
	Response *ToolCallResponse
	Err      error
}

// ToolExecutor executes tool calls, potentially in parallel.
// Implementations must return results in the same order as the input executions.
type ToolExecutor interface {
	ExecuteAll(ctx context.Context, executions []ExecutableToolCall) []ToolExecutionResult
}

// DefaultToolExecutor executes tools in parallel using goroutines.
type DefaultToolExecutor struct{}

func (e *DefaultToolExecutor) ExecuteAll(ctx context.Context, executions []ExecutableToolCall) []ToolExecutionResult {
	results := make([]ToolExecutionResult, len(executions))

	var wg sync.WaitGroup
	for i, exec := range executions {
		wg.Add(1)
		go func(idx int, ex ExecutableToolCall) {
			defer wg.Done()

			// GenAI execute_tool span. The in-process executor is the point of
			// real execution for the default runtime; the restate and temporal
			// runtimes create the equivalent span inside their own durable
			// steps instead (see RestateTool.Execute / TemporalTool.Execute).
			toolCtx, span := tracer.Start(ctx, genai.OpExecuteTool+" "+ex.ToolCall.Name)
			defer span.End()
			span.SetAttributes(
				attribute.String(genai.AttrOperationName, genai.OpExecuteTool),
				attribute.String(genai.AttrToolName, ex.ToolCall.Name),
				attribute.String(genai.AttrToolCallID, ex.ToolCall.CallID),
				attribute.String(genai.AttrToolArguments, ex.ToolCall.Arguments),
			)
			if def := ex.Tool.Tool(toolCtx); def != nil && def.OfFunction != nil && def.OfFunction.Description != nil {
				span.SetAttributes(attribute.String(genai.AttrToolDescription, *def.OfFunction.Description))
			}

			resp, err := ex.Tool.Execute(toolCtx, ex.ToolCall)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if resp != nil && resp.FunctionCallOutputMessage != nil {
				if out, mErr := sonic.Marshal(resp.Output); mErr == nil {
					span.SetAttributes(attribute.String(genai.AttrToolResult, string(out)))
				}
			}

			results[idx].Response, results[idx].Err = resp, err
		}(i, exec)
	}
	wg.Wait()

	return results
}
