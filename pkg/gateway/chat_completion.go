package gateway

import (
	"context"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/chat_completion"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) handleChatCompletionRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *chat_completion.Request) (*chat_completion.Response, error) {
	ctx, span := tracer.Start(ctx, genai.OpChat+" "+in.Model)
	defer span.End()

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpChat),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeChat),
	)
	if in.Temperature != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTemperature, *in.Temperature))
	}
	if in.TopP != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTopP, *in.TopP))
	}
	if in.FrequencyPenalty != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestFrequencyPenalty, *in.FrequencyPenalty))
	}
	if in.PresencePenalty != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestPresencePenalty, *in.PresencePenalty))
	}
	if in.Seed != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestSeed, *in.Seed))
	}
	if in.N != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestChoiceCount, *in.N))
	}
	if in.MaxCompletionTokens != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestMaxTokens, *in.MaxCompletionTokens))
	} else if in.MaxTokens != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestMaxTokens, *in.MaxTokens))
	}
	if in.Stop != nil && len(in.Stop.OfList) > 0 {
		span.SetAttributes(attribute.StringSlice(genai.AttrRequestStopSequences, in.Stop.OfList))
	} else if in.Stop != nil && in.Stop.OfString != nil {
		span.SetAttributes(attribute.StringSlice(genai.AttrRequestStopSequences, []string{*in.Stop.OfString}))
	}
	if msgsString, err := sonic.Marshal(in.Messages); err == nil {
		span.SetAttributes(attribute.String(genai.AttrInputMessages, string(msgsString)))
	}

	out, err := p.NewChatCompletion(ctx, in)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.String(genai.AttrResponseModel, out.Model))
	if out.ID != "" {
		span.SetAttributes(attribute.String(genai.AttrResponseID, out.ID))
	}
	if len(out.Choices) > 0 {
		finishReasons := make([]string, 0, len(out.Choices))
		for _, c := range out.Choices {
			finishReasons = append(finishReasons, c.FinishReason)
		}
		span.SetAttributes(attribute.StringSlice(genai.AttrFinishReasons, finishReasons))
	}
	if outString, err := sonic.Marshal(out.Choices); err == nil {
		span.SetAttributes(attribute.String(genai.AttrOutputMessages, string(outString)))
	}
	if out.Usage.TotalTokens > 0 {
		span.SetAttributes(
			attribute.Int64(genai.AttrUsageInputTokens, out.Usage.PromptTokens),
			attribute.Int64(genai.AttrUsageOutputTokens, out.Usage.CompletionTokens),
		)
	}

	return out, nil
}

func (g *LLMGateway) handleStreamingChatCompletionRequest(ctx context.Context, providerName llm.ProviderName, p llm.Provider, in *chat_completion.Request) (chan *chat_completion.ResponseChunk, error) {
	ctx, span := tracer.Start(ctx, genai.OpChat+" "+in.Model)

	addToSpan(ctx, span)
	span.SetAttributes(
		attribute.String(genai.AttrOperationName, genai.OpChat),
		attribute.String(genai.AttrProviderName, string(providerName)),
		attribute.String(genai.AttrRequestModel, in.Model),
		attribute.String(genai.AttrRequestType, genai.RequestTypeChatStream),
	)
	if in.Temperature != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTemperature, *in.Temperature))
	}
	if in.TopP != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestTopP, *in.TopP))
	}
	if in.FrequencyPenalty != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestFrequencyPenalty, *in.FrequencyPenalty))
	}
	if in.PresencePenalty != nil {
		span.SetAttributes(attribute.Float64(genai.AttrRequestPresencePenalty, *in.PresencePenalty))
	}
	if in.Seed != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestSeed, *in.Seed))
	}
	if in.N != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestChoiceCount, *in.N))
	}
	if in.MaxCompletionTokens != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestMaxTokens, *in.MaxCompletionTokens))
	} else if in.MaxTokens != nil {
		span.SetAttributes(attribute.Int64(genai.AttrRequestMaxTokens, *in.MaxTokens))
	}
	if in.Stop != nil && len(in.Stop.OfList) > 0 {
		span.SetAttributes(attribute.StringSlice(genai.AttrRequestStopSequences, in.Stop.OfList))
	} else if in.Stop != nil && in.Stop.OfString != nil {
		span.SetAttributes(attribute.StringSlice(genai.AttrRequestStopSequences, []string{*in.Stop.OfString}))
	}

	msgsString, err := sonic.Marshal(in.Messages)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.String(genai.AttrInputMessages, string(msgsString)))

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
			span.SetAttributes(attribute.String(genai.AttrOutputMessages, msgString.String()))
		}()

		for chunk := range streamChan {
			chunkCount++
			wrappedChan <- chunk

			if chunk.OfChatCompletionChunk != nil {
				if chunk.OfChatCompletionChunk.Usage != nil {
					span.SetAttributes(attribute.Int64(genai.AttrUsageInputTokens, chunk.OfChatCompletionChunk.Usage.PromptTokens))
					span.SetAttributes(attribute.Int64(genai.AttrCachedInputTokens, chunk.OfChatCompletionChunk.Usage.PromptTokensDetails.CachedTokens))
					span.SetAttributes(attribute.Int64(genai.AttrUsageOutputTokens, chunk.OfChatCompletionChunk.Usage.CompletionTokens))
				}

				if len(chunk.OfChatCompletionChunk.Choices) > 0 {
					msgString.WriteString(chunk.OfChatCompletionChunk.Choices[0].Delta.Content)
				}
			}
		}
	}()

	return wrappedChan, nil
}
