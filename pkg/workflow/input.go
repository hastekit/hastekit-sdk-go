package workflow

// Input is the typed input a workflow run receives AND accumulates —
// one struct that carries every piece of per-run state the engine
// and nodes need: identity, the triggering event, the shared
// RunContext that grows as nodes run, per-node Results, and a
// free-form Metadata bag for host-specific extensions.
//
// The engine takes Input by reference and mutates RunContext +
// Results as the run progresses. The typed header fields (RunID,
// Trigger, Metadata) are expected to stay immutable once the walker
// begins, but the SDK does not enforce that.
//
// Metadata is intentionally untyped: the SDK has no opinion about
// tenancy, auth scope, or routing. Hosts put whatever their nodes
// need there (project_id, connector_id, ...) and read it back in
// Execute.
type Input struct {
	RunID   string       `json:"run_id"`
	Trigger TriggerEvent `json:"trigger"`
	// RunContext is the shared mutable state that accumulates node
	// outputs across the run. It is the single source of truth for
	// "what has each node produced so far" — the walker shallow-
	// merges each node's partial output update into this map after
	// the node completes.
	RunContext map[string]any `json:"run_context,omitempty"`
	// Results records per-node execution status + outcome (the
	// "status plane"). Populated by the walker as nodes fire;
	// observers (HTTP responses, traces) read it after the run.
	Results  map[string]*NodeResult `json:"results,omitempty"`
	Metadata map[string]any         `json:"metadata,omitempty"`
}

// TriggerEvent describes what caused a workflow to run.
type TriggerEvent struct {
	Source  string         `json:"source"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// ensureInit fills in the mutable sub-maps the walker needs to
// operate. Nil Input is promoted to an empty one. Called by
// Walker.Walk so callers don't have to pre-populate anything.
func ensureInit(in *Input) *Input {
	if in == nil {
		in = &Input{}
	}
	if in.RunContext == nil {
		in.RunContext = map[string]any{}
	}
	if in.Results == nil {
		in.Results = map[string]*NodeResult{}
	}
	return in
}
