package textsplitters

// ChunkOptions configures chunk size and overlap for length-based splitters.
type ChunkOptions struct {
	// ChunkSize is the target size per chunk (characters or tokens depending on splitter).
	ChunkSize int
	// ChunkOverlap is the number of characters/tokens to overlap between consecutive chunks.
	// Overlap helps preserve context across boundaries. Must be < ChunkSize.
	ChunkOverlap int
}

// RecursiveOptions configures boundary selection for RecursiveSplitter.
// Separators are tried in order from most significant to least significant.
type RecursiveOptions struct {
	// Separators defines recursive split boundaries from strongest to weakest.
	// If empty, markdown defaults are used.
	Separators []string
	// PreserveCodeBlocks keeps fenced code blocks (```...```) intact when possible.
	PreserveCodeBlocks bool
}

// DefaultChunkOptions returns sensible defaults: 1000 chars/tokens, 200 overlap.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		ChunkSize:    1000,
		ChunkOverlap: 200,
	}
}

// DefaultMarkdownRecursiveOptions returns markdown-optimized recursive split options.
func DefaultMarkdownRecursiveOptions() RecursiveOptions {
	return RecursiveOptions{
		Separators: []string{
			"\n# ",
			"\n## ",
			"\n### ",
			"\n#### ",
			"\n##### ",
			"\n###### ",
			"\n---\n",
			"\n***\n",
			"\n___\n",
			"\n\n",
			"\n",
			" ",
		},
		PreserveCodeBlocks: true,
	}
}
