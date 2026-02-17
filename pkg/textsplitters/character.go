package textsplitters

import (
	"fmt"
	"unicode/utf8"
)

// CharacterLengthSplitter chunks text by character (rune) length with optional overlap.
type CharacterLengthSplitter struct {
	opts ChunkOptions
}

// NewCharacterLengthSplitter creates a splitter that chunks by character length.
// Uses ChunkOptions for chunk size and overlap (in characters).
func NewCharacterLengthSplitter(opts ChunkOptions) (*CharacterLengthSplitter, error) {
	if opts.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive, got %d", opts.ChunkSize)
	}
	if opts.ChunkOverlap < 0 || opts.ChunkOverlap >= opts.ChunkSize {
		return nil, fmt.Errorf("chunk overlap must be in [0, chunkSize), got %d", opts.ChunkOverlap)
	}
	return &CharacterLengthSplitter{opts: opts}, nil
}

// Split splits text into chunks of at most opts.ChunkSize runes, with opts.ChunkOverlap
// runes shared between consecutive chunks.
func (s *CharacterLengthSplitter) Split(text string) ([]string, error) {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil, nil
	}

	size := s.opts.ChunkSize
	overlap := s.opts.ChunkOverlap
	step := size - overlap

	var chunks []string
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[start:end])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}
	}

	return chunks, nil
}

// RuneCount returns the number of runes in s (same as character length for Unicode).
func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}
