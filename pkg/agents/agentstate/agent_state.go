package agentstate

import (
	"github.com/bytedance/sonic"
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
func (s *RunState) ToMeta(traceid string) map[string]any {
	runStateMap := map[string]any{
		"status":         s.getStatus(),
		"current_step":   string(s.CurrentStep),
		"loop_iteration": s.LoopIteration,
		"usage":          s.Usage,
		"traceid":        traceid,
	}

	if len(s.PendingToolCalls) > 0 {
		runStateMap["pending_tool_calls"] = s.PendingToolCalls
	}

	if len(s.ToolsAwaitingApproval) > 0 {
		runStateMap["tools_awaiting_approval"] = s.ToolsAwaitingApproval
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

	return state
}
