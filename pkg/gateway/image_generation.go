package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/image_generation"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleImageGenerationRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *image_generation.Request) (*image_generation.Response, error) {
	ctx, span := tracer.Start(ctx, "LLM.ImageGeneration")
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "ImageGeneration"),
	)

	out, err := p.NewImageGeneration(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return out, nil
}
