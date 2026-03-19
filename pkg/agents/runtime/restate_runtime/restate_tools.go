package restate_runtime

import (
	"context"
	"log/slog"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	restate "github.com/restatedev/sdk-go"
)

// insideRunAsyncKey signals that execution is already inside a restate.RunAsync,
// so Restate wrappers should execute directly instead of calling restate.Run
// (which would concurrently access the shared WorkflowContext).
type insideRunAsyncKey struct{}

func IsInsideRunAsync(ctx context.Context) bool {
	v, _ := ctx.Value(insideRunAsyncKey{}).(bool)
	return v
}

func withInsideRunAsync(ctx context.Context) context.Context {
	return context.WithValue(ctx, insideRunAsyncKey{}, true)
}

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
	// If already inside a RunAsync, execute directly — the outer RunAsync
	// provides durability, so wrapping in another restate.Run is redundant
	// and would concurrently access the shared WorkflowContext.
	if IsInsideRunAsync(ctx) {
		return t.wrappedTool.Execute(ctx, params)
	}

	return restate.Run(t.restateCtx, func(ctx restate.RunContext) (*agents.ToolCallResponse, error) {
		return t.wrappedTool.Execute(ctx, params)
	}, restate.WithName(params.Name+"_ToolCall"))
}

func (t *RestateTool) Tool(ctx context.Context) *responses.ToolUnion {
	return t.wrappedTool.Tool(ctx)
}

func (t *RestateTool) NeedApproval() bool {
	return t.wrappedTool.NeedApproval()
}

// RestateToolExecutor executes tools in parallel using restate.RunAsync futures.
type RestateToolExecutor struct {
	restateCtx restate.WorkflowContext
}

func NewRestateToolExecutor(restateCtx restate.WorkflowContext) *RestateToolExecutor {
	return &RestateToolExecutor{restateCtx: restateCtx}
}

func (e *RestateToolExecutor) ExecuteAll(ctx context.Context, executions []agents.ToolExecution) []agents.ToolExecutionResult {
	// Start all tool executions as async durable futures
	futures := make([]restate.RunAsyncFuture[*agents.ToolCallResponse], len(executions))
	for i, exec := range executions {
		exec := exec
		futures[i] = restate.RunAsync[*agents.ToolCallResponse](e.restateCtx, func(runCtx restate.RunContext) (*agents.ToolCallResponse, error) {
			// Mark context so nested Restate wrappers (RestateTool, RestateMCPTool)
			// skip their restate.Run calls and execute directly.
			asyncCtx := withInsideRunAsync(ctx)
			return exec.Fn(asyncCtx)
		})
	}

	var ff []restate.Future
	for _, f := range futures {
		ff = append(ff, f)
	}

	var results []agents.ToolExecutionResult
	for fut, err := range restate.Wait(e.restateCtx, ff...) {
		if err != nil {
			slog.Error("error: ", slog.Any("error", err))
			results = append(results, agents.ToolExecutionResult{Err: err})
			continue
		}

		response, err := fut.(restate.RunAsyncFuture[*agents.ToolCallResponse]).Result()
		results = append(results, agents.ToolExecutionResult{
			Response: response,
			Err:      err,
		})
	}

	return results
}
