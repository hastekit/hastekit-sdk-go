package base

import (
	"context"

	chat_completion2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/chat_completion"
	embeddings2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"
	responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	speech2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/speech"
)

type BaseProvider struct{}

func (bp *BaseProvider) NewResponses(ctx context.Context, in *responses2.Request) (*responses2.Response, error) {
	panic("implement me")
}

func (bp *BaseProvider) NewStreamingResponses(ctx context.Context, in *responses2.Request) (chan *responses2.ResponseChunk, error) {
	panic("implement me")
}

func (bp *BaseProvider) NewEmbedding(ctx context.Context, in *embeddings2.Request) (*embeddings2.Response, error) {
	panic("implement me")
}

func (bp *BaseProvider) NewChatCompletion(ctx context.Context, in *chat_completion2.Request) (*chat_completion2.Response, error) {
	panic("implement me")
}

func (bp *BaseProvider) NewStreamingChatCompletion(ctx context.Context, in *chat_completion2.Request) (chan *chat_completion2.ResponseChunk, error) {
	panic("implement me")
}

func (bp *BaseProvider) NewSpeech(ctx context.Context, in *speech2.Request) (*speech2.Response, error) {
	return nil, nil
}

func (bp *BaseProvider) NewStreamingSpeech(ctx context.Context, in *speech2.Request) (chan *speech2.ResponseChunk, error) {
	return nil, nil
}
