// Package textsplitters provides text chunking by character length, token length,
// and allows adding more advanced strategies (semantic, sentence-aware, etc.) via the TextSplitter interface.
package textsplitters

import "context"

// TextSplitter splits text into chunks. Implementations can use character length,
// token length, or other strategies.
type TextSplitter interface {
	// Split splits the given text into one or more chunks.
	Split(ctx context.Context, text string) ([]string, error)
}
