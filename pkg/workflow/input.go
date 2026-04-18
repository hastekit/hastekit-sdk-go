package workflow

// Input is the run-level object a workflow invocation receives
// and accumulates into. RunContext is an opaque map the walker
// deep-merges node updates into; Status tracks per-node lifecycle
// markers; Metadata is a free-form host bag.
type Input struct {
	RunID      string                `json:"run_id"`
	RunContext map[string]any        `json:"run_context,omitempty"`
	Status     map[string]NodeStatus `json:"status,omitempty"`
	Metadata   map[string]any        `json:"metadata,omitempty"`
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
// are maps on both sides merge recursively (last writer wins at
// leaves); all other values replace.
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

// deepMerge copies src's keys into dst, recursing when both sides
// hold a map for the same key.
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

// ensureInit promotes a nil *Input to an empty one and initialises
// RunContext / Status so callers don't have to pre-populate them.
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
