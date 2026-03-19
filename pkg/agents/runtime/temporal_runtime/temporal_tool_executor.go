package temporal_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
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

// TemporalToolExecutor executes tools in parallel using Temporal coroutines.
type TemporalToolExecutor struct {
	workflowCtx workflow.Context
}

func NewTemporalToolExecutor(workflowCtx workflow.Context) *TemporalToolExecutor {
	return &TemporalToolExecutor{workflowCtx: workflowCtx}
}

func (e *TemporalToolExecutor) ExecuteAll(ctx context.Context, executions []agents.ExecutableToolCall) []agents.ToolExecutionResult {
	results := make([]agents.ToolExecutionResult, len(executions))

	for i, exec := range executions {
		results[i].Response, results[i].Err = exec.Tool.Execute(ctx, exec.ToolCall)
	}

	// TODO: Parallelize temporal tool execution
	//wg := workflow.NewWaitGroup(e.workflowCtx)
	//for i, exec := range executions {
	//	i, exec := i, exec
	//	wg.Add(1)
	//	workflow.Go(e.workflowCtx, func(gCtx workflow.Context) {
	//		results[i].Response, results[i].Err = exec.Tool.Execute(withWorkflowContext(ctx, gCtx), exec.ToolCall)
	//		wg.Done()
	//	})
	//}
	//
	//wg.Wait(e.workflowCtx)

	return results
}
