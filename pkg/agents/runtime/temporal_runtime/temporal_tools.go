package temporal_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.temporal.io/sdk/workflow"
)

// workflowContextKey is used to pass a coroutine-specific workflow.Context
// through context.Context, so that TemporalToolProxy can use the correct
// coroutine context when called from within workflow.Go.
type workflowContextKey struct{}

func withWorkflowContext(ctx context.Context, wfCtx workflow.Context) context.Context {
	return context.WithValue(ctx, workflowContextKey{}, wfCtx)
}

func GetWorkflowContext(ctx context.Context) (workflow.Context, bool) {
	wfCtx, ok := ctx.Value(workflowContextKey{}).(workflow.Context)
	return wfCtx, ok
}

type TemporalTool struct {
	wrappedTool agents.Tool
}

func NewTemporalTool(wrappedTool agents.Tool) *TemporalTool {
	return &TemporalTool{
		wrappedTool: wrappedTool,
	}
}

func (t *TemporalTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
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

func (t *TemporalToolProxy) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	// Use coroutine-specific workflow context if available (parallel execution),
	// otherwise fall back to the proxy's stored context (sequential execution).
	wfCtx := t.workflowCtx
	if overrideCtx, ok := GetWorkflowContext(ctx); ok {
		wfCtx = overrideCtx
	}

	var output *agents.ToolCallResponse
	err := workflow.ExecuteActivity(wfCtx, t.prefix+"_ExecuteToolActivity", params).Get(wfCtx, &output)
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

// TemporalToolExecutor executes tools in parallel using Temporal coroutines.
type TemporalToolExecutor struct {
	workflowCtx workflow.Context
}

func NewTemporalToolExecutor(workflowCtx workflow.Context) *TemporalToolExecutor {
	return &TemporalToolExecutor{workflowCtx: workflowCtx}
}

func (e *TemporalToolExecutor) ExecuteAll(ctx context.Context, executions []agents.ToolExecution) []agents.ToolExecutionResult {
	results := make([]agents.ToolExecutionResult, len(executions))

	wg := workflow.NewWaitGroup(e.workflowCtx)
	for i, exec := range executions {
		i, exec := i, exec
		wg.Add(1)
		workflow.Go(e.workflowCtx, func(gCtx workflow.Context) {
			// Pass the coroutine's workflow.Context through context.Context
			// so TemporalToolProxy.Execute uses the correct coroutine context
			// for ExecuteActivity and Future.Get calls.
			execCtx := withWorkflowContext(ctx, gCtx)
			results[i].Response, results[i].Err = exec.Fn(execCtx)
			wg.Done()
		})
	}

	wg.Wait(e.workflowCtx)

	return results
}
