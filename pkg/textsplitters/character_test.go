package textsplitters

import (
	"testing"
)

func TestCharacterLengthSplitter_Split(t *testing.T) {
	opts := ChunkOptions{ChunkSize: 10, ChunkOverlap: 2}
	splitter, err := NewCharacterLengthSplitter(opts)
	if err != nil {
		t.Fatal(err)
	}

	// "Hello world" = 11 runes
	chunks, err := splitter.Split("Hello world")
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello worl" {
		t.Errorf("chunk 0: got %q", chunks[0])
	}
	if chunks[1] != "rld" { // step = 10-2 = 8, so second chunk starts at index 8
		t.Errorf("chunk 1: got %q", chunks[1])
	}
}

func TestCharacterLengthSplitter_Empty(t *testing.T) {
	splitter, _ := NewCharacterLengthSplitter(ChunkOptions{ChunkSize: 5, ChunkOverlap: 0})
	chunks, err := splitter.Split("")
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}
}

func TestCharacterLengthSplitter_Validation(t *testing.T) {
	_, err := NewCharacterLengthSplitter(ChunkOptions{ChunkSize: 0, ChunkOverlap: 0})
	if err == nil {
		t.Error("expected error for chunk size 0")
	}
	_, err = NewCharacterLengthSplitter(ChunkOptions{ChunkSize: 10, ChunkOverlap: 10})
	if err == nil {
		t.Error("expected error for overlap >= chunk size")
	}
}
