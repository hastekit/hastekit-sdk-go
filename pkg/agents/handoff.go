package agents

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type HandoffTarget interface {
	ExecuteWithRun(ctx context.Context, in *AgentInput, cb func(chunk *responses.ResponseChunk), run *history.ConversationRunManager) (*AgentOutput, error)
}

type Handoff struct {
	Name        string
	Description string
	Agent       HandoffTarget `json:"-"`
}

func NewHandoff(name string, desc string, agent *Agent) *Handoff {
	return &Handoff{
		Name:        name,
		Description: desc,
		Agent:       agent,
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
