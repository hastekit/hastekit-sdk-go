package streambroker

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/redis/go-redis/v9"
)

// RedisStreamBroker implements StreamBroker using Redis Pub/Sub.
// This is the recommended broker for production deployments where
// activities run in separate processes from the client.
//
// Redis Pub/Sub provides:
// - Cross-process communication
// - Low latency message delivery
// - Automatic cleanup when subscribers disconnect
//
// Note: Redis Pub/Sub is fire-and-forget. If no subscribers are connected
// when a message is published, the message is lost. For guaranteed delivery,
// consider using Redis Streams instead.
type RedisStreamBroker struct {
	client *redis.Client
	prefix string
}

// RedisStreamBrokerOptions configures the Redis stream broker.
type RedisStreamBrokerOptions struct {
	// Addr is the Redis server address (e.g., "localhost:6379").
	Addr string

	// Password is the Redis password (optional).
	Password string

	// DB is the Redis database number (default 0).
	DB int

	// Prefix is prepended to all channel names (default "uno:stream:").
	// This allows multiple applications to share the same Redis instance.
	Prefix string

	// Client is an existing Redis client to use instead of creating a new one.
	// If provided, Addr/Password/DB are ignored.
	Client *redis.Client
}

// NewRedisStreamBroker creates a new Redis-backed stream broker.
func NewRedisStreamBroker(opts RedisStreamBrokerOptions) (*RedisStreamBroker, error) {
	var client *redis.Client

	if opts.Client != nil {
		client = opts.Client
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:     opts.Addr,
			Password: opts.Password,
			DB:       opts.DB,
		})
	}

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	prefix := opts.Prefix
	if prefix == "" {
		prefix = "uno:stream:"
	}

	return &RedisStreamBroker{
		client: client,
		prefix: prefix,
	}, nil
}

// channelKey returns the Redis channel key for the given channel.
func (b *RedisStreamBroker) channelKey(channel string) string {
	return b.prefix + channel
}

// closeKey returns the Redis key for tracking closed channels.
func (b *RedisStreamBroker) closeKey(channel string) string {
	return b.prefix + channel + ":closed"
}

// Publish sends a response chunk to all subscribers of the given channel.
func (b *RedisStreamBroker) Publish(ctx context.Context, channel string, chunk *responses.ResponseChunk) error {
	// Serialize the chunk
	data, err := sonic.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to serialize chunk: %w", err)
	}

	// Publish to Redis
	if err := b.client.Publish(ctx, b.channelKey(channel), data).Err(); err != nil {
		return fmt.Errorf("failed to publish chunk: %w", err)
	}

	return nil
}

// Subscribe returns a channel that receives response chunks for the given channel.
func (b *RedisStreamBroker) Subscribe(ctx context.Context, channel string) (<-chan *responses.ResponseChunk, error) {
	// Check if channel is already closed
	exists, err := b.client.Exists(ctx, b.closeKey(channel)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to check channel status: %w", err)
	}
	if exists > 0 {
		// Channel is closed, return a closed channel
		ch := make(chan *responses.ResponseChunk)
		close(ch)
		return ch, nil
	}

	// Subscribe to Redis pub/sub
	pubsub := b.client.Subscribe(ctx, b.channelKey(channel))

	// Create output channel
	ch := make(chan *responses.ResponseChunk, 100)

	// Start goroutine to receive messages
	go func() {
		defer close(ch)
		defer pubsub.Close()

		msgCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					fmt.Println("channel closed")
					return
				}

				// Deserialize the chunk
				var chunk responses.ResponseChunk
				if err := sonic.Unmarshal([]byte(msg.Payload), &chunk); err != nil {
					// Log error but continue
					continue
				}

				// Send to output channel
				select {
				case ch <- &chunk:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// Close signals that no more chunks will be published to the channel.
// This sets a key in Redis to indicate the channel is closed, and publishes
// a close signal to any active subscribers.
func (b *RedisStreamBroker) Close(ctx context.Context, channel string) error {
	// Set the closed key with a TTL (e.g., 1 hour)
	// This prevents new subscribers from joining a closed channel
	if err := b.client.Set(ctx, b.closeKey(channel), "1", 0).Err(); err != nil {
		return fmt.Errorf("failed to mark channel as closed: %w", err)
	}

	// Publish a special "close" message
	// Subscribers should handle this by closing their channels
	closeMsg := &responses.ResponseChunk{
		// Use a sentinel value that subscribers can detect
		// In practice, subscribers close when the context is cancelled
	}
	data, _ := sonic.Marshal(closeMsg)
	b.client.Publish(ctx, b.channelKey(channel), data)

	return nil
}

// Cleanup removes the closed marker for a channel.
// Call this to allow re-subscribing to a previously closed channel.
func (b *RedisStreamBroker) Cleanup(ctx context.Context, channel string) error {
	return b.client.Del(ctx, b.closeKey(channel)).Err()
}

// GetClient returns the underlying Redis client.
// Useful for advanced operations or sharing the client.
func (b *RedisStreamBroker) GetClient() *redis.Client {
	return b.client
}

// Ensure RedisStreamBroker implements StreamBroker
var _ agents.StreamBroker = (*RedisStreamBroker)(nil)
