package workflow

// Input is the typed, single run-level object a workflow invocation
// receives and accumulates into. It carries:
//
//   - Identity (RunID) and the triggering event.
//   - RunContext: an opaque map[string]any that the walker fills
//     with whatever partial updates each node returns (shallow /
//     deep merge via MergeContext). The SDK has no opinion about
//     how hosts structure it — callers like the workflow-builder
//     gateway group data under their own keys (e.g. "inputs" /
//     "outputs" per nodeID) while raw SDK callers may keep it flat.
//   - Status: per-node lifecycle markers (running / completed /
//     failed / skipped). Deliberately minimal; per-node outputs
//     live in RunContext, not here.
//   - Metadata: free-form host bag (project_id, connector_id, …).
//
// All writes come from the walker's sequential merge loop (between
// waves) and its pre-dispatch Status markers. Node executors don't
// write directly to Input, so no mutex is needed — reads during a
// wave see a stable RunContext because nothing mutates it
// concurrently.
type Input struct {
	RunID      string                `json:"run_id"`
	Trigger    TriggerEvent          `json:"trigger"`
	RunContext map[string]any        `json:"run_context,omitempty"`
	Status     map[string]NodeStatus `json:"status,omitempty"`
	Metadata   map[string]any        `json:"metadata,omitempty"`
}

// TriggerEvent describes what caused a workflow to run.
type TriggerEvent struct {
	Source  string         `json:"source"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// NodeStatus tracks a node's lifecycle marker.
type NodeStatus string

const (
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
)

// MergeContext merges update into in.RunContext. Keys whose values
// are maps on BOTH sides merge recursively (last writer wins at
// leaves); all other values replace. This lets hosts that group
// per-node data under shared sub-keys (e.g. "inputs" / "outputs")
// compose partial updates across a wave without clobbering siblings.
func (in *Input) MergeContext(update map[string]any) {
	if len(update) == 0 {
		return
	}
	if in.RunContext == nil {
		in.RunContext = map[string]any{}
	}
	deepMerge(in.RunContext, update)
}

// SetStatus records a node's lifecycle marker.
func (in *Input) SetStatus(nodeID string, s NodeStatus) {
	if in.Status == nil {
		in.Status = map[string]NodeStatus{}
	}
	in.Status[nodeID] = s
}

// deepMerge shallow-copies src's top-level keys into dst, recursing
// when both sides hold a map for the same key.
func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if nested, ok := v.(map[string]any); ok {
			if existing, ok := dst[k].(map[string]any); ok {
				deepMerge(existing, nested)
				continue
			}
		}
		dst[k] = v
	}
}

// ensureInit fills in the mutable sub-maps the walker needs and
// promotes a nil *Input to an empty one. Called by Walker.Walk so
// callers never have to pre-populate RunContext / Status.
func ensureInit(in *Input) *Input {
	if in == nil {
		in = &Input{}
	}
	if in.RunContext == nil {
		in.RunContext = map[string]any{}
	}
	if in.Status == nil {
		in.Status = map[string]NodeStatus{}
	}
	return in
}
