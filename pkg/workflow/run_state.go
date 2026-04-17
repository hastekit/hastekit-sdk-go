package workflow

import "sync"

// RunState carries the shared state of a workflow run. It holds a
// single flat state map (the "data plane") — just like LangGraph —
// plus per-node execution results (the "status plane"). Host-specific
// metadata (tenancy, auth scope, sandbox identity) belongs on the
// context.Context passed into Node.Execute; the engine itself has no
// opinion about it.
//
// State model: every node returns a partial update (a map) which is
// shallow-merged into the shared state — last writer wins. Nodes read
// whatever they need directly from state via rs.State() or rs.Get().
// There is no per-node output namespace; if a node wants to publish
// under its own key it simply returns {"nodeID": value}.
type RunState struct {
	mu sync.RWMutex

	state   map[string]any
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

// NewRunState allocates a fresh RunState seeded with initial state
// (typically the invocation input). The initial map is cloned, so
// later mutations by the caller do not affect the run.
func NewRunState(initial map[string]any) *RunState {
	s := make(map[string]any, len(initial))
	for k, v := range initial {
		s[k] = v
	}
	return &RunState{
		state:   s,
		results: make(map[string]*NodeResult),
	}
}

// State returns a shallow copy of the current state map.
func (rs *RunState) State() map[string]any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make(map[string]any, len(rs.state))
	for k, v := range rs.state {
		out[k] = v
	}
	return out
}

// Get returns the state value for key, if present.
func (rs *RunState) Get(key string) (any, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	v, ok := rs.state[key]
	return v, ok
}

// MergeState shallow-merges update into the shared state. Keys in
// update overwrite existing state keys. A nil or empty update is a
// no-op. This is the single write path the walker uses after each
// node completes.
func (rs *RunState) MergeState(update map[string]any) {
	if len(update) == 0 {
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for k, v := range update {
		rs.state[k] = v
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

// NodeResults returns a snapshot copy of every node's result keyed by
// id. Useful for observers (HTTP responses, traces) that need the
// full outcome after a run.
func (rs *RunState) NodeResults() map[string]*NodeResult {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make(map[string]*NodeResult, len(rs.results))
	for k, v := range rs.results {
		out[k] = v
	}
	return out
}

// snapshotState returns a shallow copy of the current state. Used by
// the walker to hand each wave's invocations a consistent view.
func (rs *RunState) snapshotState() map[string]any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	if len(rs.state) == 0 {
		return nil
	}
	out := make(map[string]any, len(rs.state))
	for k, v := range rs.state {
		out[k] = v
	}
	return out
}
