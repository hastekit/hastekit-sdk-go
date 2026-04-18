package workflow

import "sync"

// RunState carries the shared state of a workflow run. It holds:
//
//   - The Input the walker was invoked with — RunID, the triggering
//     event, host-specific Metadata, AND the RunContext map that
//     accumulates per-node output as the run progresses. RunState's
//     State()/Get()/MergeState() methods all operate on
//     input.RunContext so there is a single source of truth.
//   - Per-node execution results (the "status plane").
//
// State model: every node returns a partial update (a map) which is
// shallow-merged into input.RunContext — last writer wins. The
// typed Input fields (RunID, Trigger, Metadata) are immutable across
// the run; only RunContext grows.
type RunState struct {
	mu sync.RWMutex

	input   *Input
	results map[string]*NodeResult
}

// NodeResult stores the outcome of a single node execution.
type NodeResult struct {
	NodeID string         `json:"node_id"`
	Status NodeStatus     `json:"status"`
	Port   string         `json:"port,omitempty"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// NewRunState allocates a fresh RunState bound to the given Input.
// A nil Input is promoted to an empty one; Input.RunContext is
// initialised so the walker can merge node outputs into it. Input
// is kept by reference and is readable via rs.Input() throughout
// the run.
func NewRunState(in *Input) *RunState {
	if in == nil {
		in = &Input{}
	}
	if in.RunContext == nil {
		in.RunContext = make(map[string]any)
	}
	return &RunState{
		input:   in,
		results: make(map[string]*NodeResult),
	}
}

// Input returns the workflow input this run was started with. Never
// nil — NewRunState promotes a nil argument to an empty Input.
func (rs *RunState) Input() *Input {
	return rs.input
}

// State returns a shallow copy of the current RunContext.
func (rs *RunState) State() map[string]any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make(map[string]any, len(rs.input.RunContext))
	for k, v := range rs.input.RunContext {
		out[k] = v
	}
	return out
}

// Get returns the RunContext value for key, if present.
func (rs *RunState) Get(key string) (any, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	v, ok := rs.input.RunContext[key]
	return v, ok
}

// MergeState shallow-merges update into input.RunContext. Keys in
// update overwrite existing keys. A nil or empty update is a no-op.
// This is the single write path the walker uses after each node
// completes.
func (rs *RunState) MergeState(update map[string]any) {
	if len(update) == 0 {
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for k, v := range update {
		rs.input.RunContext[k] = v
	}
}

// SetNodeResult stores the execution result for a node.
func (rs *RunState) SetNodeResult(nodeID string, result *NodeResult) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.results[nodeID] = result
}

// GetNodeResult returns the execution result for a node, if any.
func (rs *RunState) GetNodeResult(nodeID string) (*NodeResult, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	r, ok := rs.results[nodeID]
	return r, ok
}

// NodeResults returns a snapshot copy of every node's result keyed
// by id. Useful for observers (HTTP responses, traces) that need
// the full outcome after a run.
func (rs *RunState) NodeResults() map[string]*NodeResult {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make(map[string]*NodeResult, len(rs.results))
	for k, v := range rs.results {
		out[k] = v
	}
	return out
}

// snapshotState returns a shallow copy of the current RunContext.
// Used by the walker to hand each wave's invocations a consistent
// view while subsequent merges proceed.
func (rs *RunState) snapshotState() map[string]any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	if len(rs.input.RunContext) == 0 {
		return nil
	}
	out := make(map[string]any, len(rs.input.RunContext))
	for k, v := range rs.input.RunContext {
		out[k] = v
	}
	return out
}
