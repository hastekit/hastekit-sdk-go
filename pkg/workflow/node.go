package workflow

import "context"

// NodeType identifies the kind of node in a workflow.
type NodeType string

// Node is the interface every workflow node implements. Validate
// runs at Compile time; Execute runs at run time and returns a
// partial RunContext update plus the port name edges should follow.
type Node interface {
	Type() NodeType
	Validate() error
	Execute(ctx context.Context, in *Input) (output map[string]any, port string, err error)
}

// BaseNode is a convenience embedding that supplies Type().
type BaseNode struct {
	NodeType NodeType
}

// Type returns the node's kind.
func (b *BaseNode) Type() NodeType { return b.NodeType }

// NodeFactory builds a Node. Host-side dependencies are captured
// in the closure.
type NodeFactory func() (Node, error)
