package restate_runtime

import (
	"context"
	"log/slog"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	restate "github.com/restatedev/sdk-go"
)

// RestateToolExecutor executes tools in parallel using restate.RunAsync futures.
type RestateToolExecutor struct {
	restateCtx restate.WorkflowContext
}

func NewRestateToolExecutor(restateCtx restate.WorkflowContext) *RestateToolExecutor {
	return &RestateToolExecutor{restateCtx: restateCtx}
}

func (e *RestateToolExecutor) ExecuteAll(ctx context.Context, executions []agents.ExecutableToolCall) []agents.ToolExecutionResult {
	var results []agents.ToolExecutionResult

	for _, exec := range executions {
		resp, err := exec.Tool.Execute(ctx, exec.ToolCall)
		if err != nil {
			slog.Error("error: ", slog.Any("error", err))
			results = append(results, agents.ToolExecutionResult{Err: err})
			continue
		}
		results = append(results, agents.ToolExecutionResult{Response: resp})
	}

	// TODO: parallelize restate tool execution
	//futures := make([]restate.Future, len(executions))
	//for i, exec := range executions {
	//	futures[i] = restate.RunAsync[*agents.ToolCallResponse](e.restateCtx, func(runCtx restate.RunContext) (*agents.ToolCallResponse, error) {
	//		return exec.Tool.Execute(runCtx, exec.ToolCall)
	//	}, restate.WithName("ToolCall: "+exec.ToolCall.Name))
	//}
	//
	//for fut, err := range restate.Wait(e.restateCtx, futures...) {
	//	if err != nil {
	//		slog.Error("error: ", slog.Any("error", err))
	//		results = append(results, agents.ToolExecutionResult{Err: err})
	//		continue
	//	}
	//
	//	response, err := fut.(restate.RunAsyncFuture[*agents.ToolCallResponse]).Result()
	//	results = append(results, agents.ToolExecutionResult{
	//		Response: response,
	//		Err:      err,
	//	})
	//}

	return results
}
