package summariser

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type SlidingWindowHistorySummarizer struct {
	keepCount int // Number of recent runs to keep
}

type SlidingWindowHistorySummarizerOptions struct {
	KeepCount int // Number of recent runs to keep
}

func NewSlidingWindowHistorySummarizer(opts *SlidingWindowHistorySummarizerOptions) *SlidingWindowHistorySummarizer {
	return &SlidingWindowHistorySummarizer{
		keepCount: opts.KeepCount,
	}
}

// Summarize implements the HistorySummarizer interface.
// For sliding window, we simply keep the most recent N runs and discard the rest.
// We don't create a summary message, we just return which messages to keep.
func (s *SlidingWindowHistorySummarizer) Summarize(ctx context.Context, msgIdToRunId map[string]string, messages []responses.InputMessageUnion, usage *responses.Usage) (*history.SummaryResult, error) {
	// Group messages by their run ID
	type Run struct {
		RunID    string
		Messages []responses.InputMessageUnion
	}

	runs := []Run{}
	runIdsSeen := []string{}
	for _, msg := range messages {
		runId := msgIdToRunId[msg.ID()]

		if !slices.Contains(runIdsSeen, runId) {
			runs = append(runs, Run{
				RunID:    runId,
				Messages: []responses.InputMessageUnion{msg},
			})
			runIdsSeen = append(runIdsSeen, runId)
			continue
		}

		// Add message to the last run (assumes messages are grouped by run ID)
		run := &runs[len(runs)-1]
		run.Messages = append(run.Messages, msg)
	}

	// If we have fewer or equal runs than keepCount, keep everything
	if len(runs) <= s.keepCount {
		return nil, nil
	}

	// Keep only the most recent keepCount runs
	keepFromIndex := len(runs) - s.keepCount
	runsToKeep := runs[keepFromIndex:]
	runsToDiscard := runs[:keepFromIndex]

	// Collect all messages from runs to keep
	messagesToKeep := []responses.InputMessageUnion{}
	for _, run := range runsToKeep {
		messagesToKeep = append(messagesToKeep, run.Messages...)
	}

	// Find the last message ID that was discarded (the last run ID before keepFromIndex)
	var lastDiscardedRunID string
	for i := len(runsToDiscard) - 1; i >= 0; i-- {
		if runsToDiscard[i].RunID != "" {
			lastDiscardedRunID = runsToDiscard[i].RunID
			break
		}
	}

	// For sliding window, we don't create a summary message
	// We just return the messages to keep
	// The LastSummarizedMessageID represents the last run ID that was discarded
	return &history.SummaryResult{
		LastSummarizedMessageID: lastDiscardedRunID,
		SummaryID:               uuid.NewString(),
		MessagesToKeep:          messagesToKeep,
	}, nil
}
