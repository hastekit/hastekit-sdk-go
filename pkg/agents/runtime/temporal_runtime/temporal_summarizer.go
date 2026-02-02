package temporal_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.temporal.io/sdk/workflow"
)

type TemporalConversationSummarizer struct {
	wrappedSummarizer history.HistorySummarizer
}

func NewTemporalConversationSummarizer(wrappedSummarizer history.HistorySummarizer) *TemporalConversationSummarizer {
	return &TemporalConversationSummarizer{wrappedSummarizer: wrappedSummarizer}
}

func (t *TemporalConversationSummarizer) Summarize(ctx context.Context, msgIdToRunId map[string]string, messages []responses.InputMessageUnion, usage *responses.Usage) (*history.SummaryResult, error) {
	return t.wrappedSummarizer.Summarize(ctx, msgIdToRunId, messages, usage)
}

type TemporalConversationSummarizerProxy struct {
	workflowCtx workflow.Context
	prefix      string
}

func NewTemporalConversationSummarizerProxy(workflowCtx workflow.Context, prefix string) history.HistorySummarizer {
	return &TemporalConversationSummarizerProxy{
		workflowCtx: workflowCtx,
		prefix:      prefix,
	}
}

func (t *TemporalConversationSummarizerProxy) Summarize(ctx context.Context, msgIdToRunId map[string]string, messages []responses.InputMessageUnion, usage *responses.Usage) (*history.SummaryResult, error) {
	var summaryResult *history.SummaryResult
	err := workflow.ExecuteActivity(t.workflowCtx, t.prefix+"_SummarizerActivity", msgIdToRunId, messages, usage).Get(t.workflowCtx, &summaryResult)
	if err != nil {
		return nil, err
	}

	return summaryResult, nil
}
