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
	executed *atomic.Int64
	order    *int64
}

func (n *stubNode) Validate() error { return nil }

func (n *stubNode) Execute(_ context.Context, _ *Input) (map[string]any, string, error) {
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
	out, err := g.Invoke(context.Background(), nil)
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
		if out.Status[id] != NodeStatusCompleted {
			t.Errorf("node %q status = %q, want completed", id, out.Status[id])
		}
	}
	// Last-writer wins on flat shallow-mapped `value` key.
	if out.RunContext["value"] != "end" {
		t.Errorf("RunContext[value] = %v, want end", out.RunContext["value"])
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

	out, err := g.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range []string{"start", "a", "b", "c"} {
		if out.Status[id] != NodeStatusCompleted {
			t.Errorf("status[%s] = %q, want completed", id, out.Status[id])
		}
	}
}

// TestDeepMergeNestedUpdates proves MergeContext unions nested maps
// across successive node outputs rather than clobbering siblings —
// the behaviour gateway conventions like "inputs"/"outputs" per
// nodeID rely on.
func TestDeepMergeNestedUpdates(t *testing.T) {
	g := NewGraph("test-deepmerge")
	g.AddNode("a", newStub("node", map[string]any{"bucket": map[string]any{"a": 1}}, 0, nil, nil))
	g.AddNode("b", newStub("node", map[string]any{"bucket": map[string]any{"b": 2}}, 0, nil, nil))
	g.AddEdge("START", "a")
	g.AddEdge("a", "b")
	g.AddEdge("b", "END")

	out, err := g.Invoke(context.Background(), nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	bucket, _ := out.RunContext["bucket"].(map[string]any)
	if bucket["a"] != 1 || bucket["b"] != 2 {
		t.Fatalf("expected deep-merged bucket {a:1,b:2}, got %#v", bucket)
	}
}

func TestInvokeWithStartEnd(t *testing.T) {
	g := NewGraph("test-invoke")
	g.AddNode("a", newStub("node", map[string]any{"ran": true}, 0, nil, nil))
	g.AddEdge("START", "a")
	g.AddEdge("a", "END")

	compiled, err := g.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	out, err := compiled.Execute(context.Background(), &Input{
		Metadata: map[string]any{"greeting": "hello"},
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if out.Metadata["greeting"] != "hello" {
		t.Errorf("metadata[greeting] = %v", out.Metadata["greeting"])
	}
	if out.Status["a"] != NodeStatusCompleted {
		t.Errorf("a status = %v", out.Status["a"])
	}
	if out.RunContext["ran"] != true {
		t.Errorf("RunContext[ran] = %v", out.RunContext["ran"])
	}
}

func TestConditionalEdge(t *testing.T) {
	g := NewGraph("test-conditional")
	g.AddNode("classifier", newStub("classifier", map[string]any{"sentiment": "positive"}, 0, nil, nil))
	g.AddNode("celebrate", newStub("celebrate", map[string]any{"msg": "yay"}, 0, nil, nil))
	g.AddNode("escalate", newStub("escalate", map[string]any{"msg": "oh no"}, 0, nil, nil))
	g.AddEdge("START", "classifier")
	g.AddConditionalEdge("classifier", func(in *Input) string {
		if s, _ := in.RunContext["sentiment"].(string); s == "positive" {
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

	out, err := compiled.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if out.Status["celebrate"] != NodeStatusCompleted {
		t.Errorf("celebrate status = %v", out.Status["celebrate"])
	}
	if out.Status["escalate"] != NodeStatusSkipped {
		t.Errorf("escalate status = %v, want skipped", out.Status["escalate"])
	}
	if out.RunContext["msg"] != "yay" {
		t.Errorf("RunContext[msg] = %v, want yay", out.RunContext["msg"])
	}
}

func TestConditionalEdgeRejectsConflictWithStaticEdge(t *testing.T) {
	g := NewGraph("test-conflict")
	g.AddNode("a", newStub("a", nil, 0, nil, nil))
	g.AddNode("b", newStub("b", nil, 0, nil, nil))
	g.AddEdge("START", "a")
	g.AddEdge("a", "b")
	g.AddConditionalEdge("a", func(*Input) string { return "b" }, map[string]string{"b": "b"})

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
