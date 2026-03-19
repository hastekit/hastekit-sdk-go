package agents

import (
	"context"
	"sync"
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
			results[idx].Response, results[idx].Err = ex.Tool.Execute(ctx, ex.ToolCall)
		}(i, exec)
	}
	wg.Wait()

	return results
}
