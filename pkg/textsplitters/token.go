package textsplitters

import (
	"fmt"
	"unicode/utf8"
)

// TokenCounter returns the number of tokens in text. Implementations can use
// model-specific tokenizers (e.g. tiktoken) or approximations.
type TokenCounter interface {
	CountTokens(text string) (int, error)
}

// TokenLengthSplitter chunks text by token count using a TokenCounter, with optional overlap.
type TokenLengthSplitter struct {
	opts  ChunkOptions
	count TokenCounter
}

// NewTokenLengthSplitter creates a splitter that chunks by token length.
// Uses ChunkOptions for chunk size and overlap (in tokens).
func NewTokenLengthSplitter(opts ChunkOptions, counter TokenCounter) (*TokenLengthSplitter, error) {
	if opts.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive, got %d", opts.ChunkSize)
	}
	if opts.ChunkOverlap < 0 || opts.ChunkOverlap >= opts.ChunkSize {
		return nil, fmt.Errorf("chunk overlap must be in [0, chunkSize), got %d", opts.ChunkOverlap)
	}
	if counter == nil {
		return nil, fmt.Errorf("token counter is required")
	}
	return &TokenLengthSplitter{opts: opts, count: counter}, nil
}

// Split splits text into chunks of at most opts.ChunkSize tokens, with opts.ChunkOverlap
// tokens overlapping between consecutive chunks.
func (s *TokenLengthSplitter) Split(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	runes := []rune(text)
	size := s.opts.ChunkSize
	overlap := s.opts.ChunkOverlap

	// Build rune ranges that stay under token limits by counting tokens incrementally.
	var chunks []string
	start := 0
	for start < len(runes) {
		chunkRunes, nextStart, err := s.nextChunk(runes, start, size, overlap)
		if err != nil {
			return nil, err
		}
		if len(chunkRunes) == 0 {
			break
		}
		chunks = append(chunks, string(chunkRunes))
		start = nextStart
		if start >= len(runes) {
			break
		}
	}

	return chunks, nil
}

// nextChunk returns the runes for the next chunk starting at start, and the start index for the following chunk.
func (s *TokenLengthSplitter) nextChunk(runes []rune, start, maxTokens, overlap int) ([]rune, int, error) {
	lastUnder := start
	for end := start; end < len(runes); end++ {
		segment := string(runes[start : end+1])
		n, err := s.count.CountTokens(segment)
		if err != nil {
			return nil, 0, err
		}
		if n <= maxTokens {
			lastUnder = end + 1
		} else {
			break
		}
	}
	// Ensure we take at least one rune if any remain (e.g. single rune over limit)
	if lastUnder == start && start < len(runes) {
		lastUnder = start + 1
	}

	chunk := runes[start:lastUnder]
	if len(chunk) == 0 {
		return nil, lastUnder, nil
	}

	// Next start: move back by overlap tokens so next chunk overlaps
	nextStart := lastUnder
	if overlap > 0 && lastUnder < len(runes) {
		nextStart = s.findOverlapStart(runes, start, lastUnder, overlap)
	}

	return chunk, nextStart, nil
}

// findOverlapStart finds the rune index from which the next chunk should start
// so that we have approximately 'overlap' tokens of overlap with the current chunk.
func (s *TokenLengthSplitter) findOverlapStart(runes []rune, chunkStart, chunkEnd, overlap int) int {
	// Binary-search or scan from chunkEnd backward until token count of [i:chunkEnd] >= overlap
	for i := chunkEnd - 1; i >= chunkStart; i-- {
		seg := string(runes[i:chunkEnd])
		n, err := s.count.CountTokens(seg)
		if err != nil || n >= overlap {
			return i
		}
	}
	return chunkStart
}

// EstimatorCounter estimates token count from character count (e.g. ~4 chars per token for English).
// Useful when a real tokenizer is not available.
type EstimatorCounter struct {
	CharsPerToken int
}

// DefaultEstimatorCounter uses 4 characters per token (typical for English and many tokenizers).
var DefaultEstimatorCounter = &EstimatorCounter{CharsPerToken: 4}

// CountTokens returns ceil(rune_count / CharsPerToken).
func (e *EstimatorCounter) CountTokens(text string) (int, error) {
	if e.CharsPerToken <= 0 {
		return 0, fmt.Errorf("chars per token must be positive, got %d", e.CharsPerToken)
	}
	n := utf8.RuneCountInString(text)
	tokens := (n + e.CharsPerToken - 1) / e.CharsPerToken
	if tokens < 1 && n > 0 {
		tokens = 1
	}
	return tokens, nil
}
