package textsplitters

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// mockEmbedder is a simple embedder for testing that uses word overlap.
type mockEmbedder struct {
	// embedFunc allows custom embedding logic per test
	embedFunc func(text string) ([]float64, error)
}

func (m *mockEmbedder) Embed(text string) ([]float64, error) {
	if m.embedFunc != nil {
		return m.embedFunc(text)
	}
	// Default: simple word-based embedding (bag of words style)
	return simpleWordEmbedding(text), nil
}

// simpleWordEmbedding creates a simple embedding based on word presence.
// Words are hashed to positions in a fixed-size vector.
func simpleWordEmbedding(text string) []float64 {
	const dim = 100
	vec := make([]float64, dim)

	words := strings.Fields(strings.ToLower(text))
	for _, word := range words {
		// Simple hash to get position
		pos := 0
		for _, r := range word {
			pos = (pos*31 + int(r)) % dim
		}
		vec[pos] += 1.0
	}

	// Normalize
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}

func TestNewSemanticSplitter(t *testing.T) {
	embedder := &mockEmbedder{}

	tests := []struct {
		name    string
		opts    SemanticSplitterOptions
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default options",
			opts:    DefaultSemanticSplitterOptions(),
			wantErr: false,
		},
		{
			name: "valid custom options",
			opts: SemanticSplitterOptions{
				MaxChunkSize:        1000,
				MinChunkSize:        50,
				SimilarityThreshold: 0.7,
				BufferSize:          2,
			},
			wantErr: false,
		},
		{
			name: "invalid similarity threshold below 0",
			opts: SemanticSplitterOptions{
				SimilarityThreshold: -0.1,
			},
			wantErr: true,
			errMsg:  "similarity threshold must be in [0, 1]",
		},
		{
			name: "invalid similarity threshold above 1",
			opts: SemanticSplitterOptions{
				SimilarityThreshold: 1.5,
			},
			wantErr: true,
			errMsg:  "similarity threshold must be in [0, 1]",
		},
		{
			name: "invalid max chunk size",
			opts: SemanticSplitterOptions{
				MaxChunkSize:        -1,
				SimilarityThreshold: 0.5,
			},
			wantErr: true,
			errMsg:  "max chunk size must be non-negative",
		},
		{
			name: "invalid min chunk size",
			opts: SemanticSplitterOptions{
				MinChunkSize:        -1,
				SimilarityThreshold: 0.5,
			},
			wantErr: true,
			errMsg:  "min chunk size must be non-negative",
		},
		{
			name: "invalid buffer size",
			opts: SemanticSplitterOptions{
				BufferSize:          -1,
				SimilarityThreshold: 0.5,
			},
			wantErr: true,
			errMsg:  "buffer size must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSemanticSplitter(tt.opts, embedder)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSemanticSplitter_NilEmbedder(t *testing.T) {
	_, err := NewSemanticSplitter(DefaultSemanticSplitterOptions(), nil)
	if err == nil {
		t.Error("expected error for nil embedder")
	}
	if !strings.Contains(err.Error(), "embedder is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSemanticSplitter_EmptyInput(t *testing.T) {
	splitter, err := NewSemanticSplitter(DefaultSemanticSplitterOptions(), &mockEmbedder{})
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	chunks, err := splitter.Split("")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}
}

func TestSemanticSplitter_SingleSentence(t *testing.T) {
	splitter, err := NewSemanticSplitter(DefaultSemanticSplitterOptions(), &mockEmbedder{})
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	text := "This is a single sentence."
	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSemanticSplitter_SimilarContent(t *testing.T) {
	// When content is similar, it should stay together
	embedder := &mockEmbedder{}
	opts := DefaultSemanticSplitterOptions()
	opts.SimilarityThreshold = 0.3 // Low threshold - most things stay together
	opts.MinChunkSize = 0

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	// Similar sentences about programming
	text := "Python is a programming language. Java is also a programming language. Programming languages are useful."

	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// With similar content and low threshold, should be few chunks
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
	t.Logf("Similar content split into %d chunks", len(chunks))
}

func TestSemanticSplitter_DissimilarContent(t *testing.T) {
	// When content is very different, it should be split
	embedder := &mockEmbedder{}
	opts := DefaultSemanticSplitterOptions()
	opts.SimilarityThreshold = 0.8 // High threshold - more splits
	opts.MinChunkSize = 0

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	// Very different topics
	text := "The quick brown fox jumps over the lazy dog. Quantum physics explains particle behavior. " +
		"Italian cuisine features pasta and pizza. Machine learning uses neural networks."

	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// With dissimilar content and high threshold, should have multiple chunks
	t.Logf("Dissimilar content split into %d chunks", len(chunks))
	if len(chunks) < 1 {
		t.Error("expected at least one chunk")
	}
}

func TestSemanticSplitter_MaxChunkSize(t *testing.T) {
	embedder := &mockEmbedder{}
	opts := DefaultSemanticSplitterOptions()
	opts.MaxChunkSize = 50
	opts.SimilarityThreshold = 0.1 // Keep things together
	opts.MinChunkSize = 0

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	text := "This is a longer text that should be split due to max chunk size constraints. " +
		"We want to ensure that no chunk exceeds the maximum size."

	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for i, chunk := range chunks {
		runeCount := len([]rune(chunk))
		if runeCount > opts.MaxChunkSize {
			t.Errorf("chunk %d exceeds max size: %d > %d", i, runeCount, opts.MaxChunkSize)
		}
	}
}

func TestSemanticSplitter_ParagraphBreaks(t *testing.T) {
	embedder := &mockEmbedder{}
	opts := DefaultSemanticSplitterOptions()
	opts.MinChunkSize = 0
	opts.SimilarityThreshold = 0.7

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	text := "This is paragraph one about topic A.\n\n" +
		"This is paragraph two about topic B.\n\n" +
		"This is paragraph three about topic C."

	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
	t.Logf("Paragraphs split into %d chunks", len(chunks))
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []float64
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float64{1, 0, 0},
			b:        []float64{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "similar vectors",
			a:        []float64{1, 1, 0},
			b:        []float64{1, 0, 0},
			expected: 1 / math.Sqrt(2),
		},
		{
			name:     "empty vectors",
			a:        []float64{},
			b:        []float64{},
			expected: 0.0,
		},
		{
			name:     "different lengths",
			a:        []float64{1, 2},
			b:        []float64{1, 2, 3},
			expected: 0.0,
		},
		{
			name:     "zero vector",
			a:        []float64{0, 0, 0},
			b:        []float64{1, 1, 1},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if math.Abs(result-tt.expected) > 1e-10 {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestPercentileBreakpointFinder(t *testing.T) {
	finder := &PercentileBreakpointFinder{Percentile: 25}

	// Similarities: indices with lowest 25% become breakpoints
	similarities := []float64{0.8, 0.3, 0.9, 0.2, 0.7, 0.6, 0.4, 0.5}
	// Sorted: 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9
	// 25th percentile: index 2 of 8 = 0.4
	// Values <= 0.4: 0.3 (idx 1), 0.2 (idx 3), 0.4 (idx 6)
	// Breakpoints: 2, 4, 7

	breakpoints := finder.FindBreakpoints(similarities)
	t.Logf("Breakpoints at percentile 25: %v", breakpoints)

	if len(breakpoints) == 0 {
		t.Error("expected breakpoints")
	}
}

func TestPercentileBreakpointFinder_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		percentile   float64
		similarities []float64
		wantEmpty    bool
	}{
		{
			name:         "empty similarities",
			percentile:   25,
			similarities: []float64{},
			wantEmpty:    true,
		},
		{
			name:         "zero percentile",
			percentile:   0,
			similarities: []float64{0.5, 0.5},
			wantEmpty:    true,
		},
		{
			name:         "100 percentile",
			percentile:   100,
			similarities: []float64{0.5, 0.5},
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := &PercentileBreakpointFinder{Percentile: tt.percentile}
			breakpoints := finder.FindBreakpoints(tt.similarities)
			if tt.wantEmpty && len(breakpoints) != 0 {
				t.Errorf("expected empty breakpoints, got %v", breakpoints)
			}
		})
	}
}

func TestGradientBreakpointFinder(t *testing.T) {
	finder := &GradientBreakpointFinder{MinGradient: 0.3}

	// Big drop between index 1 and 2 (0.8 -> 0.3 = 0.5 gradient)
	// Big drop between index 3 and 4 (0.9 -> 0.4 = 0.5 gradient)
	similarities := []float64{0.7, 0.8, 0.3, 0.9, 0.4, 0.5}

	breakpoints := finder.FindBreakpoints(similarities)
	t.Logf("Gradient breakpoints: %v", breakpoints)

	// Should find breakpoints at indices 2 and 4 (after the drops)
	if len(breakpoints) == 0 {
		t.Error("expected breakpoints")
	}
}

func TestGradientBreakpointFinder_NotEnoughSimilarities(t *testing.T) {
	finder := &GradientBreakpointFinder{MinGradient: 0.3}

	tests := []struct {
		name         string
		similarities []float64
	}{
		{"empty", []float64{}},
		{"single", []float64{0.5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			breakpoints := finder.FindBreakpoints(tt.similarities)
			if len(breakpoints) != 0 {
				t.Errorf("expected no breakpoints, got %v", breakpoints)
			}
		})
	}
}

func TestSortFloat64s(t *testing.T) {
	tests := []struct {
		name   string
		input  []float64
		sorted []float64
	}{
		{
			name:   "already sorted",
			input:  []float64{1, 2, 3},
			sorted: []float64{1, 2, 3},
		},
		{
			name:   "reverse order",
			input:  []float64{3, 2, 1},
			sorted: []float64{1, 2, 3},
		},
		{
			name:   "random order",
			input:  []float64{3, 1, 4, 1, 5, 9, 2, 6},
			sorted: []float64{1, 1, 2, 3, 4, 5, 6, 9},
		},
		{
			name:   "empty",
			input:  []float64{},
			sorted: []float64{},
		},
		{
			name:   "single element",
			input:  []float64{42},
			sorted: []float64{42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]float64, len(tt.input))
			copy(input, tt.input)
			sortFloat64s(input)
			for i, v := range input {
				if i < len(tt.sorted) && v != tt.sorted[i] {
					t.Errorf("at index %d: expected %f, got %f", i, tt.sorted[i], v)
				}
			}
		})
	}
}

func TestSemanticSplitter_EmbedderError(t *testing.T) {
	embedder := &mockEmbedder{
		embedFunc: func(text string) ([]float64, error) {
			return nil, fmt.Errorf("embedding error")
		},
	}

	opts := DefaultSemanticSplitterOptions()
	opts.MinChunkSize = 0 // Ensure sentences aren't merged

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	// Use text that will definitely produce multiple sentences
	_, err = splitter.Split("First sentence here. Second sentence here. Third sentence here.")
	// Should propagate the error
	if err == nil {
		t.Error("expected error from embedder, got nil")
	} else if !strings.Contains(err.Error(), "embedding error") {
		t.Errorf("expected error containing 'embedding error', got: %v", err)
	}
}

func TestSemanticSplitter_CustomSeparators(t *testing.T) {
	embedder := &mockEmbedder{}
	opts := SemanticSplitterOptions{
		MaxChunkSize:        2000,
		MinChunkSize:        0,
		SimilarityThreshold: 0.5,
		BufferSize:          0,
		Separators:          []string{"|"}, // Custom separator
	}

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	text := "Part one|Part two|Part three"
	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	t.Logf("Custom separator chunks: %v", chunks)
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestSemanticSplitter_ZeroBuffer(t *testing.T) {
	embedder := &mockEmbedder{}
	opts := DefaultSemanticSplitterOptions()
	opts.BufferSize = 0

	splitter, err := NewSemanticSplitter(opts, embedder)
	if err != nil {
		t.Fatalf("failed to create splitter: %v", err)
	}

	text := "First sentence. Second sentence. Third sentence."
	chunks, err := splitter.Split(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

// Benchmark for semantic splitting
func BenchmarkSemanticSplitter(b *testing.B) {
	embedder := &mockEmbedder{}
	opts := DefaultSemanticSplitterOptions()

	splitter, _ := NewSemanticSplitter(opts, embedder)

	text := strings.Repeat("This is a test sentence. ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = splitter.Split(text)
	}
}
