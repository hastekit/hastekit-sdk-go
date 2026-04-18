package workflow

// NodeResult stores the outcome of a single node execution.
type NodeResult struct {
	NodeID string         `json:"node_id"`
	Status NodeStatus     `json:"status"`
	Port   string         `json:"port,omitempty"`
	Output map[string]any `json:"output,omitempty"`
	Error  string         `json:"error,omitempty"`
}
