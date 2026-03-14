package llm

import (
	"context"
	"slices"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/chat_completion"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/speech"
)

type Provider interface {
	NewResponses(ctx context.Context, in *responses.Request) (*responses.Response, error)
	NewStreamingResponses(ctx context.Context, in *responses.Request) (chan *responses.ResponseChunk, error)
	NewEmbedding(ctx context.Context, in *embeddings.Request) (*embeddings.Response, error)
	NewChatCompletion(ctx context.Context, in *chat_completion.Request) (*chat_completion.Response, error)
	NewStreamingChatCompletion(ctx context.Context, in *chat_completion.Request) (chan *chat_completion.ResponseChunk, error)
	NewSpeech(ctx context.Context, in *speech.Request) (*speech.Response, error)
	NewStreamingSpeech(ctx context.Context, in *speech.Request) (chan *speech.ResponseChunk, error)
}

type ProviderName string

var (
	ProviderNameOpenAI    ProviderName = "OpenAI"
	ProviderNameAnthropic ProviderName = "Anthropic"
	ProviderNameGemini    ProviderName = "Gemini"
	ProviderNameXAI       ProviderName = "xAI"
	ProviderNameOllama    ProviderName = "Ollama"
)

func GetAllProviderNames() []ProviderName {
	return []ProviderName{
		ProviderNameOpenAI,
		ProviderNameAnthropic,
		ProviderNameGemini,
		ProviderNameXAI,
		ProviderNameOllama,
	}
}

func (p *ProviderName) IsValid() bool {
	return slices.Contains(GetAllProviderNames(), *p)
}
