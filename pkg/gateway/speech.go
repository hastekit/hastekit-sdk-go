package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/speech"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleSpeechRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *speech.Request) (*speech.Response, error) {
	ctx, span := tracer.Start(ctx, "LLM.Speech")
	defer span.End()

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "Speech"),
	)

	out, err := p.NewSpeech(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Add output attributes
	//if out.Usage.TotalTokens > 0 {
	//	span.SetAttributes(
	//		attribute.Int64("llm.usage.prompt_tokens", out.Usage.PromptTokens),
	//		attribute.Int64("llm.usage.completion_tokens", out.Usage.CompletionTokens),
	//		attribute.Int64("llm.usage.total_tokens", out.Usage.TotalTokens),
	//	)
	//}

	return out, nil
}

func (g *LLMGateway) handleStreamingSpeechRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *speech.Request) (chan *speech.ResponseChunk, error) {
	ctx, span := tracer.Start(ctx, "LLM.Speech")
	defer span.End()

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "Speech"),
	)

	streamChan, err := p.NewStreamingSpeech(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Wrap the channel to track completion and end span
	wrappedChan := make(chan *speech.ResponseChunk)
	go func() {
		defer span.End()
		defer close(wrappedChan)

		chunkCount := 0
		for chunk := range streamChan {
			chunkCount++
			wrappedChan <- chunk

			if chunk.OfAudioDone != nil {
				span.SetAttributes(attribute.Int("gen_ai.response.usage.input_tokens", chunk.OfAudioDone.Usage.InputTokens))
				span.SetAttributes(attribute.Int("gen_ai.response.usage.cached_input_tokens", chunk.OfAudioDone.Usage.InputTokensDetails.CachedTokens))
				span.SetAttributes(attribute.Int("gen_ai.response.usage.output_tokens", chunk.OfAudioDone.Usage.OutputTokens))
				span.SetAttributes(attribute.Int("gen_ai.response.usage.total_tokens", chunk.OfAudioDone.Usage.TotalTokens))
			}
		}
	}()

	return wrappedChan, nil
}
