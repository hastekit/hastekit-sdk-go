package gateway

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleResponsesRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *responses.Request) (*responses.Response, error) {
	ctx, span := tracer.Start(ctx, genai.OpChat+" "+in.Model)
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpChat),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeResponses),
	)
	if in.Temperature != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTemperature, *in.Temperature))
	}
	if in.TopP != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTopP, *in.TopP))
	}
	if in.MaxOutputTokens != nil {
		span.SetAttributes(attribute.Int(genai.AttrRequestMaxTokens, *in.MaxOutputTokens))
	}
	if in.Instructions != nil {
		span.SetAttributes(attribute.String(genai.AttrSystemInstructions, *in.Instructions))
	}
	if msgsString, err := sonic.Marshal(in.Input); err == nil {
		span.SetAttributes(attribute.String(genai.AttrInputMessages, string(msgsString)))
	}

	out, err := p.NewResponses(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String(genai.AttrResponseModel, out.Model))
	if out.ID != "" {
		span.SetAttributes(attribute.String(genai.AttrResponseID, out.ID))
	}
	if outString, err := sonic.Marshal(out.Output); err == nil {
		span.SetAttributes(attribute.String(genai.AttrOutputMessages, string(outString)))
	}
	if out.Usage != nil {
		span.SetAttributes(
			attribute.Int(genai.AttrUsageInputTokens, int(out.Usage.InputTokens)),
			attribute.Int(genai.AttrUsageOutputTokens, int(out.Usage.OutputTokens)),
		)
	}

	return out, nil
}

func (g *LLMGateway) handleStreamingResponsesRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *responses.Request) (chan *responses.ResponseChunk, error) {
	ctx, span := tracer.Start(ctx, genai.OpChat+" "+in.Model)

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpChat),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeResponsesStream),
	)
	if in.Temperature != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTemperature, *in.Temperature))
	}
	if in.TopP != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTopP, *in.TopP))
	}
	if in.MaxOutputTokens != nil {
		span.SetAttributes(attribute.Int(genai.AttrRequestMaxTokens, *in.MaxOutputTokens))
	}
	if in.Instructions != nil {
		span.SetAttributes(attribute.String(genai.AttrSystemInstructions, *in.Instructions))
	}

	msgsString, err := sonic.Marshal(in.Input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.String(genai.AttrInputMessages, string(msgsString)))

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
				span.SetAttributes(attribute.Int(genai.AttrUsageInputTokens, chunk.OfResponseCompleted.Response.Usage.InputTokens))
				span.SetAttributes(attribute.Int(genai.AttrCachedInputTokens, chunk.OfResponseCompleted.Response.Usage.InputTokensDetails.CachedTokens))
				span.SetAttributes(attribute.Int(genai.AttrUsageOutputTokens, chunk.OfResponseCompleted.Response.Usage.OutputTokens))

				msgsString, err = sonic.Marshal(chunk.OfResponseCompleted.Response.Output)
				if err != nil {
					span.RecordError(err)
				}
				span.SetAttributes(attribute.String(genai.AttrOutputMessages, string(msgsString)))
			}
		}
	}()

	return wrappedChan, nil
}
