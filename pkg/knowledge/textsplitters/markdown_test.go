package textsplitters

import (
	"context"
	"strings"
	"testing"
)

// tc is the token counter used across markdown tests.
// DefaultEstimatorCounter uses ~4 chars per token.
var tc = DefaultEstimatorCounter

func tokenLen(s string) int {
	n, _ := tc.CountTokens(s)
	return n
}

func TestNewMarkdownSplitter(t *testing.T) {
	tests := []struct {
		name    string
		opts    ChunkOptions
		counter TokenCounter
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid defaults",
			opts:    DefaultChunkOptions(),
			counter: tc,
			wantErr: false,
		},
		{
			name:    "valid custom",
			opts:    ChunkOptions{ChunkSize: 500, ChunkOverlap: 50},
			counter: tc,
			wantErr: false,
		},
		{
			name:    "zero chunk size",
			opts:    ChunkOptions{ChunkSize: 0, ChunkOverlap: 0},
			counter: tc,
			wantErr: true,
			errMsg:  "chunk size must be positive",
		},
		{
			name:    "negative chunk size",
			opts:    ChunkOptions{ChunkSize: -1, ChunkOverlap: 0},
			counter: tc,
			wantErr: true,
			errMsg:  "chunk size must be positive",
		},
		{
			name:    "overlap equals chunk size",
			opts:    ChunkOptions{ChunkSize: 10, ChunkOverlap: 10},
			counter: tc,
			wantErr: true,
			errMsg:  "chunk overlap must be in [0, chunkSize)",
		},
		{
			name:    "negative overlap",
			opts:    ChunkOptions{ChunkSize: 10, ChunkOverlap: -1},
			counter: tc,
			wantErr: true,
			errMsg:  "chunk overlap must be in [0, chunkSize)",
		},
		{
			name:    "nil counter",
			opts:    ChunkOptions{ChunkSize: 10, ChunkOverlap: 0},
			counter: nil,
			wantErr: true,
			errMsg:  "token counter is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMarkdownSplitter(tt.opts, tt.counter)
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

func TestMarkdownSplitter_EmptyInput(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 25, ChunkOverlap: 0}, tc)
	chunks, err := splitter.Split(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}
}

func TestMarkdownSplitter_FitsInOneChunk(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 250, ChunkOverlap: 0}, tc)
	text := "# Title\n\nSome short content."
	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != strings.TrimSpace(text) {
		t.Errorf("expected %q, got %q", strings.TrimSpace(text), chunks[0])
	}
}

func TestMarkdownSplitter_SplitOnH2Headers(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 15, ChunkOverlap: 0}, tc)
	text := "# Title\n\nIntro paragraph.\n\n## Section 1\n\nContent for section one.\n\n## Section 2\n\nContent for section two."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}

	// Verify that header markers are preserved.
	foundH2 := false
	for _, c := range chunks {
		if strings.Contains(c, "## Section") {
			foundH2 = true
			break
		}
	}
	if !foundH2 {
		t.Error("expected at least one chunk to contain a ## header")
	}
}

func TestMarkdownSplitter_SplitOnH1Headers(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 8, ChunkOverlap: 0}, tc)
	text := "# First\n\nContent one.\n\n# Second\n\nContent two."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}

	// First chunk should contain "# First".
	if !strings.Contains(chunks[0], "# First") {
		t.Errorf("first chunk should contain '# First', got %q", chunks[0])
	}
}

func TestMarkdownSplitter_CodeBlockPreserved(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 50, ChunkOverlap: 0}, tc)
	text := "# Intro\n\nSome text.\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\nMore text after code."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	// Find the chunk containing the code block.
	foundCodeBlock := false
	for _, c := range chunks {
		if strings.Contains(c, "```go") && strings.Contains(c, "```") {
			foundCodeBlock = true
			// Verify the code block is intact.
			if !strings.Contains(c, "func main()") {
				t.Errorf("code block content should be intact, got %q", c)
			}
			break
		}
	}
	if !foundCodeBlock {
		t.Errorf("expected a chunk to contain the complete code block, chunks: %v", chunks)
	}
}

func TestMarkdownSplitter_LargeCodeBlock(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 8, ChunkOverlap: 0}, tc)
	// Code block that exceeds chunk size.
	text := "```\nline one of code\nline two of code\nline three of code\nline four of code\n```"

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected code block to be split into multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		tl := tokenLen(c)
		if tl > 8 {
			t.Errorf("chunk %d exceeds chunk size: %d tokens", i, tl)
		}
	}
}

func TestMarkdownSplitter_HorizontalRule(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 10, ChunkOverlap: 0}, tc)
	text := "Content above rule.\n\n---\n\nContent below rule."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks split on horizontal rule, got %d: %v", len(chunks), chunks)
	}
}

func TestMarkdownSplitter_ParagraphSplitting(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 10, ChunkOverlap: 0}, tc)
	text := "First paragraph here.\n\nSecond paragraph here.\n\nThird paragraph here."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}

	// Each chunk should be within token size limit.
	for i, c := range chunks {
		tl := tokenLen(c)
		if tl > 10 {
			t.Errorf("chunk %d exceeds chunk size: %d tokens, content: %q", i, tl, c)
		}
	}
}

func TestMarkdownSplitter_Overlap(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 8, ChunkOverlap: 4}, tc)
	text := "Part A text.\n\nPart B text.\n\nPart C text."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(chunks), chunks)
	}

	// With overlap, consecutive chunks should share some content.
	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1]
		curr := chunks[i]
		// The overlap means some piece from the end of prev appears at the start of curr.
		prevParts := strings.Split(prev, "\n\n")
		lastPiece := prevParts[len(prevParts)-1]
		if !strings.Contains(curr, lastPiece) {
			t.Errorf("expected chunk %d to overlap with chunk %d; last piece %q not found in %q", i, i-1, lastPiece, curr)
		}
	}
}

func TestMarkdownSplitter_MixedContent(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 30, ChunkOverlap: 0}, tc)
	text := `# Getting Started

Welcome to the guide.

## Installation

Run the following:

` + "```bash\nnpm install my-package\n```" + `

## Usage

Import and use:

` + "```js\nimport { foo } from 'my-package';\nfoo();\n```" + `

That's it!`

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// All chunks should be within token size limit.
	for i, c := range chunks {
		tl := tokenLen(c)
		if tl > 30 {
			t.Errorf("chunk %d exceeds chunk size: %d tokens", i, tl)
		}
	}

	// Verify all significant content is present across chunks.
	joined := strings.Join(chunks, " ")
	for _, want := range []string{"Getting Started", "Installation", "npm install", "Usage", "foo()"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected combined chunks to contain %q", want)
		}
	}
}

func TestMarkdownSplitter_HeaderAtDocumentStart(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 10, ChunkOverlap: 0}, tc)
	text := "# Title\n\nSome content that is long enough to force a split on paragraphs.\n\nAnother paragraph."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	// The "# Title" should be preserved in the output.
	found := false
	for _, c := range chunks {
		if strings.HasPrefix(c, "# Title") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a chunk starting with '# Title', got: %v", chunks)
	}
}

func TestMarkdownSplitter_NestedHeaders(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 20, ChunkOverlap: 0}, tc)
	text := "# Main\n\nIntro.\n\n## Sub A\n\nContent A.\n\n### Sub Sub\n\nDeep content.\n\n## Sub B\n\nContent B."

	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for nested headers, got %d: %v", len(chunks), chunks)
	}

	joined := strings.Join(chunks, " ")
	for _, want := range []string{"# Main", "## Sub A", "### Sub Sub", "## Sub B"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected combined chunks to contain %q", want)
		}
	}
}

func TestSplitByCodeBlocks(t *testing.T) {
	text := "Before code.\n\n```go\nfunc foo() {}\n```\n\nAfter code."
	segments := splitByCodeBlocks(text)

	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %v", len(segments), segments)
	}

	if segments[0].isCode {
		t.Error("first segment should not be code")
	}
	if !segments[1].isCode {
		t.Error("second segment should be code")
	}
	if segments[2].isCode {
		t.Error("third segment should not be code")
	}

	if !strings.Contains(segments[1].text, "func foo()") {
		t.Errorf("code segment should contain function, got %q", segments[1].text)
	}
}

func TestSplitByCodeBlocks_NoCode(t *testing.T) {
	text := "Just plain markdown."
	segments := splitByCodeBlocks(text)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].isCode {
		t.Error("segment should not be code")
	}
}

func TestSplitByCodeBlocks_UnclosedFence(t *testing.T) {
	text := "Before.\n\n```python\nimport os\nprint(os.getcwd())"
	segments := splitByCodeBlocks(text)

	// Should handle gracefully — unclosed block treated as code.
	codeFound := false
	for _, seg := range segments {
		if seg.isCode && strings.Contains(seg.text, "import os") {
			codeFound = true
		}
	}
	if !codeFound {
		t.Error("expected unclosed code block to be treated as code")
	}
}

func TestSplitByCodeBlocks_MultipleBlocks(t *testing.T) {
	text := "Intro.\n\n```\nblock1\n```\n\nMiddle.\n\n```\nblock2\n```\n\nEnd."
	segments := splitByCodeBlocks(text)

	codeCount := 0
	for _, seg := range segments {
		if seg.isCode {
			codeCount++
		}
	}
	if codeCount != 2 {
		t.Errorf("expected 2 code segments, got %d", codeCount)
	}
}

func TestMdSplitKeepSep(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		sep      string
		wantMin  int // minimum number of parts
		wantMax  int // maximum number of parts
		contains []string
	}{
		{
			name:     "split on H2",
			text:     "Intro.\n\n## A\n\nContent A.\n\n## B\n\nContent B.",
			sep:      "\n## ",
			wantMin:  3,
			wantMax:  3,
			contains: []string{"Intro.", "## A", "## B"},
		},
		{
			name:    "no separator present",
			text:    "Just text.",
			sep:     "\n## ",
			wantMin: 1,
			wantMax: 1,
		},
		{
			name:     "header at document start",
			text:     "# Title\n\nContent.\n\n# Section\n\nMore.",
			sep:      "\n# ",
			wantMin:  2,
			wantMax:  2,
			contains: []string{"# Title", "# Section"},
		},
		{
			name:     "paragraph break",
			text:     "Para one.\n\nPara two.\n\nPara three.",
			sep:      "\n\n",
			wantMin:  3,
			wantMax:  3,
			contains: []string{"Para one.", "Para two.", "Para three."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := mdSplitKeepSep(tt.text, tt.sep)
			if len(parts) < tt.wantMin || len(parts) > tt.wantMax {
				t.Errorf("expected %d-%d parts, got %d: %v", tt.wantMin, tt.wantMax, len(parts), parts)
			}
			for _, want := range tt.contains {
				found := false
				for _, p := range parts {
					if strings.Contains(p, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected a part containing %q, parts: %v", want, parts)
				}
			}
		})
	}
}

func TestMarkdownSplitter_OnlyWhitespace(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 25, ChunkOverlap: 0}, tc)
	chunks, err := splitter.Split(context.Background(), "   \n\n\t  \n  ")
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for whitespace-only input, got %v", chunks)
	}
}

func TestMarkdownSplitter_SingleLongLine(t *testing.T) {
	splitter, _ := NewMarkdownSplitter(ChunkOptions{ChunkSize: 5, ChunkOverlap: 0}, tc)
	text := strings.Repeat("word ", 20) // 100 chars ≈ 25 tokens
	chunks, err := splitter.Split(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		tl := tokenLen(c)
		if tl > 5 {
			t.Errorf("chunk %d exceeds size: %d tokens", i, tl)
		}
	}
}
