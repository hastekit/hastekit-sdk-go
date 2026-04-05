package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/image_edit"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleImageEditRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *image_edit.Request) (*image_edit.Response, error) {
	ctx, span := tracer.Start(ctx, "LLM.ImageEdit")
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "ImageEdit"),
	)

	out, err := p.NewImageEdit(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return out, nil
}
