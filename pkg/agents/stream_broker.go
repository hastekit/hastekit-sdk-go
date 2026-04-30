package agents

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// StreamBroker provides an abstraction for streaming response chunks
// between activities/workers and clients. This enables streaming support
// for both Restate and Temporal runtimes.
type StreamBroker interface {
	// Publish sends a response chunk to subscribers of the given channel.
	// The channel is typically the run ID or a unique identifier for the execution.
	Publish(ctx context.Context, channel string, chunk *responses.ResponseChunk) error

	// Subscribe returns a channel that receives response chunks for the given channel.
	// The returned channel will be closed when Close is called or the context is cancelled.
	Subscribe(ctx context.Context, channel string) (<-chan *responses.ResponseChunk, error)

	// Close signals that no more chunks will be published to the channel.
	// This should close all subscriber channels for the given channel.
	Close(ctx context.Context, channel string) error

	// Stop records a stop request for the given channel. The agent loop
	// reads this via IsStopped at iteration boundaries and transitions
	// to completed when set. Idempotent.
	Stop(ctx context.Context, channel string) error

	// IsStopped reports whether Stop has been called for the channel.
	IsStopped(ctx context.Context, channel string) (bool, error)
}
