package textsplitters

import (
	"context"
	"fmt"
	"strings"
)

// MarkdownSplitter splits markdown text by its structural elements.
// It uses a hierarchical splitting strategy: headers first, then horizontal
// rules, paragraph breaks, line breaks, and finally word boundaries.
// Fenced code blocks are kept intact when possible.
type MarkdownSplitter struct {
	opts ChunkOptions
}

// NewMarkdownSplitter creates a splitter that chunks markdown by structural elements.
// Uses ChunkOptions for chunk size and overlap (in characters).
func NewMarkdownSplitter(opts ChunkOptions) (*MarkdownSplitter, error) {
	if opts.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive, got %d", opts.ChunkSize)
	}
	if opts.ChunkOverlap < 0 || opts.ChunkOverlap >= opts.ChunkSize {
		return nil, fmt.Errorf("chunk overlap must be in [0, chunkSize), got %d", opts.ChunkOverlap)
	}
	return &MarkdownSplitter{opts: opts}, nil
}

// mdSeparators defines the hierarchy of markdown structural boundaries,
// from most significant to least significant.
var mdSeparators = []string{
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
}

// mdSegment represents a contiguous block of markdown text.
type mdSegment struct {
	text   string
	isCode bool
}

// Split splits markdown text into chunks that respect structural boundaries.
// Fenced code blocks are kept intact when possible. The text is split
// hierarchically on headers, then horizontal rules, paragraphs, lines,
// and finally word boundaries.
func (s *MarkdownSplitter) Split(ctx context.Context, text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	// Extract code blocks as atomic units.
	segments := splitByCodeBlocks(text)

	// Split non-code segments using the markdown separator hierarchy;
	// code segments are kept intact (or line-split if oversized).
	var pieces []string
	for _, seg := range segments {
		if seg.isCode {
			trimmed := strings.TrimSpace(seg.text)
			if trimmed == "" {
				continue
			}
			if len([]rune(trimmed)) <= s.opts.ChunkSize {
				pieces = append(pieces, trimmed)
			} else {
				// Code block exceeds chunk size — split by lines and spaces only.
				sub := s.mdSplitRecursive(seg.text, len(mdSeparators)-2)
				pieces = append(pieces, sub...)
			}
		} else {
			sub := s.mdSplitRecursive(seg.text, 0)
			pieces = append(pieces, sub...)
		}
	}

	if len(pieces) == 0 {
		return nil, nil
	}

	return s.mdMergeChunks(pieces), nil
}

// splitByCodeBlocks splits text into segments, separating fenced code blocks
// (``` delimited) from surrounding markdown text.
func splitByCodeBlocks(text string) []mdSegment {
	const fence = "```"
	var segments []mdSegment
	remaining := text

	for len(remaining) > 0 {
		openIdx := strings.Index(remaining, fence)
		if openIdx == -1 {
			segments = append(segments, mdSegment{text: remaining})
			break
		}

		// Text before the code block.
		if openIdx > 0 {
			segments = append(segments, mdSegment{text: remaining[:openIdx]})
		}

		// Find end of opening fence line.
		afterFence := remaining[openIdx+len(fence):]
		nlIdx := strings.Index(afterFence, "\n")
		if nlIdx == -1 {
			// No newline after opening fence — treat rest as code.
			segments = append(segments, mdSegment{text: remaining[openIdx:], isCode: true})
			break
		}

		// Find closing fence.
		body := afterFence[nlIdx+1:]
		closeIdx := strings.Index(body, fence)
		if closeIdx == -1 {
			// Unclosed code block — include the rest as code.
			segments = append(segments, mdSegment{text: remaining[openIdx:], isCode: true})
			break
		}

		// End position includes closing fence and optional trailing newline.
		endPos := openIdx + len(fence) + nlIdx + 1 + closeIdx + len(fence)
		if endPos < len(remaining) && remaining[endPos] == '\n' {
			endPos++
		}

		segments = append(segments, mdSegment{text: remaining[openIdx:endPos], isCode: true})
		remaining = remaining[endPos:]
	}

	return segments
}

// mdSplitRecursive splits text into pieces each ≤ ChunkSize using the
// separator hierarchy starting at sepIdx.
func (s *MarkdownSplitter) mdSplitRecursive(text string, sepIdx int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if len([]rune(trimmed)) <= s.opts.ChunkSize {
		return []string{trimmed}
	}

	for i := sepIdx; i < len(mdSeparators); i++ {
		sep := mdSeparators[i]
		parts := mdSplitKeepSep(trimmed, sep)
		if len(parts) <= 1 {
			continue
		}

		var pieces []string
		for _, part := range parts {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			if len([]rune(p)) <= s.opts.ChunkSize {
				pieces = append(pieces, p)
			} else {
				pieces = append(pieces, s.mdSplitRecursive(p, i+1)...)
			}
		}

		if len(pieces) > 0 {
			return pieces
		}
	}

	// Fallback: character-level split.
	fb, err := NewCharacterLengthSplitter(s.opts)
	if err != nil {
		return []string{trimmed}
	}
	chunks, err := fb.Split(context.Background(), trimmed)
	if err != nil || len(chunks) == 0 {
		return []string{trimmed}
	}
	return chunks
}

// mdSplitKeepSep splits text on sep, keeping the separator content attached
// to the start of each subsequent part. For separators that begin with "\n",
// the leading newline is dropped so headers retain their "#" prefix.
//
// A "\n" is prepended to the text before splitting so that separators like
// "\n# " also match headers at the very start of the document.
func mdSplitKeepSep(text, sep string) []string {
	keepSep := sep
	if strings.HasPrefix(sep, "\n") {
		keepSep = sep[1:]
	}

	// Prepend \n so that headers at the start of text are matched.
	normalized := "\n" + text

	if !strings.Contains(normalized, sep) {
		return []string{text}
	}

	parts := strings.Split(normalized, sep)
	if len(parts) <= 1 {
		return []string{text}
	}

	var result []string
	for i, p := range parts {
		var piece string
		if i == 0 {
			// Remove the leading \n that was prepended.
			piece = strings.TrimPrefix(p, "\n")
		} else {
			piece = keepSep + p
		}
		if strings.TrimSpace(piece) != "" {
			result = append(result, piece)
		}
	}

	if len(result) <= 1 {
		return []string{text}
	}

	return result
}

// mdMergeChunks combines small pieces into chunks that stay within ChunkSize,
// inserting ChunkOverlap characters of shared content between consecutive chunks.
func (s *MarkdownSplitter) mdMergeChunks(pieces []string) []string {
	if len(pieces) == 0 {
		return nil
	}

	size := s.opts.ChunkSize
	overlap := s.opts.ChunkOverlap
	const joiner = "\n\n"
	joinerLen := len([]rune(joiner))

	var chunks []string
	var buf []string
	bufLen := 0

	for _, piece := range pieces {
		pl := len([]rune(piece))

		addLen := pl
		if bufLen > 0 {
			addLen += joinerLen
		}

		if bufLen+addLen > size && bufLen > 0 {
			chunks = append(chunks, strings.Join(buf, joiner))

			if overlap > 0 {
				buf, bufLen = mdOverlapTail(buf, joinerLen, overlap)
			} else {
				buf = nil
				bufLen = 0
			}

			// Recalculate addLen after overlap.
			addLen = pl
			if bufLen > 0 {
				addLen += joinerLen
			}
		}

		buf = append(buf, piece)
		bufLen += addLen
	}

	if len(buf) > 0 {
		chunks = append(chunks, strings.Join(buf, joiner))
	}

	return chunks
}

// mdOverlapTail returns the trailing pieces from buf whose total length
// (including joiners) fits within the overlap limit.
func mdOverlapTail(buf []string, joinerLen, overlap int) ([]string, int) {
	var result []string
	total := 0

	for i := len(buf) - 1; i >= 0; i-- {
		pl := len([]rune(buf[i]))
		newTotal := pl
		if total > 0 {
			newTotal = total + joinerLen + pl
		}
		if newTotal > overlap && total > 0 {
			break
		}
		result = append([]string{buf[i]}, result...)
		total = newTotal
	}

	return result, total
}
