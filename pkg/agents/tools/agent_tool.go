package tools

import (
	"context"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/constants"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type SubAgentContextMode string

const (
	SubAgentContextModeNone     SubAgentContextMode = "None"
	SubAgentContextModeIsolated SubAgentContextMode = "Isolated"
)

type AgentTool struct {
	*agents.BaseTool
	agent       *agents.Agent
	contextMode SubAgentContextMode
}

func NewAgentTool(t *responses.ToolUnion, agent *agents.Agent, contextMode SubAgentContextMode) *AgentTool {
	return &AgentTool{
		BaseTool: &agents.BaseTool{
			ToolUnion: *t,
		},
		agent:       agent,
		contextMode: contextMode,
	}
}

func (t *AgentTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	namespace := params.Namespace + "/" + params.Name

	var threadId string
	if t.contextMode == SubAgentContextModeIsolated {
		agentToolContextId, exists := params.SubAgentContext[t.agent.Name]
		if exists {
			threadId = agentToolContextId
		}
	} else {
		threadId = uuid.NewString()
	}

	result, err := t.agent.Execute(ctx, &agents.AgentInput{
		Namespace: namespace,
		ThreadID:  threadId,
		Messages: []responses.InputMessageUnion{
			{
				OfEasyInput: &responses.EasyMessage{
					Role:    constants.RoleUser,
					Content: responses.EasyInputContentUnion{OfString: &params.Arguments},
				},
			},
		},
		SessionID: params.SessionID, // Using conversation id as the shared session id
	})
	if err != nil {
		return nil, err
	}

	data := ""
	for _, out := range result.Output {
		if out.OfOutputMessage != nil {
			for _, content := range *out.OfOutputMessage.Content {
				if content.OfOutputText != nil {
					data += content.OfOutputText.Text
				}
			}
		}

		if out.OfEasyInput != nil {
			if out.OfEasyInput.Content.OfString != nil {
				data += *out.OfEasyInput.Content.OfString
			}

			if out.OfEasyInput.Content.OfInputMessageList != nil {
				for _, message := range out.OfEasyInput.Content.OfInputMessageList {
					if message.OfOutputText != nil {
						data += message.OfOutputText.Text
					}
				}
			}
		}
	}

	return &agents.ToolCallResponse{
		FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
			ID:     params.ID,
			CallID: params.CallID,
			Output: responses.FunctionCallOutputContentUnion{
				OfString: utils.Ptr(data),
			},
		},
		SubAgentContext: map[string]string{
			t.agent.Name: threadId,
		},
	}, nil
}
