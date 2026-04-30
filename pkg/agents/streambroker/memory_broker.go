package streambroker

import (
	"context"
	"sync"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// MemoryStreamBroker is an in-memory implementation of StreamBroker.
// It's suitable for testing and local development, or when all components
// run in the same process.
//
// Note: This broker does not persist across restarts. For production
// deployments with separate processes, use RedisStreamBroker.
type MemoryStreamBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan *responses.ResponseChunk
	closed      map[string]bool
	stopped     map[string]bool
}

// NewMemoryStreamBroker creates a new in-memory stream broker.
func NewMemoryStreamBroker() *MemoryStreamBroker {
	return &MemoryStreamBroker{
		subscribers: make(map[string][]chan *responses.ResponseChunk),
		closed:      make(map[string]bool),
		stopped:     make(map[string]bool),
	}
}

// Publish sends a response chunk to all subscribers of the given channel.
func (b *MemoryStreamBroker) Publish(ctx context.Context, channel string, chunk *responses.ResponseChunk) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Don't publish to closed channels
	if b.closed[channel] {
		return nil
	}

	subscribers := b.subscribers[channel]
	for _, sub := range subscribers {
		select {
		case sub <- chunk:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// Subscribe returns a channel that receives response chunks for the given channel.
// The buffer size is 100 chunks to handle bursts.
func (b *MemoryStreamBroker) Subscribe(ctx context.Context, channel string) (<-chan *responses.ResponseChunk, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// If channel is already closed, return a closed channel
	if b.closed[channel] {
		ch := make(chan *responses.ResponseChunk)
		close(ch)
		return ch, nil
	}

	// Create a buffered channel for the subscriber
	// Buffer size of 100 allows publishing to proceed without blocking immediately
	ch := make(chan *responses.ResponseChunk, 100)
	b.subscribers[channel] = append(b.subscribers[channel], ch)

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		b.unsubscribe(channel, ch)
	}()

	return ch, nil
}

// unsubscribe removes a subscriber from a channel.
func (b *MemoryStreamBroker) unsubscribe(channel string, ch chan *responses.ResponseChunk) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscribers := b.subscribers[channel]
	for i, sub := range subscribers {
		if sub == ch {
			// Remove subscriber
			b.subscribers[channel] = append(subscribers[:i], subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// Close signals that no more chunks will be published to the channel.
// This closes all subscriber channels for the given channel.
func (b *MemoryStreamBroker) Close(ctx context.Context, channel string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Mark channel as closed
	b.closed[channel] = true

	// Close all subscriber channels
	for _, ch := range b.subscribers[channel] {
		close(ch)
	}

	// Clear subscribers
	delete(b.subscribers, channel)

	return nil
}

// Stop records a stop request for the given channel.
func (b *MemoryStreamBroker) Stop(ctx context.Context, channel string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopped[channel] = true
	return nil
}

// IsStopped reports whether Stop has been called for the channel.
func (b *MemoryStreamBroker) IsStopped(ctx context.Context, channel string) (bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.stopped[channel], nil
}

// Reset clears all subscribers and closed state.
// Useful for testing.
func (b *MemoryStreamBroker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close all existing subscriber channels
	for _, subs := range b.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}

	b.subscribers = make(map[string][]chan *responses.ResponseChunk)
	b.closed = make(map[string]bool)
	b.stopped = make(map[string]bool)
}
