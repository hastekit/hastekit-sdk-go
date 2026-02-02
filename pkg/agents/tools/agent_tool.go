package tools

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type AgentTool struct {
	*agents.BaseTool
	agent *agents.Agent
}

func NewAgentTool(t *responses.ToolUnion, agent *agents.Agent) *AgentTool {
	return &AgentTool{
		BaseTool: &agents.BaseTool{
			ToolUnion: *t,
		},
		agent: agent,
	}
}

func (t *AgentTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	result, err := t.agent.Execute(ctx, &agents.AgentInput{
		Messages: []responses.InputMessageUnion{
			{
				OfEasyInput: &responses.EasyMessage{
					Content: responses.EasyInputContentUnion{OfString: &params.Arguments},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	data := ""
	for _, out := range result.Output {
		if out.OfOutputMessage != nil {
			for _, content := range out.OfOutputMessage.Content {
				if content.OfOutputText != nil {
					data += content.OfOutputText.Text
				}
			}
		}
	}

	return &responses.FunctionCallOutputMessage{
		ID:     params.ID,
		CallID: params.CallID,
		Output: responses.FunctionCallOutputContentUnion{
			OfString: utils.Ptr(data),
		},
	}, nil
}
