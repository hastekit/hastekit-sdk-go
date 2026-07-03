package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/image_generation"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleImageGenerationRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *image_generation.Request) (*image_generation.Response, error) {
	ctx, span := tracer.Start(ctx, genai.OpImageGeneration+" "+in.Model)
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpImageGeneration),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeImageGeneration),
	)

	out, err := p.NewImageGeneration(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return out, nil
}
