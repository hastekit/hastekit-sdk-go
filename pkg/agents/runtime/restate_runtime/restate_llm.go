package restate_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	restate "github.com/restatedev/sdk-go"
)

type RestateLLM struct {
	restateCtx restate.WorkflowContext
	wrappedLLM llm.Provider
}

func NewRestateLLM(restateCtx restate.WorkflowContext, wrappedLLM llm.Provider) agents.LLM {
	return &RestateLLM{
		restateCtx: restateCtx,
		wrappedLLM: wrappedLLM,
	}
}

func (l *RestateLLM) NewStreamingResponses(ctx context.Context, in *responses.Request, cb func(chunk *responses.ResponseChunk)) (*responses.Response, error) {
	return restate.Run(l.restateCtx, func(ctx restate.RunContext) (*responses.Response, error) {
		stream, err := l.wrappedLLM.NewStreamingResponses(ctx, in)
		if err != nil {
			return nil, err
		}

		acc := agents.Accumulator{}
		resp, err := acc.ReadStream(stream, func(chunk *responses.ResponseChunk) {
			cb(chunk)
		})
		if err != nil {
			return nil, err
		}

		return resp, nil
	}, restate.WithName("LLMCall"))
}
