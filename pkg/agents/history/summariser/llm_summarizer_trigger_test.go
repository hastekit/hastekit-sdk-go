package summariser

import (
	"context"
	"testing"
)

func makeRuns(n int) []Run {
	runs := make([]Run, n)
	for i := range runs {
		runs[i] = Run{RunID: string(rune('a' + i))}
	}
	return runs
}

// TestShouldSummarizeUsesCurrentContextSize verifies the trigger keys off the
// token count it is given (now the most recent call's size, i.e. current
// context occupancy) rather than firing unconditionally.
func TestShouldSummarizeUsesCurrentContextSize(t *testing.T) {
	s := NewLLMHistorySummarizer(&LLMHistorySummarizerOptions{
		TokenThreshold:  1000,
		KeepRecentCount: 5,
	})
	ctx := context.Background()
	runs := makeRuns(10)

	tests := []struct {
		name          string
		contextTokens int
		want          bool
	}{
		{"no context yet", 0, false},
		{"below threshold", 500, false},
		{"at threshold", 1000, true},
		{"above threshold", 5000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.shouldSummarize(ctx, runs, tt.contextTokens)
			if got != tt.want {
				t.Fatalf("shouldSummarize = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestShouldSummarizeNotEnoughRuns ensures we never summarize when there are
// not enough runs to keep the recent window AND summarize something.
func TestShouldSummarizeNotEnoughRuns(t *testing.T) {
	s := NewLLMHistorySummarizer(&LLMHistorySummarizerOptions{
		TokenThreshold:  1000,
		KeepRecentCount: 5,
	})
	ctx := context.Background()

	// Over threshold but only keepRecentCount runs -> nothing to summarize.
	got, idx := s.shouldSummarize(ctx, makeRuns(5), 5000)
	if got || idx != -1 {
		t.Fatalf("expected no summarization with %d runs, got (%v, %d)", 5, got, idx)
	}
}
