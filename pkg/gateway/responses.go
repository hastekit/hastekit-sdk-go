package gateway

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleResponsesRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *responses.Request) (*responses.Response, error) {
	ctx, span := tracer.Start(ctx, "LLM.Responses")
	defer span.End()

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.Bool("llm.streaming", false),
		attribute.Int("llm.tools_count", len(in.Tools)),
		attribute.String("llm.request_type", "Responses"),
	)

	out, err := p.NewResponses(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Add output attributes
	if out.Usage != nil {
		span.SetAttributes(
			attribute.Int("llm.usage.input_tokens", int(out.Usage.InputTokens)),
			attribute.Int("llm.usage.output_tokens", int(out.Usage.OutputTokens)),
			attribute.Int("llm.usage.total_tokens", int(out.Usage.TotalTokens)),
		)
	}

	return out, nil
}

func (g *LLMGateway) handleStreamingResponsesRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *responses.Request) (chan *responses.ResponseChunk, error) {
	_, span := tracer.Start(ctx, "LLM.StreamingResponses")

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "Streaming Responses"),
		attribute.Int("tools_count", len(in.Tools)),
	)

	if in.Instructions != nil {
		span.SetAttributes(attribute.String("gen_ai.system_instructions", *in.Instructions))
	}

	msgsString, err := sonic.Marshal(in.Input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.String("gen_ai.input.messages", string(msgsString)))

	streamChan, err := p.NewStreamingResponses(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, err
	}

	// Wrap the channel to track completion and end span
	wrappedChan := make(chan *responses.ResponseChunk)
	go func() {
		defer span.End()
		defer close(wrappedChan)

		chunkCount := 0
		for chunk := range streamChan {
			chunkCount++
			wrappedChan <- chunk

			if chunk.OfResponseCompleted != nil {
				span.SetAttributes(attribute.Int("gen_ai.response.usage.input_tokens", chunk.OfResponseCompleted.Response.Usage.InputTokens))
				span.SetAttributes(attribute.Int("gen_ai.response.usage.cached_input_tokens", chunk.OfResponseCompleted.Response.Usage.InputTokensDetails.CachedTokens))
				span.SetAttributes(attribute.Int("gen_ai.response.usage.output_tokens", chunk.OfResponseCompleted.Response.Usage.OutputTokens))
				span.SetAttributes(attribute.Int("gen_ai.response.usage.total_tokens", chunk.OfResponseCompleted.Response.Usage.TotalTokens))

				msgsString, err = sonic.Marshal(chunk.OfResponseCompleted.Response.Output)
				if err != nil {
					span.RecordError(err)
				}
				span.SetAttributes(attribute.String("gen_ai.output.messages", string(msgsString)))
			}
		}
	}()

	return wrappedChan, nil
}
