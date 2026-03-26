// Package codesplitters provides AST-aware code chunking for source files.
// Language-specific parsing is handled by LanguageParser implementations
// (e.g. goparser.GoParser), making it easy to add support for new languages.
package codesplitters

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// TextSplitter splits text into chunks.
type TextSplitter interface {
	Split(ctx context.Context, text string) ([]string, error)
}

// TokenCounter returns the number of tokens in text. Implementations can use
// model-specific tokenizers (e.g. tiktoken) or approximations.
type TokenCounter interface {
	CountTokens(text string) (int, error)
}

// EstimatorCounter estimates token count from character count (~4 chars per token).
type EstimatorCounter struct {
	CharsPerToken int
}

// DefaultEstimatorCounter uses 4 characters per token (typical for English).
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

// LanguageParser extracts structured code chunks from source code for a
// specific programming language. Implementations use language-specific parsers
// (e.g. tree-sitter) to identify functions, methods, types, and other
// top-level declarations.
type LanguageParser interface {
	// Parse parses source code and returns the top-level code chunks found.
	Parse(source []byte) ([]CodeChunk, error)

	// Language returns the language identifier (e.g. "go", "python", "typescript").
	Language() string

	// Extensions returns the file extensions this parser handles (e.g. [".go"]).
	Extensions() []string
}

// CodeChunk represents a parsed top-level code region extracted by a LanguageParser.
type CodeChunk struct {
	// Kind is the declaration kind: "function", "method", "type", "class", etc.
	Kind string
	// Name is the symbol name (e.g. function name, type name).
	Name string
	// Package is the package/module name, if applicable.
	Package string
	// Signature is the parameters and return type. Empty for type declarations.
	Signature string
	// Receiver is the method receiver (e.g. "(c *Client)"). Empty for non-methods.
	Receiver string
	// DocComment is the immediately preceding doc comment, if any.
	DocComment string
	// Exported is true if the symbol is exported/public.
	Exported bool
	// StartLine is the 1-based start line.
	StartLine uint
	// EndLine is the 1-based end line.
	EndLine uint
	// Code is the raw source text of the entire declaration (including body).
	Code string
	// Snippet is a short form: signature only for funcs, first line for types.
	Snippet string
}

// Options configures the CodeSplitter behavior.
type Options struct {
	// MaxTokens is the target maximum tokens per output chunk (default: 512).
	MaxTokens int
	// MinTokens is the minimum tokens for a chunk to be emitted (default: 20).
	MinTokens int
	// OverlapTokens is the token overlap when splitting large declarations (default: 50).
	OverlapTokens int
	// IncludeFullCode includes full function/method bodies. If false, only
	// signature + doc comment are included (smaller, faster indexing).
	IncludeFullCode bool
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		MaxTokens:       512,
		MinTokens:       20,
		OverlapTokens:   50,
		IncludeFullCode: true,
	}
}

// CodeSplitter implements textsplitters.TextSplitter by using a LanguageParser
// to extract AST-aware code chunks. It formats each chunk with contextual
// headers (package, symbol kind, signature) and splits large declarations into
// overlapping sub-chunks.
type CodeSplitter struct {
	parser  LanguageParser
	opts    Options
	counter TokenCounter
}

// Compile-time check that CodeSplitter implements TextSplitter.
var _ TextSplitter = (*CodeSplitter)(nil)

// NewCodeSplitter creates a CodeSplitter for the given language parser.
// counter is used for token estimation; if nil, the default estimator (4 chars/token) is used.
func NewCodeSplitter(parser LanguageParser, opts Options, counter TokenCounter) (*CodeSplitter, error) {
	if parser == nil {
		return nil, fmt.Errorf("language parser is required")
	}
	if counter == nil {
		counter = DefaultEstimatorCounter
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 512
	}
	if opts.MinTokens <= 0 {
		opts.MinTokens = 20
	}
	return &CodeSplitter{
		parser:  parser,
		opts:    opts,
		counter: counter,
	}, nil
}

// Split parses the source code text and returns formatted, context-rich chunks
// suitable for embedding. Each chunk contains a header with metadata (package,
// kind, name, signature) followed by the code. Large declarations are split
// into overlapping sub-chunks.
func (s *CodeSplitter) Split(ctx context.Context, text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	chunks, err := s.parser.Parse([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("parse source: %w", err)
	}

	if len(chunks) == 0 {
		return nil, nil
	}

	var result []string
	for _, chunk := range chunks {
		formatted := s.formatChunk(chunk)
		if len(formatted) == 0 {
			continue
		}
		result = append(result, formatted...)
	}

	return result, nil
}

// formatChunk converts a CodeChunk into one or more formatted text strings.
// If the chunk fits within MaxTokens, a single string is returned.
// Otherwise it is split into overlapping sub-chunks, each prefixed with the header.
func (s *CodeSplitter) formatChunk(chunk CodeChunk) []string {
	header := buildHeader(chunk)

	codeContent := chunk.Code
	if !s.opts.IncludeFullCode {
		codeContent = chunk.Snippet
	}

	fullText := header + "\nCode:\n" + codeContent
	tokens, _ := s.counter.CountTokens(fullText)

	// If within limits, return as single chunk (if above minimum)
	if tokens <= s.opts.MaxTokens {
		if tokens >= s.opts.MinTokens {
			return []string{fullText}
		}
		return nil
	}

	// Split large code into overlapping sub-chunks, each with the header
	return s.splitLargeChunk(header, codeContent)
}

// splitLargeChunk splits code that exceeds MaxTokens into overlapping pieces,
// each prefixed with the header.
func (s *CodeSplitter) splitLargeChunk(header, code string) []string {
	headerTokens, _ := s.counter.CountTokens(header + "\nCode:\n")
	maxCodeTokens := s.opts.MaxTokens - headerTokens - 10 // small buffer
	if maxCodeTokens < 100 {
		maxCodeTokens = 100
	}

	lines := strings.Split(code, "\n")

	var result []string
	var currentLines []string
	currentTokens := 0
	partIndex := 0

	flush := func() {
		if len(currentLines) == 0 {
			return
		}
		codeContent := strings.Join(currentLines, "\n")
		tokens, _ := s.counter.CountTokens(codeContent)
		if tokens < s.opts.MinTokens {
			return
		}

		var sb strings.Builder
		sb.WriteString(header)
		if partIndex > 0 {
			sb.WriteString(fmt.Sprintf("\n[Part %d]\n", partIndex+1))
		}
		sb.WriteString("\nCode:\n")
		sb.WriteString(codeContent)
		result = append(result, sb.String())
		partIndex++
	}

	for _, line := range lines {
		lineTokens, _ := s.counter.CountTokens(line)

		if currentTokens+lineTokens > maxCodeTokens && len(currentLines) > 0 {
			flush()

			// Start new chunk with overlap from end of previous
			overlapLines := getOverlapLines(currentLines, s.opts.OverlapTokens, s.counter)
			currentLines = overlapLines
			currentTokens = 0
			for _, ol := range overlapLines {
				t, _ := s.counter.CountTokens(ol)
				currentTokens += t
			}
		}

		currentLines = append(currentLines, line)
		currentTokens += lineTokens

		// Handle very long single lines
		if lineTokens > maxCodeTokens && len(currentLines) == 1 {
			flush()
			currentLines = nil
			currentTokens = 0
		}
	}

	flush()
	return result
}

// buildHeader creates the context header for a code chunk.
func buildHeader(chunk CodeChunk) string {
	var sb strings.Builder

	if chunk.Package != "" {
		sb.WriteString("Package: ")
		sb.WriteString(chunk.Package)
		sb.WriteString("\n")
	}

	switch chunk.Kind {
	case "function":
		sb.WriteString("Function: ")
		sb.WriteString(chunk.Name)
		if chunk.Signature != "" {
			sb.WriteString(chunk.Signature)
		}
	case "method":
		sb.WriteString("Method: ")
		if chunk.Receiver != "" {
			sb.WriteString(chunk.Receiver)
			sb.WriteString(" ")
		}
		sb.WriteString(chunk.Name)
		if chunk.Signature != "" {
			sb.WriteString(chunk.Signature)
		}
	case "type", "class", "struct", "interface":
		sb.WriteString("Type: ")
		sb.WriteString(chunk.Name)
	default:
		sb.WriteString(strings.Title(chunk.Kind))
		sb.WriteString(": ")
		sb.WriteString(chunk.Name)
	}
	sb.WriteString("\n")

	if chunk.DocComment != "" {
		sb.WriteString("\nDocumentation:\n")
		sb.WriteString(strings.TrimSpace(chunk.DocComment))
		sb.WriteString("\n")
	}

	return sb.String()
}

// getOverlapLines returns the last lines that fit within the overlap token budget.
func getOverlapLines(lines []string, overlapTokens int, counter TokenCounter) []string {
	if overlapTokens <= 0 || len(lines) == 0 {
		return nil
	}

	var result []string
	tokens := 0

	for i := len(lines) - 1; i >= 0; i-- {
		lineTokens, _ := counter.CountTokens(lines[i])
		if tokens+lineTokens > overlapTokens {
			break
		}
		result = append([]string{lines[i]}, result...)
		tokens += lineTokens
	}

	return result
}
