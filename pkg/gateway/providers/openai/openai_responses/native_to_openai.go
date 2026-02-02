package openai_responses

import (
	responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

func NativeRequestToRequest(in *responses2.Request) *Request {
	return &Request{
		*in,
	}
}

func NativeResponseToResponse(in *responses2.Response) *Response {
	return &Response{
		*in,
	}
}
