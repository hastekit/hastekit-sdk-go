package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleEmbeddingsRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *embeddings.Request) (*embeddings.Response, error) {
	ctx, span := tracer.Start(ctx, genai.OpEmbeddings+" "+in.Model)
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpEmbeddings),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeEmbeddings),
	)
	if in.EncodingFormat != nil {
		span.SetAttributes(attribute.StringSlice(genai.AttrRequestEncodingFormats, []string{*in.EncodingFormat}))
	}

	out, err := p.NewEmbedding(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String(genai.AttrResponseModel, out.Model))
	if out.Usage != nil {
		// Embeddings only consume input tokens.
		span.SetAttributes(attribute.Int64(genai.AttrUsageInputTokens, out.Usage.PromptTokens))
	}

	return out, nil
}
