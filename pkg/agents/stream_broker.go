package agents

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/streambroker"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// The built-in brokers implement the optional run-claim capability used
// for deterministic per-thread stream ids.
var (
	_ RunClaimBroker = (*streambroker.MemoryStreamBroker)(nil)
	_ RunClaimBroker = (*streambroker.RedisStreamBroker)(nil)
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

	// EnqueueMessage pushes an input message onto the channel's queue.
	// The agent loop drains this queue at iteration boundaries — same
	// cadence as IsStopped — and folds queued messages into the current
	// run. Generic so future callers can deliver user messages, tool
	// outputs, etc., without a new transport.
	EnqueueMessage(ctx context.Context, channel string, msg history.Message) error

	// DrainMessages atomically returns and clears all queued messages
	// for the channel. Empty slice if nothing queued.
	DrainMessages(ctx context.Context, channel string) ([]history.Message, error)

	// IsActive reports whether the channel has an in-flight run — used
	// by the gateway to decide between enqueueing onto an existing
	// stream and starting a fresh one. A channel is active once
	// Subscribe has been called and stays active until Close.
	IsActive(ctx context.Context, channel string) (bool, error)
}

// RunClaimBroker is an optional StreamBroker capability that enables
// deterministic, per-thread stream IDs. With a deterministic streamID the
// same broker channel is reused across a thread's turns, so EnqueueOrStart
// must atomically decide, in one shot, whether a turn joins an in-flight
// run or starts a fresh one — and reset the channel when it starts, so a
// reused channel never replays a previous turn's transcript.
type RunClaimBroker interface {
	// EnqueueOrStart atomically routes a turn for streamID:
	//   - if a run is already live on the channel, it appends msgs to the
	//     run's queue and returns started=false;
	//   - otherwise it claims the channel, resets any stale transcript /
	//     queue / stop state, and returns started=true — the caller then
	//     Subscribes and runs with msgs as the run's input.
	// The claim is released by Close.
	EnqueueOrStart(ctx context.Context, streamID string, msgs []history.Message) (started bool, err error)
}
