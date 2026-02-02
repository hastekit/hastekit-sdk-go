package history

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// SummaryResult contains the result of summarization including metadata needed for saving
type SummaryResult struct {
	Summary                 *responses.InputMessageUnion // The summary message
	MessagesToKeep          []responses.InputMessageUnion
	LastSummarizedMessageID string // ID of the last message that was summarized
	SummaryID               string // Unique ID for the summary (generated if empty)
}

type HistorySummarizer interface {
	// Summarize takes a list of messages and returns a summary result.
	// If summarization is not needed, returns a result with KeepFromIndex = -1.
	Summarize(ctx context.Context, msgIdToRunId map[string]string, messages []responses.InputMessageUnion, usage *responses.Usage) (*SummaryResult, error)
}
