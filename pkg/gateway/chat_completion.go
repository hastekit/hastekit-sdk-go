package gateway

import (
	"context"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/chat_completion"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleChatCompletionRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *chat_completion.Request) (*chat_completion.Response, error) {
	ctx, span := tracer.Start(ctx, "LLM.ChatCompletion")
	defer span.End()

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "Chat Completion"),
	)

	out, err := p.NewChatCompletion(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Add output attributes
	if out.Usage.TotalTokens > 0 {
		span.SetAttributes(
			attribute.Int64("llm.usage.prompt_tokens", out.Usage.PromptTokens),
			attribute.Int64("llm.usage.completion_tokens", out.Usage.CompletionTokens),
			attribute.Int64("llm.usage.total_tokens", out.Usage.TotalTokens),
		)
	}

	return out, nil
}

func (g *LLMGateway) handleStreamingChatCompletionRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *chat_completion.Request) (chan *chat_completion.ResponseChunk, error) {
	_, span := tracer.Start(ctx, "LLM.StreamingChatCompletion")

	span.SetAttributes(
		attribute.String("llm.provider", string(providerName)),
		attribute.String("llm.model", in.Model),
		attribute.String("llm.request_type", "Streaming Chat Completion"),
		//attribute.Int("tools_count", len(in.Tools)),
	)

	msgsString, err := sonic.Marshal(in.Messages)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.String("gen_ai.input.messages", string(msgsString)))

	streamChan, err := p.NewStreamingChatCompletion(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.End()
		return nil, err
	}

	// Wrap the channel to track completion and end span
	wrappedChan := make(chan *chat_completion.ResponseChunk)
	go func() {
		defer span.End()
		defer close(wrappedChan)

		chunkCount := 0
		var msgString strings.Builder
		defer func() {
			span.SetAttributes(attribute.String("gen_ai.output.messages", msgString.String()))
		}()

		for chunk := range streamChan {
			chunkCount++
			wrappedChan <- chunk

			if chunk.OfChatCompletionChunk != nil {
				if chunk.OfChatCompletionChunk.Usage != nil {
					span.SetAttributes(attribute.Int64("gen_ai.response.usage.input_tokens", chunk.OfChatCompletionChunk.Usage.PromptTokens))
					span.SetAttributes(attribute.Int64("gen_ai.response.usage.cached_input_tokens", chunk.OfChatCompletionChunk.Usage.PromptTokensDetails.CachedTokens))
					span.SetAttributes(attribute.Int64("gen_ai.response.usage.output_tokens", chunk.OfChatCompletionChunk.Usage.CompletionTokens))
					span.SetAttributes(attribute.Int64("gen_ai.response.usage.total_tokens", chunk.OfChatCompletionChunk.Usage.TotalTokens))
				}

				if len(chunk.OfChatCompletionChunk.Choices) > 0 {
					msgString.WriteString(chunk.OfChatCompletionChunk.Choices[0].Delta.Content)
				}
			}
		}
	}()

	return wrappedChan, nil
}
