package workflow

import (
	"context"
	"sync/atomic"
	"testing"
)

// countingNode records how many times it executed and emits a fixed
// port, so tests can assert a node does not re-run across a resume.
type countingNode struct {
	BaseNode
	runs *atomic.Int64
	port string
}

func (n *countingNode) Validate() error { return nil }

func (n *countingNode) Execute(_ context.Context, _ *Input) (map[string]any, string, error) {
	n.runs.Add(1)
	port := n.port
	if port == "" {
		port = DefaultPort
	}
	return map[string]any{"ran": true}, port, nil
}

// gateNode pauses until a resume decision is staged for its node id,
// then emits that decision's port. It mimics the approval node.
type gateNode struct {
	BaseNode
	nodeID string
	runs   *atomic.Int64
}

func (n *gateNode) Validate() error { return nil }

func (n *gateNode) Execute(_ context.Context, in *Input) (map[string]any, string, error) {
	n.runs.Add(1)
	if dec, ok := in.Resume(n.nodeID); ok {
		return map[string]any{"decided": true}, dec["decision"].(string), nil
	}
	return nil, "", Pause(map[string]any{"title": "approve me"})
}

func buildGateGraph(t *testing.T, beforeRuns, approvedRuns, rejectedRuns *atomic.Int64) *Compiled {
	t.Helper()
	g := NewGraph("gate-graph")
	g.AddNode("before", &countingNode{BaseNode: BaseNode{NodeType: "before"}, runs: beforeRuns})
	g.AddNode("gate", &gateNode{BaseNode: BaseNode{NodeType: "gate"}, nodeID: "gate", runs: new(atomic.Int64)})
	g.AddNode("approved", &countingNode{BaseNode: BaseNode{NodeType: "approved"}, runs: approvedRuns})
	g.AddNode("rejected", &countingNode{BaseNode: BaseNode{NodeType: "rejected"}, runs: rejectedRuns})
	g.AddEdge("before", "gate")
	g.AddEdgeOnPort("gate", "approved", "approved")
	g.AddEdgeOnPort("gate", "rejected", "rejected")
	g.AddEdge("approved", EndNode)
	g.AddEdge("rejected", EndNode)
	c, err := g.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return c
}

func TestWalkPausesAtGate(t *testing.T) {
	var before, approved, rejected atomic.Int64
	c := buildGateGraph(t, &before, &approved, &rejected)

	out, err := InProcessRuntime{}.Execute(context.Background(), c, &Input{RunID: "r1"}, RuntimeOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Pause == nil {
		t.Fatalf("expected run to be paused")
	}
	if out.Pause.NodeID != "gate" {
		t.Errorf("paused node = %q, want %q", out.Pause.NodeID, "gate")
	}
	if out.Pause.Payload["title"] != "approve me" {
		t.Errorf("pause payload title = %v, want %q", out.Pause.Payload["title"], "approve me")
	}
	if out.Status["before"] != NodeStatusCompleted {
		t.Errorf("before status = %q, want completed", out.Status["before"])
	}
	if out.Status["gate"] != NodeStatusPaused {
		t.Errorf("gate status = %q, want paused", out.Status["gate"])
	}
	if before.Load() != 1 {
		t.Errorf("before ran %d times, want 1", before.Load())
	}
	if approved.Load() != 0 || rejected.Load() != 0 {
		t.Errorf("downstream nodes should not have run before approval")
	}
}

func TestWalkResumesOnApprovedPort(t *testing.T) {
	var before, approved, rejected atomic.Int64
	c := buildGateGraph(t, &before, &approved, &rejected)

	out, err := InProcessRuntime{}.Execute(context.Background(), c, &Input{RunID: "r1"}, RuntimeOptions{})
	if err != nil {
		t.Fatalf("first walk: %v", err)
	}

	// Stage the approval decision and re-run from the saved Input.
	out.SetResume("gate", map[string]any{"decision": "approved"})
	out, err = InProcessRuntime{}.Execute(context.Background(), c, out, RuntimeOptions{})
	if err != nil {
		t.Fatalf("resume walk: %v", err)
	}

	if out.Pause != nil {
		t.Fatalf("expected run to complete, still paused at %q", out.Pause.NodeID)
	}
	if before.Load() != 1 {
		t.Errorf("before re-ran on resume: ran %d times, want 1 (idempotent short-circuit)", before.Load())
	}
	if approved.Load() != 1 {
		t.Errorf("approved branch ran %d times, want 1", approved.Load())
	}
	if rejected.Load() != 0 {
		t.Errorf("rejected branch ran %d times, want 0", rejected.Load())
	}
	if out.Status["gate"] != NodeStatusCompleted {
		t.Errorf("gate status after resume = %q, want completed", out.Status["gate"])
	}
}

func TestWalkResumesOnRejectedPort(t *testing.T) {
	var before, approved, rejected atomic.Int64
	c := buildGateGraph(t, &before, &approved, &rejected)

	out, _ := InProcessRuntime{}.Execute(context.Background(), c, &Input{RunID: "r1"}, RuntimeOptions{})
	out.SetResume("gate", map[string]any{"decision": "rejected"})
	out, err := InProcessRuntime{}.Execute(context.Background(), c, out, RuntimeOptions{})
	if err != nil {
		t.Fatalf("resume walk: %v", err)
	}

	if out.Pause != nil {
		t.Fatalf("expected completion, still paused")
	}
	if rejected.Load() != 1 {
		t.Errorf("rejected branch ran %d times, want 1", rejected.Load())
	}
	if approved.Load() != 0 {
		t.Errorf("approved branch ran %d times, want 0", approved.Load())
	}
}
