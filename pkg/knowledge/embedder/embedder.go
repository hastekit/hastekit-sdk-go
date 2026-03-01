package embedder

import "context"

// Embedder computes embeddings for text. Implementations can use
// OpenAI, local models, or other embedding services.
type Embedder interface {
	// Embed returns the embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float64, error)
	EmbedBatch(ctx context.Context, text []string) ([][]float64, error)
}
