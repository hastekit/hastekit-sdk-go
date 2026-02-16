package agents

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type Handoff struct {
	Description string
	Agent       *Agent
}

func NewHandoff(agent *Agent, desc string) *Handoff {
	return &Handoff{
		Agent:       agent,
		Description: desc,
	}
}

type HandoffTool struct {
	*BaseTool
	agent *Agent
}

func NewHandoffTool(t *responses.ToolUnion) *HandoffTool {
	return &HandoffTool{
		BaseTool: &BaseTool{
			ToolUnion: *t,
		},
	}
}

func (t *HandoffTool) Execute(ctx context.Context, params *ToolCall) (*responses.FunctionCallOutputMessage, error) {
	return nil, nil
}
