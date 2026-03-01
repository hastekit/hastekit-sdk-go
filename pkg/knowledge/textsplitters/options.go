package textsplitters

// ChunkOptions configures chunk size and overlap for length-based splitters.
type ChunkOptions struct {
	// ChunkSize is the target size per chunk (characters or tokens depending on splitter).
	ChunkSize int
	// ChunkOverlap is the number of characters/tokens to overlap between consecutive chunks.
	// Overlap helps preserve context across boundaries. Must be < ChunkSize.
	ChunkOverlap int
}

// DefaultChunkOptions returns sensible defaults: 1000 chars/tokens, 200 overlap.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		ChunkSize:    1000,
		ChunkOverlap: 200,
	}
}
