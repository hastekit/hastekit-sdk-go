package xai_responses

import (
	responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/openai/openai_responses"
)

func NativeRequestToRequest(in *responses2.Request) *Request {
	r := &Request{
		Request: &openai_responses.Request{
			*in,
		},
	}

	// Grok doesn't support reasoning effort except for older models like grok-3
	if in.Reasoning != nil {
		r.Reasoning.Effort = nil
	}

	return r
}

func NativeResponseToResponse(in *responses2.Response) *Response {
	return &Response{
		*in,
	}
}
