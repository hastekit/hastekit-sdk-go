package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/speech"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleSpeechRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *speech.Request) (*speech.Response, error) {
	ctx, span := tracer.Start(ctx, genai.OpSpeech+" "+in.Model)
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpSpeech),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeSpeech),
	)

	out, err := p.NewSpeech(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return out, nil
}

func (g *LLMGateway) handleStreamingSpeechRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *speech.Request) (chan *speech.ResponseChunk, error) {
	ctx, span := tracer.Start(ctx, genai.OpSpeech+" "+in.Model)
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpSpeech),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeSpeechStream),
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
				span.SetAttributes(attribute.Int(genai.AttrUsageInputTokens, chunk.OfAudioDone.Usage.InputTokens))
				span.SetAttributes(attribute.Int(genai.AttrCachedInputTokens, chunk.OfAudioDone.Usage.InputTokensDetails.CachedTokens))
				span.SetAttributes(attribute.Int(genai.AttrUsageOutputTokens, chunk.OfAudioDone.Usage.OutputTokens))
			}
		}
	}()

	return wrappedChan, nil
}
