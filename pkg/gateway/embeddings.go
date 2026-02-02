package gateway

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleEmbeddingsRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *embeddings.Request) (*embeddings.Response, error) {
	ctx, span := tracer.Start(ctx, "LLM.Embeddings")
	defer span.End()

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
	)

	out, err := p.NewEmbedding(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if out.Usage != nil {
		span.SetAttributes(
			attribute.Int64("llm.usage.prompt_tokens", out.Usage.PromptTokens),
			attribute.Int64("llm.usage.total_tokens", out.Usage.TotalTokens),
		)
	}

	return out, nil
}
