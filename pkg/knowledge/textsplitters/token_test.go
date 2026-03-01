package textsplitters

import (
	"testing"
)

func TestTokenLengthSplitter_Split(t *testing.T) {
	opts := ChunkOptions{ChunkSize: 5, ChunkOverlap: 1}
	splitter, err := NewTokenLengthSplitter(opts, DefaultEstimatorCounter)
	if err != nil {
		t.Fatal(err)
	}
	// 4 chars per token: 20 chars = 5 tokens. So "12345678901234567890" -> 5 tokens per chunk
	text := "12345678901234567890"
	chunks, err := splitter.Split(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 1 {
		t.Fatalf("expected at least 1 chunk, got %d", len(chunks))
	}
	// First chunk should be ~20 chars (5 tokens * 4). With overlap 1 token (4 chars), next starts at 16.
	// So chunk1 = 0:20, chunk2 starts at 16 -> 16:36 but we only have 20 chars, so chunk2 = 16:20
	if chunks[0] != "12345678901234567890" {
		t.Errorf("chunk 0: got %q (len %d)", chunks[0], len(chunks[0]))
	}
}

func TestTokenLengthSplitter_Empty(t *testing.T) {
	splitter, _ := NewTokenLengthSplitter(ChunkOptions{ChunkSize: 5, ChunkOverlap: 0}, DefaultEstimatorCounter)
	chunks, err := splitter.Split("")
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}
}

func TestEstimatorCounter_CountTokens(t *testing.T) {
	c := DefaultEstimatorCounter
	n, err := c.CountTokens("abcd")
	if err != nil || n != 1 {
		t.Errorf("CountTokens(abcd): n=%d err=%v", n, err)
	}
	n, _ = c.CountTokens("12345678")
	if n != 2 {
		t.Errorf("expected 2 tokens for 8 chars, got %d", n)
	}
}

func TestNewTokenLengthSplitter_Validation(t *testing.T) {
	_, err := NewTokenLengthSplitter(ChunkOptions{ChunkSize: 0}, DefaultEstimatorCounter)
	if err == nil {
		t.Error("expected error for chunk size 0")
	}
	_, err = NewTokenLengthSplitter(ChunkOptions{ChunkSize: 5, ChunkOverlap: 5}, DefaultEstimatorCounter)
	if err == nil {
		t.Error("expected error for overlap >= chunk size")
	}
	_, err = NewTokenLengthSplitter(ChunkOptions{ChunkSize: 5}, nil)
	if err == nil {
		t.Error("expected error for nil counter")
	}
}
