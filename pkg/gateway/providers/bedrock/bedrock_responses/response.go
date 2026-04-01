package bedrock_responses

import (
	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/constants"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// ConverseResponse is the response from the Bedrock Converse API.
type ConverseResponse struct {
	Output     ConverseOutput   `json:"output"`
	StopReason string           `json:"stopReason"` // "end_turn", "tool_use", "max_tokens", "stop_sequence", "content_filtered"
	Usage      ConverseUsage    `json:"usage"`
	Metrics    *ConverseMetrics `json:"metrics,omitempty"`
}

type ConverseOutput struct {
	Message *ConverseMessage `json:"message,omitempty"`
}

type ConverseUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

type ConverseMetrics struct {
	LatencyMs int `json:"latencyMs"`
}

// ToNativeResponse converts a Bedrock Converse response to the native SDK response format.
func (r *ConverseResponse) ToNativeResponse(model string) *responses.Response {
	var output []responses.OutputMessageUnion

	if r.Output.Message != nil {
		for _, block := range r.Output.Message.Content {
			if block.Text != nil {
				output = append(output, responses.OutputMessageUnion{
					OfOutputMessage: &responses.OutputMessage{
						ID:   responses.NewOutputItemMessageID(),
						Role: constants.RoleAssistant,
						Content: &responses.OutputContent{
							{
								OfOutputText: &responses.OutputTextContent{
									Text:        *block.Text,
									Annotations: []responses.Annotation{},
								},
							},
						},
					},
				})
			}

			if block.ToolUse != nil {
				args, err := sonic.Marshal(block.ToolUse.Input)
				if err != nil {
					args = []byte("{}")
				}

				output = append(output, responses.OutputMessageUnion{
					OfFunctionCall: &responses.FunctionCallMessage{
						ID:        responses.NewOutputItemFunctionCallID(),
						CallID:    block.ToolUse.ToolUseId,
						Name:      block.ToolUse.Name,
						Arguments: string(args),
					},
				})
			}
			if block.ReasoningContent != nil {
				rm := &responses.ReasoningMessage{
					ID: responses.NewOutputItemReasoningID(),
				}
				if block.ReasoningContent.ReasoningText != nil {
					rm.Summary = []responses.SummaryTextContent{
						{Text: block.ReasoningContent.ReasoningText.Text},
					}
					if block.ReasoningContent.ReasoningText.Signature != "" {
						sig := block.ReasoningContent.ReasoningText.Signature
						rm.EncryptedContent = &sig
					}
				}
				if block.ReasoningContent.RedactedContent != nil {
					rm.EncryptedContent = block.ReasoningContent.RedactedContent
				}
				output = append(output, responses.OutputMessageUnion{
					OfReasoning: rm,
				})
			}
		}
	}

	return &responses.Response{
		ID:     responses.NewOutputItemMessageID(),
		Model:  model,
		Output: output,
		Usage: &responses.Usage{
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
			TotalTokens:  r.Usage.TotalTokens,
		},
		Metadata: map[string]any{
			"stop_reason": r.StopReason,
		},
	}
}
