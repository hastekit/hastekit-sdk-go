// Package textsplitters provides text chunking by character length, token length,
// and allows adding more advanced strategies (semantic, sentence-aware, etc.) via the TextSplitter interface.
package textsplitters

// TextSplitter splits text into chunks. Implementations can use character length,
// token length, or other strategies.
type TextSplitter interface {
	// Split splits the given text into one or more chunks.
	Split(text string) ([]string, error)
}
