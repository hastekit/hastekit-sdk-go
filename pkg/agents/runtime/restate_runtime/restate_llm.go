package restate_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	restate "github.com/restatedev/sdk-go"
)

type RestateLLM struct {
	restateCtx        restate.WorkflowContext
	wrappedLLM        llm.Provider
	providerConfigKey string
}

func NewRestateLLM(restateCtx restate.WorkflowContext, wrappedLLM llm.Provider, providerConfigKey string) agents.LLM {
	return &RestateLLM{
		restateCtx:        restateCtx,
		wrappedLLM:        wrappedLLM,
		providerConfigKey: providerConfigKey,
	}
}

func (l *RestateLLM) NewStreamingResponses(ctx context.Context, in *responses.Request, cb func(chunk *responses.ResponseChunk)) (*responses.Response, error) {
	return restate.Run(l.restateCtx, func(ctx restate.RunContext) (*responses.Response, error) {
		// The Restate RunContext is minted fresh and does not inherit the
		// caller's context values, so re-establish the provider config key
		// that gateway.ProviderConfigKeyFromContext (in the LLM client) reads.
		runCtx := gateway.WithProviderConfigKey(ctx, l.providerConfigKey)
		stream, err := l.wrappedLLM.NewStreamingResponses(runCtx, in)
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
