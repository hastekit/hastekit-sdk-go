package textsplitters

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Embedder computes embeddings for text. Implementations can use
// OpenAI, local models, or other embedding services.
type Embedder interface {
	// Embed returns the embedding vector for the given text.
	Embed(text string) ([]float64, error)
}

// SemanticSplitterOptions configures the semantic chunking behavior.
type SemanticSplitterOptions struct {
	// MaxChunkSize is the maximum size of a chunk in characters.
	// If 0, no maximum is enforced (chunks are purely similarity-based).
	MaxChunkSize int

	// MinChunkSize is the minimum size of a chunk in characters.
	// Sentences below this will be merged with neighbors. Default: 0.
	MinChunkSize int

	// SimilarityThreshold is the cosine similarity threshold (0-1) for grouping sentences.
	// Higher values create more, smaller chunks. Lower values create fewer, larger chunks.
	// Default: 0.5
	SimilarityThreshold float64

	// BufferSize is the number of sentences to include on each side when computing embeddings.
	// This provides context for more accurate similarity calculation. Default: 1.
	BufferSize int

	// Separators defines the sentence boundaries to split on, in order of priority.
	// Default: paragraph breaks, sentence endings, semicolons, commas.
	Separators []string
}

// DefaultSemanticSplitterOptions returns sensible defaults for semantic splitting.
func DefaultSemanticSplitterOptions() SemanticSplitterOptions {
	return SemanticSplitterOptions{
		MaxChunkSize:        2000,
		MinChunkSize:        100,
		SimilarityThreshold: 0.5,
		BufferSize:          1,
		Separators:          []string{"\n\n", ".\n", ". ", ".\t", "!\n", "! ", "?\n", "? ", ";\n", "; ", ",\n", ", "},
	}
}

// SemanticSplitter chunks text based on semantic similarity between sentences.
// It first splits text into sentences, computes embeddings, and then groups
// consecutive sentences with similar embeddings into chunks.
type SemanticSplitter struct {
	opts     SemanticSplitterOptions
	embedder Embedder
}

// NewSemanticSplitter creates a splitter that chunks by semantic similarity.
func NewSemanticSplitter(opts SemanticSplitterOptions, embedder Embedder) (*SemanticSplitter, error) {
	if embedder == nil {
		return nil, fmt.Errorf("embedder is required for semantic splitting")
	}
	if opts.SimilarityThreshold < 0 || opts.SimilarityThreshold > 1 {
		return nil, fmt.Errorf("similarity threshold must be in [0, 1], got %f", opts.SimilarityThreshold)
	}
	if opts.MaxChunkSize < 0 {
		return nil, fmt.Errorf("max chunk size must be non-negative, got %d", opts.MaxChunkSize)
	}
	if opts.MinChunkSize < 0 {
		return nil, fmt.Errorf("min chunk size must be non-negative, got %d", opts.MinChunkSize)
	}
	if opts.BufferSize < 0 {
		return nil, fmt.Errorf("buffer size must be non-negative, got %d", opts.BufferSize)
	}
	if len(opts.Separators) == 0 {
		opts.Separators = DefaultSemanticSplitterOptions().Separators
	}

	return &SemanticSplitter{opts: opts, embedder: embedder}, nil
}

// Split splits text into semantically coherent chunks.
func (s *SemanticSplitter) Split(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	// Step 1: Split into sentences
	sentences := s.splitIntoSentences(text)
	if len(sentences) == 0 {
		return nil, nil
	}

	// If only one sentence, return it as a single chunk
	if len(sentences) == 1 {
		return sentences, nil
	}

	// Step 2: Compute embeddings for each sentence (with buffer context)
	embeddings, err := s.computeEmbeddings(sentences)
	if err != nil {
		return nil, fmt.Errorf("failed to compute embeddings: %w", err)
	}

	// Step 3: Compute similarity between consecutive sentences
	similarities := s.computeSimilarities(embeddings)

	// Step 4: Find breakpoints where similarity drops below threshold
	breakpoints := s.findBreakpoints(similarities)

	// Step 5: Group sentences into chunks based on breakpoints
	chunks := s.groupIntoChunks(sentences, breakpoints)

	// Step 6: Enforce size constraints if specified
	if s.opts.MaxChunkSize > 0 {
		chunks = s.enforceMaxSize(chunks)
	}

	return chunks, nil
}

// splitIntoSentences splits text into sentences using the configured separators.
func (s *SemanticSplitter) splitIntoSentences(text string) []string {
	// Build a regex pattern from separators
	var escapedSeps []string
	for _, sep := range s.opts.Separators {
		escapedSeps = append(escapedSeps, regexp.QuoteMeta(sep))
	}
	pattern := "(" + strings.Join(escapedSeps, "|") + ")"
	re := regexp.MustCompile(pattern)

	// Split while keeping separators attached to the preceding text
	parts := re.Split(text, -1)
	seps := re.FindAllString(text, -1)

	var sentences []string
	for i, part := range parts {
		sentence := part
		// Attach the separator to the sentence
		if i < len(seps) {
			sentence += seps[i]
		}
		sentence = strings.TrimSpace(sentence)
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	// Merge very short sentences with the next one
	if s.opts.MinChunkSize > 0 {
		sentences = s.mergeShortSentences(sentences)
	}

	return sentences
}

// mergeShortSentences combines sentences shorter than MinChunkSize with neighbors.
func (s *SemanticSplitter) mergeShortSentences(sentences []string) []string {
	if len(sentences) <= 1 {
		return sentences
	}

	var merged []string
	buffer := ""

	for _, sent := range sentences {
		if buffer == "" {
			buffer = sent
		} else {
			buffer = buffer + " " + sent
		}

		if len(buffer) >= s.opts.MinChunkSize {
			merged = append(merged, buffer)
			buffer = ""
		}
	}

	// Don't lose remaining buffer
	if buffer != "" {
		if len(merged) > 0 {
			// Merge with last chunk if buffer is too small
			merged[len(merged)-1] = merged[len(merged)-1] + " " + buffer
		} else {
			merged = append(merged, buffer)
		}
	}

	return merged
}

// computeEmbeddings computes embeddings for each sentence with buffer context.
func (s *SemanticSplitter) computeEmbeddings(sentences []string) ([][]float64, error) {
	embeddings := make([][]float64, len(sentences))

	for i := range sentences {
		// Build context with buffer
		contextText := s.buildContextWindow(sentences, i)

		emb, err := s.embedder.Embed(contextText)
		if err != nil {
			return nil, fmt.Errorf("failed to embed sentence %d: %w", i, err)
		}
		embeddings[i] = emb
	}

	return embeddings, nil
}

// buildContextWindow creates a context string including buffer sentences.
func (s *SemanticSplitter) buildContextWindow(sentences []string, idx int) string {
	start := idx - s.opts.BufferSize
	if start < 0 {
		start = 0
	}
	end := idx + s.opts.BufferSize + 1
	if end > len(sentences) {
		end = len(sentences)
	}

	return strings.Join(sentences[start:end], " ")
}

// computeSimilarities calculates cosine similarity between consecutive embeddings.
func (s *SemanticSplitter) computeSimilarities(embeddings [][]float64) []float64 {
	if len(embeddings) <= 1 {
		return nil
	}

	similarities := make([]float64, len(embeddings)-1)
	for i := 0; i < len(embeddings)-1; i++ {
		similarities[i] = cosineSimilarity(embeddings[i], embeddings[i+1])
	}

	return similarities
}

// findBreakpoints identifies indices where chunks should be split.
func (s *SemanticSplitter) findBreakpoints(similarities []float64) []int {
	if len(similarities) == 0 {
		return nil
	}

	var breakpoints []int

	// Method 1: Absolute threshold
	// Split where similarity drops below threshold
	for i, sim := range similarities {
		if sim < s.opts.SimilarityThreshold {
			breakpoints = append(breakpoints, i+1) // breakpoint is after sentence i
		}
	}

	return breakpoints
}

// groupIntoChunks combines sentences into chunks based on breakpoints.
func (s *SemanticSplitter) groupIntoChunks(sentences []string, breakpoints []int) []string {
	if len(sentences) == 0 {
		return nil
	}

	if len(breakpoints) == 0 {
		// No breakpoints - return all as single chunk
		return []string{strings.Join(sentences, " ")}
	}

	var chunks []string
	start := 0

	for _, bp := range breakpoints {
		if bp > start && bp <= len(sentences) {
			chunk := strings.Join(sentences[start:bp], " ")
			if strings.TrimSpace(chunk) != "" {
				chunks = append(chunks, chunk)
			}
			start = bp
		}
	}

	// Handle remaining sentences after last breakpoint
	if start < len(sentences) {
		chunk := strings.Join(sentences[start:], " ")
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// enforceMaxSize splits chunks that exceed MaxChunkSize.
func (s *SemanticSplitter) enforceMaxSize(chunks []string) []string {
	var result []string

	for _, chunk := range chunks {
		if len([]rune(chunk)) <= s.opts.MaxChunkSize {
			result = append(result, chunk)
		} else {
			// Use character splitter as fallback for oversized chunks
			fallback, err := NewCharacterLengthSplitter(ChunkOptions{
				ChunkSize:    s.opts.MaxChunkSize,
				ChunkOverlap: s.opts.MaxChunkSize / 10, // 10% overlap
			})
			if err != nil {
				// Shouldn't happen with valid MaxChunkSize, but just append as-is
				result = append(result, chunk)
				continue
			}
			subChunks, err := fallback.Split(chunk)
			if err != nil {
				result = append(result, chunk)
				continue
			}
			result = append(result, subChunks...)
		}
	}

	return result
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// PercentileBreakpointFinder finds breakpoints using statistical analysis.
// It identifies breakpoints where similarity is significantly lower than average.
type PercentileBreakpointFinder struct {
	// Percentile (0-100) below which similarities are considered breakpoints.
	// E.g., 25 means the lowest 25% of similarity scores become breakpoints.
	Percentile float64
}

// FindBreakpoints returns indices where similarity is in the lowest percentile.
func (p *PercentileBreakpointFinder) FindBreakpoints(similarities []float64) []int {
	if len(similarities) == 0 || p.Percentile <= 0 || p.Percentile >= 100 {
		return nil
	}

	// Copy and sort to find percentile threshold
	sorted := make([]float64, len(similarities))
	copy(sorted, similarities)
	sortFloat64s(sorted)

	// Find threshold at percentile
	idx := int(float64(len(sorted)) * p.Percentile / 100)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	threshold := sorted[idx]

	// Find breakpoints below threshold
	var breakpoints []int
	for i, sim := range similarities {
		if sim <= threshold {
			breakpoints = append(breakpoints, i+1)
		}
	}

	return breakpoints
}

// sortFloat64s sorts a slice of float64 in ascending order.
func sortFloat64s(s []float64) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// GradientBreakpointFinder identifies breakpoints based on similarity gradients.
// It finds points where the similarity change is most dramatic.
type GradientBreakpointFinder struct {
	// MinGradient is the minimum absolute change in similarity to consider a breakpoint.
	MinGradient float64
}

// FindBreakpoints returns indices where similarity gradient exceeds threshold.
func (g *GradientBreakpointFinder) FindBreakpoints(similarities []float64) []int {
	if len(similarities) < 2 {
		return nil
	}

	var breakpoints []int

	for i := 1; i < len(similarities); i++ {
		gradient := similarities[i-1] - similarities[i] // positive = drop in similarity
		if gradient >= g.MinGradient {
			breakpoints = append(breakpoints, i)
		}
	}

	return breakpoints
}
