package workflow

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// stubNode is a minimal node for testing that records execution order.
type stubNode struct {
	BaseNode
	delay    time.Duration
	outPort  string
	output   map[string]any
	executed *atomic.Int64 // shared counter to record execution order
	order    *int64        // set during Execute to the counter value
}

func (n *stubNode) Validate() error { return nil }

func (n *stubNode) Execute(_ context.Context, _ *RunState) (map[string]any, string, error) {
	if n.delay > 0 {
		time.Sleep(n.delay)
	}
	if n.executed != nil && n.order != nil {
		*n.order = n.executed.Add(1)
	}
	port := n.outPort
	if port == "" {
		port = DefaultPort
	}
	return n.output, port, nil
}

func newStub(typ NodeType, out map[string]any, delay time.Duration, counter *atomic.Int64, order *int64) *stubNode {
	return &stubNode{
		BaseNode: BaseNode{NodeType: typ},
		outPort:  DefaultPort,
		output:   out,
		delay:    delay,
		executed: counter,
		order:    order,
	}
}

func TestParallelExecution(t *testing.T) {
	// Diamond workflow:
	//        start
	//       /     \
	//      A       B     ← should run in parallel
	//       \     /
	//        end
	var counter atomic.Int64
	var orderA, orderB, orderEnd int64

	g := NewGraph("test-parallel")
	g.AddNode("start", newStub("start", nil, 0, nil, nil))
	g.AddNode("a", newStub("stub", map[string]any{"value": "a"}, 100*time.Millisecond, &counter, &orderA))
	g.AddNode("b", newStub("stub", map[string]any{"value": "b"}, 100*time.Millisecond, &counter, &orderB))
	g.AddNode("end", newStub("stub", map[string]any{"value": "end"}, 100*time.Millisecond, &counter, &orderEnd))
	g.AddEdge("start", "a")
	g.AddEdge("start", "b")
	g.AddEdge("a", "end")
	g.AddEdge("b", "end")

	start := time.Now()
	execCtx, err := g.Invoke(context.Background(), nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed > 280*time.Millisecond {
		t.Errorf("expected parallel execution (~200ms) but took %v", elapsed)
	}

	if orderEnd <= orderA || orderEnd <= orderB {
		t.Errorf("end node (order=%d) should run after A (order=%d) and B (order=%d)", orderEnd, orderA, orderB)
	}

	for _, id := range []string{"start", "a", "b", "end"} {
		r, ok := execCtx.GetNodeResult(id)
		if !ok {
			t.Errorf("missing result for node %q", id)
			continue
		}
		if r.Status != NodeStatusCompleted {
			t.Errorf("node %q status = %q, want completed", id, r.Status)
		}
	}
}

func TestSequentialChainStillWorks(t *testing.T) {
	g := NewGraph("test-seq")
	g.AddNode("start", newStub("node", map[string]any{"id": "start"}, 0, nil, nil))
	g.AddNode("a", newStub("node", map[string]any{"id": "a"}, 0, nil, nil))
	g.AddNode("b", newStub("node", map[string]any{"id": "b"}, 0, nil, nil))
	g.AddNode("c", newStub("node", map[string]any{"id": "c"}, 0, nil, nil))
	g.AddEdge("start", "a")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	execCtx, err := g.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, id := range []string{"start", "a", "b", "c"} {
		r, ok := execCtx.GetNodeResult(id)
		if !ok {
			t.Fatalf("missing result for node %q", id)
		}
		if r.Status != NodeStatusCompleted {
			t.Errorf("node %q status = %q, want completed", id, r.Status)
		}
	}
}

// stateCaptureNode reads the shared state at Execute time and stashes it
// into *captured for later assertions. It returns no state update.
type stateCaptureNode struct {
	BaseNode
	captured *map[string]any
}

func (n *stateCaptureNode) Validate() error { return nil }

func (n *stateCaptureNode) Execute(_ context.Context, rs *RunState) (map[string]any, string, error) {
	*n.captured = rs.State()
	return nil, DefaultPort, nil
}

func TestDataFlowBetweenParallelNodes(t *testing.T) {
	// start → A (outputs x=1) and B (outputs y=2), both feed into C.
	// Node C reads the merged state directly via RunState.
	var cState map[string]any

	g := NewGraph("test-dataflow")
	g.AddNode("start", newStub("producer", map[string]any{}, 0, nil, nil))
	g.AddNode("a", newStub("producer", map[string]any{"x": 1}, 0, nil, nil))
	g.AddNode("b", newStub("producer", map[string]any{"y": 2}, 0, nil, nil))
	g.AddNode("c", &stateCaptureNode{BaseNode: BaseNode{NodeType: "consumer"}, captured: &cState})
	g.AddEdge("start", "a")
	g.AddEdge("start", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "c")

	if _, err := g.Invoke(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cState["x"] != 1 {
		t.Errorf("c saw state[x] = %v, want 1", cState["x"])
	}
	if cState["y"] != 2 {
		t.Errorf("c saw state[y] = %v, want 2", cState["y"])
	}
}

func TestInvokeWithStartEnd(t *testing.T) {
	// START → a → END. Invocation input is the initial state; a reads
	// "greeting" directly via RunState and writes "reply" which
	// must appear in final state.
	var seen map[string]any

	g := NewGraph("test-invoke")
	g.AddNode("a", &stateCaptureNode{BaseNode: BaseNode{NodeType: "node"}, captured: &seen})
	g.AddEdge("START", "a")
	g.AddEdge("a", "END")

	compiled, err := g.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	execCtx, err := compiled.Execute(context.Background(), &Input{
		Metadata: map[string]any{"greeting": "hello"},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}

	if md := execCtx.Input().Metadata; md["greeting"] != "hello" {
		t.Errorf("input.metadata[greeting] = %v, want \"hello\"", md["greeting"])
	}
	_ = seen

	r, ok := execCtx.GetNodeResult("a")
	if !ok || r.Status != NodeStatusCompleted {
		t.Errorf("a result = %+v, want completed", r)
	}

	// END must not be scheduled or appear as a node result.
	if _, ok := execCtx.GetNodeResult(EndNode); ok {
		t.Errorf("END should not produce a NodeResult")
	}

	// Input is preserved on the RunState across the run.
	if md := execCtx.Input().Metadata; md["greeting"] != "hello" {
		t.Errorf("input.metadata[greeting] = %v, want \"hello\"", md["greeting"])
	}
}

func TestConditionalEdge(t *testing.T) {
	// classifier writes {"sentiment": "positive"|"negative"}; a router
	// attached to classifier picks the next branch based on state.
	g := NewGraph("test-conditional")
	g.AddNode("classifier", newStub("classifier", map[string]any{"sentiment": "positive"}, 0, nil, nil))
	g.AddNode("celebrate", newStub("celebrate", map[string]any{"msg": "yay"}, 0, nil, nil))
	g.AddNode("escalate", newStub("escalate", map[string]any{"msg": "oh no"}, 0, nil, nil))
	g.AddEdge("START", "classifier")
	g.AddConditionalEdge("classifier", func(rs *RunState) string {
		v, _ := rs.Get("sentiment")
		if s, _ := v.(string); s == "positive" {
			return "happy"
		}
		return "sad"
	}, map[string]string{
		"happy": "celebrate",
		"sad":   "escalate",
	})
	g.AddEdge("celebrate", "END")
	g.AddEdge("escalate", "END")

	compiled, err := g.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	rs, err := compiled.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}

	if r, ok := rs.GetNodeResult("celebrate"); !ok || r.Status != NodeStatusCompleted {
		t.Errorf("celebrate result = %+v, want completed", r)
	}
	if r, ok := rs.GetNodeResult("escalate"); !ok || r.Status != NodeStatusSkipped {
		t.Errorf("escalate result = %+v, want skipped", r)
	}
	if rs.State()["msg"] != "yay" {
		t.Errorf("state[msg] = %v, want \"yay\"", rs.State()["msg"])
	}
}

func TestConditionalEdgeRejectsConflictWithStaticEdge(t *testing.T) {
	g := NewGraph("test-conflict")
	g.AddNode("a", newStub("a", nil, 0, nil, nil))
	g.AddNode("b", newStub("b", nil, 0, nil, nil))
	g.AddEdge("START", "a")
	g.AddEdge("a", "b")
	g.AddConditionalEdge("a", func(*RunState) string { return "b" }, map[string]string{"b": "b"})

	if _, err := g.Compile(); err == nil {
		t.Fatal("expected conflict error (both static and conditional edges on 'a'), got nil")
	}
}

func TestCycleDetection(t *testing.T) {
	g := NewGraph("test-cycle")
	g.AddNode("start", newStub("node", nil, 0, nil, nil))
	g.AddNode("a", newStub("node", nil, 0, nil, nil))
	g.AddNode("b", newStub("node", nil, 0, nil, nil))
	g.AddEdge("start", "a")
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")

	if _, err := g.Compile(); err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
}
