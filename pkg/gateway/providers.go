package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/anthropic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/gemini"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/openai"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/xai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func (g *LLMGateway) getProvider(ctx context.Context, providerName llm.ProviderName, req *llm.Request, key string) (llm.Provider, error) {
	_, span := tracer.Start(ctx, "Gateway.GetProvider")
	defer span.End()

	var baseUrl string
	var customHeaders map[string]string

	providerConfig, err := g.ConfigStore.GetProviderConfig(providerName)
	if err != nil {
		err = errors.New("failed to get provider config")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if providerConfig != nil {
		baseUrl = providerConfig.BaseURL
		customHeaders = providerConfig.CustomHeaders
	}

	span.SetAttributes(attribute.String("base_url", baseUrl))

	switch providerName {
	case llm.ProviderNameOpenAI:
		return openai.NewClient(&openai.ClientOptions{
			BaseURL: baseUrl,
			ApiKey:  key,
			Headers: customHeaders,
		}), nil

	case llm.ProviderNameAnthropic:
		return anthropic.NewClient(&anthropic.ClientOptions{
			BaseURL: baseUrl,
			ApiKey:  key,
			Headers: customHeaders,
		}), nil

	case llm.ProviderNameGemini:
		return gemini.NewClient(&gemini.ClientOptions{
			BaseURL: baseUrl,
			ApiKey:  key,
			Headers: customHeaders,
		}), nil

	case llm.ProviderNameXAI:
		return xai.NewClient(&xai.ClientOptions{
			BaseURL: baseUrl,
			ApiKey:  key,
			Headers: customHeaders,
		}), nil
	case llm.ProviderNameOllama:
		return openai.NewClient(&openai.ClientOptions{
			BaseURL: baseUrl,
			ApiKey:  key,
			Headers: customHeaders,
		}), nil
	}

	return nil, fmt.Errorf("unknown provider: %s", providerName)
}
