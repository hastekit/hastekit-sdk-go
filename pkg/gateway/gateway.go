package gateway

import (
	"context"
	"errors"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("LLMGateway")

// ConfigStore is the interface required by LLMGateway to get provider and virtual key configurations.
type ConfigStore interface {
	// GetProviderConfig returns provider configuration and associated API keys.
	GetProviderConfig(providerName llm.ProviderName) (*ProviderConfig, error)

	// GetVirtualKey returns virtual key configuration for access control.
	GetVirtualKey(secretKey string) (*VirtualKeyConfig, error)
}

type LLMGateway struct {
	ConfigStore ConfigStore
	middlewares []Middleware
}

func NewLLMGateway(ConfigStore ConfigStore) *LLMGateway {
	return &LLMGateway{
		ConfigStore: ConfigStore,
		middlewares: []Middleware{},
	}
}

func (g *LLMGateway) UseMiddleware(middleware ...Middleware) {
	g.middlewares = append(g.middlewares, middleware...)
}

func (g *LLMGateway) HandleRequest(ctx context.Context, providerName llm.ProviderName, key string, r *llm.Request) (*llm.Response, error) {
	// Build the middleware chain
	handler := g.baseRequestHandler
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		handler = g.middlewares[i].HandleRequest(handler)
	}

	// Execute the handler through the middleware chain
	return handler(ctx, providerName, key, r)
}

// baseRequestHandler contains the core request handling logic
func (g *LLMGateway) baseRequestHandler(ctx context.Context, providerName llm.ProviderName, key string, r *llm.Request) (*llm.Response, error) {
	// Construct the provider
	p, err := g.getProvider(ctx, providerName, r, key)
	if err != nil {
		return nil, err
	}

	// Create the response
	resp := &llm.Response{}

	switch {
	case r.OfEmbeddingsInput != nil:
		respOut, err := g.handleEmbeddingsRequest(ctx, providerName, p, r.OfEmbeddingsInput)
		if err != nil {
			return nil, err
		}
		resp.OfEmbeddingsOutput = respOut
	case r.OfResponsesInput != nil:
		respOut, err := g.handleResponsesRequest(ctx, providerName, p, r.OfResponsesInput)
		if err != nil {
			return nil, err
		}

		resp.OfResponsesOutput = respOut
	case r.OfChatCompletionInput != nil:
		respOut, err := g.handleChatCompletionRequest(ctx, providerName, p, r.OfChatCompletionInput)
		if err != nil {
			return nil, err
		}

		resp.OfChatCompletionOutput = respOut
	case r.OfSpeech != nil:
		respOut, err := g.handleSpeechRequest(ctx, providerName, p, r.OfSpeech)
		if err != nil {
			return nil, err
		}

		resp.OfSpeech = respOut
	}

	return resp, nil
}

func (g *LLMGateway) HandleStreamingRequest(ctx context.Context, providerName llm.ProviderName, key string, r *llm.Request) (*llm.StreamingResponse, error) {
	// Build the middleware chain
	handler := g.baseStreamingRequestHandler
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		handler = g.middlewares[i].HandleStreamingRequest(handler)
	}

	// Execute the handler through the middleware chain
	return handler(ctx, providerName, key, r)
}

// baseStreamingRequestHandler contains the core streaming request handling logic
func (g *LLMGateway) baseStreamingRequestHandler(ctx context.Context, providerName llm.ProviderName, key string, r *llm.Request) (*llm.StreamingResponse, error) {
	// Construct the provider
	p, err := g.getProvider(ctx, providerName, r, key)
	if err != nil {
		return nil, err
	}

	// Create the response
	resp := &llm.StreamingResponse{}

	switch {
	case r.OfResponsesInput != nil:
		respOut, err := g.handleStreamingResponsesRequest(ctx, providerName, p, r.OfResponsesInput)
		if err != nil {
			return nil, err
		}

		resp.ResponsesStreamData = respOut
		return resp, nil

	case r.OfChatCompletionInput != nil:
		respOut, err := g.handleStreamingChatCompletionRequest(ctx, providerName, p, r.OfChatCompletionInput)
		if err != nil {
			return nil, err
		}

		resp.ChatCompletionStreamData = respOut
		return resp, nil

	case r.OfSpeech != nil:
		respOut, err := g.handleStreamingSpeechRequest(ctx, providerName, p, r.OfSpeech)
		if err != nil {
			return nil, err
		}

		resp.SpeechStreamData = respOut
		return resp, nil
	}

	return nil, errors.New("invalid request")
}
