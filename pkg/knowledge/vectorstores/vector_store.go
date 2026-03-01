package vectorstores

import "context"

// VectorStore defines the interface for vector database operations.
// Implementations can use Qdrant, Pinecone, Weaviate, pgvector, etc.
type VectorStore interface {
	// EnsureCollection creates the collection/index if it doesn't exist.
	// dimension is the vector dimension (e.g., 768 for nomic-embed-text, 1536 for OpenAI ada-002).
	EnsureCollection(ctx context.Context, collectionName string, dimension int) error

	// UpsertVectors inserts or updates vectors in the collection.
	// ids, embeddings, and metadata must have the same length.
	UpsertVectors(ctx context.Context, collectionName string, ids []string, embeddings [][]float64, metadata []map[string]any) error

	// Search performs a vector similarity search and returns the top results.
	Search(ctx context.Context, collectionName string, queryVector []float64, limit int, filter map[string]any) ([]SearchResult, error)

	// DeleteCollection removes the entire collection.
	DeleteCollection(ctx context.Context, collectionName string) error

	// DeleteVectors removes specific vectors by ID from the collection.
	DeleteVectors(ctx context.Context, collectionName string, ids []string) error

	// Close closes the connection to the vector store.
	Close() error
}

// SearchResult represents a single search result from the vector store.
type SearchResult struct {
	ID       string         `json:"id"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata"`
}
