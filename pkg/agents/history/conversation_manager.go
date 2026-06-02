package history

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/agentstate"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.opentelemetry.io/otel"
)

var (
	tracer = otel.Tracer("History")
)

// ConversationMessage represents a message within a thread
type ConversationMessage struct {
	MessageID      string                        `json:"message_id" db:"message_id"`
	ThreadID       string                        `json:"thread_id" db:"thread_id"`
	ConversationID string                        `json:"conversation_id" db:"conversation_id"`
	Messages       []responses.InputMessageUnion `json:"messages" db:"messages"`
	Meta           map[string]any                `json:"meta" db:"meta"`
}

// AuthorContext returns the RunContext the row was authored under,
// recovered from its persisted Meta (see RunContextMetaKey). Returns nil
// for rows written before this was persisted, so callers treat them as
// unattributed. Keeps the Meta-key coupling in one place.
func (m ConversationMessage) AuthorContext() map[string]any {
	rc, _ := m.Meta[RunContextMetaKey].(map[string]any)
	return rc
}

// Summary represents a conversation summary stored in the summaries table
type Summary struct {
	ID                      string                      `json:"id" db:"id"`
	ThreadID                string                      `json:"thread_id" db:"thread_id"`
	SummaryMessage          responses.InputMessageUnion `json:"summary_message" db:"summary_message"`
	LastSummarizedMessageID string                      `json:"last_summarized_message_id" db:"last_summarized_message_id"`
	CreatedAt               time.Time                   `json:"created_at" db:"created_at"`
	Meta                    map[string]any              `json:"meta" db:"meta"`
}

type ConversationPersistenceAdapter interface {
	NewConversationID(ctx context.Context) string
	NewRunID(ctx context.Context) string
	LoadMessages(ctx context.Context, namespace string, threadID string, previousMessageID string) ([]ConversationMessage, error)
	SaveMessages(ctx context.Context, namespace, msgId, previousMsgId, threadID string, conversationId string, messages []responses.InputMessageUnion, meta map[string]any) error
	SaveSummary(ctx context.Context, namespace string, summary Summary) error
}

// AuthoredMessages is a group of messages paired with the RunContext
// they were authored under. Loaded history (one group per stored row,
// AuthorContext from the row's Meta) and newly-queued input (one group
// per message, AuthorContext from its submitter) reduce to the same
// shape so a single MessageProcessor pass handles both.
type AuthoredMessages struct {
	Messages      []responses.InputMessageUnion
	AuthorContext map[string]any // nil for legacy rows → passed through untouched
}

type CommonConversationManager struct {
	ConversationPersistenceAdapter ConversationPersistenceAdapter
	Summarizer                     HistorySummarizer

	Options []ConversationManagerOptions
}

func NewConversationManager(p ConversationPersistenceAdapter, opts ...ConversationManagerOptions) *CommonConversationManager {
	cm := &CommonConversationManager{
		ConversationPersistenceAdapter: p,
	}

	for _, o := range opts {
		o(cm)
	}

	return cm
}

type ConversationManagerOptions func(*CommonConversationManager)

func WithSummarizer(summarizer HistorySummarizer) ConversationManagerOptions {
	return func(cm *CommonConversationManager) {
		cm.Summarizer = summarizer
	}
}

type ConversationRunManager struct {
	ConversationPersistenceAdapter

	namespace      string
	conversationId string
	msgId          string
	previousMsgId  string
	msgIdToRunId   map[string]string
	threadId       string

	convMessages    []ConversationMessage
	oldMessages     []responses.InputMessageUnion
	newMessages     []responses.InputMessageUnion
	usage           *responses.Usage
	lastMessageMeta map[string]any

	// runContext
	runContext map[string]any

	// State is used to store any key-value pairs that need to be persisted along with the run
	State map[string]string

	// RunState is used to store the state of the run, such as the current step and the usage of the run
	RunState *agentstate.RunState

	summarizer HistorySummarizer
	summaries  *SummaryResult
}

func NewRun(ctx context.Context, cm *CommonConversationManager, namespace string, threadID string, previousMessageID string, messages []responses.InputMessageUnion, options ...RunOption) (*ConversationRunManager, error) {
	cr := &ConversationRunManager{
		ConversationPersistenceAdapter: cm.ConversationPersistenceAdapter,
		summarizer:                     cm.Summarizer,
		msgIdToRunId:                   make(map[string]string),
		State:                          make(map[string]string),
	}

	// Load messages
	_, err := cr.LoadMessages(ctx, namespace, threadID, previousMessageID)
	if err != nil {
		return nil, err
	}

	// Load the run state
	var runID string
	if cr.RunState == nil || cr.RunState.IsComplete() {
		// Create a new run id
		runID = cr.ConversationPersistenceAdapter.NewRunID(ctx)
		cr.RunState = agentstate.NewRunState()
	} else {
		// Continuing the previous run
		runID = cr.previousMsgId
	}
	cr.ProcessIncomingMessages(messages)

	// Store the run id
	cr.msgId = runID

	// Run the options
	for _, o := range options {
		o(cr)
	}

	if cr.conversationId == "" {
		cr.conversationId = cr.ConversationPersistenceAdapter.NewConversationID(ctx)
	}

	return cr, nil
}

type RunOption func(manager *ConversationRunManager)

func WithConversationID(cid string) RunOption {
	return func(cm *ConversationRunManager) {
		cm.conversationId = cid
	}
}

// RunContextMetaKey is the saved row meta entry under which
// SaveMessages records the run's RunContext (see WithRunContext).
const RunContextMetaKey = "run_context"

// WithRunContext records the run's RunContext, which SaveMessages
// stores in the saved row's meta under RunContextMetaKey. It is
// persistence-only — never sent to the provider — so later reads can
// recover the context a turn ran under (e.g. the inbound sender).
func WithRunContext(rc map[string]any) RunOption {
	return func(cm *ConversationRunManager) {
		cm.runContext = rc
	}
}

func (cm *ConversationRunManager) AddMessages(ctx context.Context, messages []responses.InputMessageUnion, usage *responses.Usage) {
	cm.newMessages = append(cm.newMessages, messages...)

	if usage != nil {
		cm.usage = usage
	}
}

func (cm *ConversationRunManager) GetMessages(ctx context.Context) ([]responses.InputMessageUnion, error) {
	// Process messages with summarizer if available
	if cm.summarizer != nil {
		summaryResult, err := cm.summarizer.Summarize(ctx, cm.msgIdToRunId, cm.oldMessages, cm.usage)
		if err != nil {
			return nil, err
		}

		// If a summary was created, track it for saving later and apply it to messages
		if summaryResult != nil {
			cm.summaries = summaryResult
			if summaryResult.Summary == nil {
				cm.oldMessages = summaryResult.MessagesToKeep
			} else {
				cm.oldMessages = append([]responses.InputMessageUnion{*summaryResult.Summary}, summaryResult.MessagesToKeep...)
			}
		}
	}

	// Add queued messages to the new messages
	cm.newMessages = append(cm.newMessages, cm.RunState.QueuedMessages...)
	cm.RunState.QueuedMessages = []responses.InputMessageUnion{}

	return append(cm.oldMessages, cm.newMessages...), nil
}

func (cm *ConversationRunManager) LoadMessages(ctx context.Context, namespace string, threadID string, previousMessageID string) ([]responses.InputMessageUnion, error) {
	cm.threadId = threadID

	if cm.ConversationPersistenceAdapter == nil {
		return []responses.InputMessageUnion{}, nil
	}

	// Don't have to reload
	if len(cm.oldMessages) > 0 {
		return cm.oldMessages, nil
	}

	convMessages, err := cm.ConversationPersistenceAdapter.LoadMessages(ctx, namespace, threadID, previousMessageID)
	if err != nil {
		return nil, err
	}

	messages := []responses.InputMessageUnion{}
	for _, msg := range convMessages {
		for _, m := range msg.Messages {
			cm.msgIdToRunId[m.ID()] = msg.MessageID
		}
		cm.threadId = msg.ThreadID
		cm.conversationId = msg.ConversationID
		cm.previousMsgId = msg.MessageID

		messages = append(messages, msg.Messages...)

		// Store the most recent message's meta for run state loading
		// The last message in the chain contains the current run state
		if msg.Meta != nil {
			cm.lastMessageMeta = msg.Meta
		}
	}

	// Initialize lastMessageMeta if no messages were found
	if cm.lastMessageMeta == nil {
		cm.lastMessageMeta = make(map[string]any)
	}

	cm.namespace = namespace
	cm.convMessages = convMessages
	cm.oldMessages = messages
	cm.RunState = agentstate.LoadRunStateFromMeta(cm.lastMessageMeta)
	cm.loadSubAgentContext(ctx)
	if cm.RunState != nil {
		cm.usage = &cm.RunState.Usage
	}

	return messages, nil
}

// GetMeta returns the meta from the most recent message
func (cm *ConversationRunManager) GetMeta() map[string]any {
	return cm.lastMessageMeta
}

// GetMessageID returns the current run id
func (cm *ConversationRunManager) GetMessageID() string {
	return cm.msgId
}

// GetConversationID GetOrCreateConversationID returns the conversation ID, if it doesn't exist it will create one
func (cm *ConversationRunManager) GetConversationID() string {
	return cm.conversationId
}

func (cm *ConversationRunManager) SaveMessages(ctx context.Context) error {
	meta := cm.RunState.ToMeta()
	if meta == nil {
		meta = map[string]any{}
	}

	meta["state"] = cm.State

	// Record the run's RunContext (persistence-only; never sent to the
	// provider) so later reads recover the turn's context.
	if len(cm.runContext) > 0 {
		meta[RunContextMetaKey] = cm.runContext
	}

	if cm.summaries != nil {
		sum := Summary{
			ID:                      cm.summaries.SummaryID,
			ThreadID:                cm.threadId,
			LastSummarizedMessageID: cm.summaries.LastSummarizedMessageID,
			CreatedAt:               time.Now(),
			Meta: map[string]any{
				"is_summary": true,
			},
		}

		if cm.summaries.Summary != nil {
			sum.SummaryMessage = *cm.summaries.Summary
		}

		if cm.ConversationPersistenceAdapter != nil {
			err := cm.ConversationPersistenceAdapter.SaveSummary(ctx, cm.namespace, sum)
			if err != nil {
				return err
			}
		}

		cm.summaries = nil
	}

	if cm.ConversationPersistenceAdapter != nil {
		err := cm.ConversationPersistenceAdapter.SaveMessages(ctx, cm.namespace, cm.msgId, cm.previousMsgId, cm.threadId, cm.conversationId, cm.newMessages, meta)
		if err != nil {
			return err
		}
	}

	runState := agentstate.LoadRunStateFromMeta(meta)
	if runState.IsComplete() {
		cm.previousMsgId = cm.msgId
		cm.msgId = uuid.NewString()
	}

	cm.lastMessageMeta = meta
	cm.oldMessages = append(cm.oldMessages, cm.newMessages...)
	cm.newMessages = []responses.InputMessageUnion{}

	return nil
}

func (cm *ConversationRunManager) TrackUsage(usage *responses.Usage) {
	if usage == nil {
		return
	}
	cm.RunState.Usage.InputTokens += usage.InputTokens
	cm.RunState.Usage.OutputTokens += usage.OutputTokens
	cm.RunState.Usage.InputTokensDetails.CachedTokens += usage.InputTokensDetails.CachedTokens
	cm.RunState.Usage.TotalTokens += usage.TotalTokens
}

func (cm *ConversationRunManager) loadSubAgentContext(ctx context.Context) {
	data := cm.lastMessageMeta["state"]

	if data == nil {
		return
	}

	buf, err := sonic.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal state", "error", err)
		return
	}

	if err = sonic.Unmarshal(buf, &cm.State); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal state", "error", err)
		return
	}
}

func (cm *ConversationRunManager) ProcessIncomingMessages(messages []responses.InputMessageUnion) {
	// Process incoming message, and extract tool approvals and user messages
	hasNewApproval := false
	for _, msg := range messages {
		if msg.OfFunctionCallApprovalResponse != nil {
			hasNewApproval = true
			r := msg.OfFunctionCallApprovalResponse
			cm.RunState.QueuedApprovals = append(cm.RunState.QueuedApprovals, r.ApprovedCallIds...)
			cm.RunState.QueuedRejections = append(cm.RunState.QueuedRejections, r.RejectedCallIds...)
		} else {
			cm.RunState.QueuedMessages = append(cm.RunState.QueuedMessages, msg)
		}
	}

	// If we are waiting for approval, and got an new approval message, move to execute tools
	//  - If new approval is not received, let the user messages be in the queue
	if cm.RunState.CurrentStep == agentstate.StepAwaitApproval {
		if hasNewApproval {
			cm.RunState.CurrentStep = agentstate.StepExecuteTools
		}
	}
}

func (cm *ConversationRunManager) ProcessPendingNestedToolCalls(parentToolCall responses.FunctionCallMessage, toolCalls []responses.FunctionCallMessage) {
	if cm.RunState.PendingNestedToolCalls == nil {
		cm.RunState.PendingNestedToolCalls = map[string]string{}
	}

	if cm.RunState.PausedToolCalls == nil {
		cm.RunState.PausedToolCalls = map[string]responses.FunctionCallMessage{}
	}

	cm.RunState.ToolsAwaitingApproval = append(cm.RunState.ToolsAwaitingApproval, toolCalls...)

	// Track the parent tool call ID and the nested tool call IDs
	for _, pendingApproval := range toolCalls {
		cm.RunState.PendingNestedToolCalls[pendingApproval.CallID] = parentToolCall.CallID
	}

	// Add the parent tool call to the paused tool calls
	cm.RunState.PausedToolCalls[parentToolCall.CallID] = parentToolCall
}
