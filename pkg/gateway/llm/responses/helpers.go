package responses

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/constants"
)

func UserMessage(msg string) InputMessageUnion {
	return InputMessageUnion{
		OfInputMessage: &InputMessage{Role: constants.RoleUser, Content: InputContent{{OfInputText: &InputTextContent{Text: msg}}}},
	}
}
