package openai_chat_completion

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/chat_completion"
)

type ResponseChunk struct {
	chat_completion.ResponseChunk
}

func (in *ResponseChunk) ToNativeResponseChunk() *chat_completion.ResponseChunk {
	return &in.ResponseChunk
}
