package agentstate

import (
	"sort"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/messages"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// Step represents the current step in the agent execution state machine
type Step string

const (
	StepCallLLM       Step = "call_llm"
	StepExecuteTools  Step = "execute_tools"
	StepAwaitApproval Step = "await_approval"
	StepComplete      Step = "complete"
)

// RunStatus represents the overall status of a run
type RunStatus string

const (
	RunStatusInProgress RunStatus = "running"
	RunStatusPaused     RunStatus = "paused"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusError      RunStatus = "error"
)

// RunState encapsulates the execution state of an agent run
type RunState struct {
	CurrentStep           Step                            `json:"current_step"`
	LoopIteration         int                             `json:"loop_iteration"`
	Usage                 responses.Usage                 `json:"usage"`
	PendingToolCalls      []responses.FunctionCallMessage `json:"pending_tool_calls,omitempty"`
	ToolsAwaitingApproval []responses.FunctionCallMessage `json:"tools_awaiting_approval,omitempty"`
	QueuedApprovals       []string                        `json:"queued_approvals,omitempty"`
	QueuedRejections      []string                        `json:"queued_rejections,omitempty"`
	QueuedMessages        []messages.Message              `json:"queued_messages,omitempty"`
	TraceID               string                          `json:"traceid"`

	// PendingNestedToolCalls maps the parent tool call ID to the nested tool call ID
	PendingNestedToolCalls map[string]string `json:"pending_nested_tool_calls,omitempty"`

	// PausedToolCalls maps the parent tool call ID to the paused tool call
	PausedToolCalls map[string]responses.FunctionCallMessage `json:"paused_tool_calls,omitempty"`

	// LastAgentName is the name of the agent that was responding to the user last.
	LastAgentName string `json:"last_agent_name,omitempty"`
}

// NextStep returns what the agent should do next
func (s *RunState) NextStep() Step {
	return s.CurrentStep
}

// TransitionToLLM moves to the LLM call step and increments loop iteration
func (s *RunState) TransitionToLLM() {
	s.CurrentStep = StepCallLLM
	s.LoopIteration++
}

// TransitionToExecuteTools moves to tool execution step with the given tools
func (s *RunState) TransitionToExecuteTools(tools []responses.FunctionCallMessage) {
	s.CurrentStep = StepExecuteTools
	s.PendingToolCalls = tools
}

// TransitionToAwaitApproval moves to await approval step with the given tools
func (s *RunState) TransitionToAwaitApproval(tools []responses.FunctionCallMessage) {
	s.CurrentStep = StepAwaitApproval
	s.PendingToolCalls = tools
}

// TransitionToComplete moves to the complete step and clears pending tools
func (s *RunState) TransitionToComplete() {
	s.CurrentStep = StepComplete
	s.PendingToolCalls = nil
}

// ClearPendingTools clears the pending tool calls
func (s *RunState) ClearPendingTools() {
	s.PendingToolCalls = nil
}

// HasToolsAwaitingApproval returns true if there are tools waiting for approval
func (s *RunState) HasToolsAwaitingApproval() bool {
	return len(s.ToolsAwaitingApproval) > 0
}

// PromoteAwaitingToApproval moves tools awaiting approval to pending and transitions to await state
func (s *RunState) PromoteAwaitingToApproval() {
	s.CurrentStep = StepAwaitApproval
	s.PendingToolCalls = s.ToolsAwaitingApproval
	s.ToolsAwaitingApproval = nil
}

// IsPaused returns true if the state is awaiting approval
func (s *RunState) IsPaused() bool {
	return s.CurrentStep == StepAwaitApproval
}

// IsComplete returns true if the state is complete
func (s *RunState) IsComplete() bool {
	return s.CurrentStep == StepComplete
}

// NewRunState creates initial state for a fresh run
func NewRunState() *RunState {
	return &RunState{
		CurrentStep:   StepCallLLM,
		LoopIteration: 0,
		Usage:         responses.Usage{},
	}
}

// ToMeta converts RunState to a map for storage in messages.meta
func (s *RunState) ToMeta() map[string]any {
	runStateMap := map[string]any{
		"status":         s.getStatus(),
		"current_step":   string(s.CurrentStep),
		"loop_iteration": s.LoopIteration,
		"usage":          s.Usage,
		"traceid":        s.TraceID,
	}

	if len(s.PendingToolCalls) > 0 {
		runStateMap["pending_tool_calls"] = s.PendingToolCalls
	}

	if len(s.ToolsAwaitingApproval) > 0 {
		runStateMap["tools_awaiting_approval"] = s.ToolsAwaitingApproval
	}

	if len(s.QueuedApprovals) > 0 {
		runStateMap["queued_approvals"] = s.QueuedApprovals
	}

	if len(s.QueuedRejections) > 0 {
		runStateMap["queued_rejections"] = s.QueuedRejections
	}

	if len(s.QueuedMessages) > 0 {
		runStateMap["queued_messages"] = s.QueuedMessages
	}

	if len(s.PendingNestedToolCalls) > 0 {
		runStateMap["pending_nested_tool_calls"] = s.PendingNestedToolCalls
	}

	if len(s.PausedToolCalls) > 0 {
		runStateMap["paused_tool_calls"] = s.PausedToolCalls
	}

	if s.LastAgentName != "" {
		runStateMap["last_agent_name"] = s.LastAgentName
	}

	return map[string]any{
		"run_state": runStateMap,
	}
}

func (s *RunState) getStatus() RunStatus {
	switch s.CurrentStep {
	case StepCallLLM, StepExecuteTools:
		return RunStatusInProgress
	case StepAwaitApproval:
		return RunStatusPaused
	case StepComplete:
		return RunStatusCompleted
	default:
		return RunStatusError
	}
}

// LoadRunStateFromMeta loads RunState from messages.meta
func LoadRunStateFromMeta(meta map[string]any) *RunState {
	if meta == nil {
		return nil
	}

	runStateData, ok := meta["run_state"].(map[string]any)
	if !ok {
		return nil
	}

	state := &RunState{
		Usage: responses.Usage{},
	}

	if currentStep, ok := runStateData["current_step"].(string); ok {
		state.CurrentStep = Step(currentStep)
	}

	if loopIteration, ok := runStateData["loop_iteration"].(float64); ok {
		state.LoopIteration = int(loopIteration)
	}

	if usageData, ok := runStateData["usage"].(map[string]any); ok {
		// Parse usage from meta using JSON marshaling for proper type conversion
		usageBytes, err := sonic.Marshal(usageData)
		if err == nil {
			sonic.Unmarshal(usageBytes, &state.Usage)
		}
	}

	if pendingToolCalls, ok := runStateData["pending_tool_calls"]; ok {
		// Parse pending tool calls using JSON marshaling
		toolCallsBytes, err := sonic.Marshal(pendingToolCalls)
		if err == nil {
			sonic.Unmarshal(toolCallsBytes, &state.PendingToolCalls)
		}
	}

	if awaitingApproval, ok := runStateData["tools_awaiting_approval"]; ok {
		// Parse tools awaiting approval using JSON marshaling
		toolCallsBytes, err := sonic.Marshal(awaitingApproval)
		if err == nil {
			sonic.Unmarshal(toolCallsBytes, &state.ToolsAwaitingApproval)
		}
	}

	if queuedApprovals, ok := runStateData["queued_approvals"]; ok {
		queuedApprovalsBytes, err := sonic.Marshal(queuedApprovals)
		if err == nil {
			sonic.Unmarshal(queuedApprovalsBytes, &state.QueuedApprovals)
		}
	}

	if queuedRejections, ok := runStateData["queued_rejections"]; ok {
		queuedRejectionsBytes, err := sonic.Marshal(queuedRejections)
		if err == nil {
			sonic.Unmarshal(queuedRejectionsBytes, &state.QueuedRejections)
		}
	}

	if queuedMessages, ok := runStateData["queued_messages"]; ok {
		queuedMessagesBytes, err := sonic.Marshal(queuedMessages)
		if err == nil {
			sonic.Unmarshal(queuedMessagesBytes, &state.QueuedMessages)
		}
	}

	if pendingNestedToolCalls, ok := runStateData["pending_nested_tool_calls"]; ok {
		pendingNestedToolCallsBytes, err := sonic.Marshal(pendingNestedToolCalls)
		if err == nil {
			sonic.Unmarshal(pendingNestedToolCallsBytes, &state.PendingNestedToolCalls)
		}
	}

	if pausedToolCalls, ok := runStateData["paused_tool_calls"]; ok {
		pausedToolCallsBytes, err := sonic.Marshal(pausedToolCalls)
		if err == nil {
			sonic.Unmarshal(pausedToolCallsBytes, &state.PausedToolCalls)
		}
	}

	if lastAgentName, ok := runStateData["last_agent_name"].(string); ok {
		state.LastAgentName = lastAgentName
	}

	return state
}

// CollectNestedApprovalsForParent walks PendingNestedToolCalls for
// every inner CallID whose parent is parentCallID, then partitions
// those inner CallIDs by membership in QueuedApprovals /
// QueuedRejections.
//
// An inner CallID present in neither queue means the user hasn't
// decided that one yet — it's left out of both partitions. The
// inner agent will see only what's been decided and will pause again
// on the rest if any remain undecided after its own
// ProcessIncomingMessages run.
func (s *RunState) CollectNestedApprovalsForParent(parentCallID string) (approved, rejected []string) {
	// Iterate in a stable order. Map iteration order is randomized in Go,
	// which would make the resulting approved/rejected lists (and the
	// messages built from them) non-deterministic across Temporal/Restate
	// replays.
	innerIDs := make([]string, 0, len(s.PendingNestedToolCalls))
	for innerID := range s.PendingNestedToolCalls {
		innerIDs = append(innerIDs, innerID)
	}
	sort.Strings(innerIDs)

	for _, innerID := range innerIDs {
		if s.PendingNestedToolCalls[innerID] != parentCallID {
			continue
		}
		switch {
		case contains(s.QueuedRejections, innerID):
			rejected = append(rejected, innerID)
		case contains(s.QueuedApprovals, innerID):
			approved = append(approved, innerID)
		}
	}
	return approved, rejected
}

// contains is a small helper kept package-local to avoid pulling in
// slices for a single check (and to keep this file Go-version
// agnostic).
func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
