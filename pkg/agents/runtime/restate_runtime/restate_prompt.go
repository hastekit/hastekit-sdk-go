package restate_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	restate "github.com/restatedev/sdk-go"
)

type RestatePrompt struct {
	restateCtx    restate.WorkflowContext
	wrappedPrompt agents.SystemPromptProvider
}

func NewRestatePrompt(restateCtx restate.WorkflowContext, instruction agents.SystemPromptProvider) agents.SystemPromptProvider {
	return &RestatePrompt{
		restateCtx:    restateCtx,
		wrappedPrompt: instruction,
	}
}

func (r *RestatePrompt) GetPrompt(ctx context.Context, runContext map[string]any) (string, error) {
	return restate.Run(r.restateCtx, func(ctx restate.RunContext) (string, error) {
		return r.wrappedPrompt.GetPrompt(ctx, runContext)
	}, restate.WithName("GetPrompt"))
}
