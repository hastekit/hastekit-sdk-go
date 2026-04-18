package workflow

// Input is the typed input a workflow run receives. It mirrors
// agents.AgentInput's role on the agent side: one struct that
// carries every piece of per-run metadata hosts want available to
// nodes — the run's identity, the event that caused it, the
// per-node execution history, and a free-form bag for host-specific
// extensions.
//
// The engine takes Input by reference and mutates RunContext as the
// run progresses: each node's output is merged in under whatever
// keys the node/host chooses (typically the node's id). The walker
// hands each node's Execute a snapshot of RunContext taken at the
// wave's start.
//
// Metadata is intentionally untyped: the SDK has no opinion about
// tenancy, auth scope, or routing. Hosts put whatever their nodes
// need there (project_id, connector_id, ...) and read it back via
// RunState.Input().Metadata inside Execute.
type Input struct {
	RunID   string       `json:"run_id"`
	Trigger TriggerEvent `json:"trigger"`
	// RunContext is the shared mutable state that accumulates node
	// outputs across the run. It is the single source of truth for
	// "what has each node produced so far" and is what rs.State() /
	// rs.Get() / rs.MergeState() read and write.
	RunContext map[string]any `json:"run_context,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// TriggerEvent describes what caused a workflow to run. Source and
// Type identify the origin; Payload is the event's raw data.
type TriggerEvent struct {
	Source  string         `json:"source"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}
