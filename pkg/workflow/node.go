package workflow

import "context"

// NodeType identifies the kind of node in a workflow.
type NodeType string

// NodeStatus represents the execution status of a node.
type NodeStatus string

const (
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
)

// Node is the interface every workflow node implements. A Node
// describes behaviour only — it has no identity inside the graph.
// Identity (the per-instance id) is assigned by the Graph when the
// node is added.
//
//   - Validate is called at Compile time. Return an error for bad
//     configuration; no execution has happened yet.
//   - Execute is called at run time. The node reads whatever it needs
//     from the shared state via rs.State() or rs.Get(key), and returns
//     a partial state update (the output map) plus the port name
//     edges should follow. The walker shallow-merges the output into
//     the shared state — last writer wins.
type Node interface {
	Type() NodeType
	Validate() error
	Execute(ctx context.Context, rs *RunState) (output map[string]any, port string, err error)
}

// BaseNode is a convenience base every Node can embed to get Type()
// for free. It carries no identity — the Graph owns that.
type BaseNode struct {
	NodeType NodeType
}

// Type returns the node's kind.
func (b *BaseNode) Type() NodeType { return b.NodeType }

// NodeFactory builds a behaviour-only Node. Declarative layers
// (wfext) and durable runtimes (temporal) both use this single shape;
// host-side deps (http clients, connector services, …) are captured
// in the factory's closure.
type NodeFactory func() (Node, error)
