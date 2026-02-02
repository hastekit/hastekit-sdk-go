package temporal_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"go.temporal.io/sdk/workflow"
)

type TemporalPrompt struct {
	wrappedPrompt agents.SystemPromptProvider
}

func NewTemporalPrompt(wrappedPrompt agents.SystemPromptProvider) *TemporalPrompt {
	return &TemporalPrompt{
		wrappedPrompt: wrappedPrompt,
	}
}

func (p *TemporalPrompt) GetPrompt(ctx context.Context, data map[string]any) (string, error) {
	return p.wrappedPrompt.GetPrompt(ctx, data)
}

type TemporalPromptProxy struct {
	workflowCtx workflow.Context
	prefix      string
}

func NewTemporalPromptProxy(workflowCtx workflow.Context, prefix string) agents.SystemPromptProvider {
	return &TemporalPromptProxy{
		workflowCtx: workflowCtx,
		prefix:      prefix,
	}
}

func (p *TemporalPromptProxy) GetPrompt(ctx context.Context, data map[string]any) (string, error) {
	var prompt string
	err := workflow.ExecuteActivity(p.workflowCtx, p.prefix+"_GetPromptActivity", data).Get(p.workflowCtx, &prompt)
	if err != nil {
		return "", err
	}

	return prompt, nil
}
