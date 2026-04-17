package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
)

// NodeExecutor is the single seam between the shared graph walker and
// a specific runtime's execution primitives. The walker prepares each
// wave (dedup, result bookkeeping) and hands the invocations to the
// executor; the executor decides how to run them (goroutines
// in-process, workflow.Future under Temporal, ...).
//
// A NodeExecutor is invoked at most once per wave and must return
// one Result per Invocation in the same order. It should honour the
// passed ctx for cancellation but is otherwise free in how it
// parallelises the batch.
type NodeExecutor interface {
	ExecuteWave(ctx context.Context, invocations []Invocation) []Result
}

// Invocation is one unit of work the walker sends to the executor.
// It is self-contained: everything the executor needs to execute
// a node (or, for a durable runtime, to hand it off as an activity)
// is carried here. Nothing outside Invocation should be consulted by
// the executor.
type Invocation struct {
	// Node is the built, validated Node instance. In-process
	// executors call Node.Execute directly; durable executors
	// typically use Node.Type() as the activity name.
	Node Node
	// NodeID is the identity inside the current graph run.
	NodeID string
	// State is a snapshot of the shared state as of the wave's start.
	// The executor seeds a local RunState with this before calling
	// Node.Execute so the in-process and durable paths see the same
	// view. Nodes read state through rs.State() / rs.Get().
	State map[string]any
}

// Result is the outcome of one node execution, returned by the
// executor to the walker. Output is the node's partial state
// update; the walker shallow-merges it into the shared state.
type Result struct {
	NodeID string
	Output map[string]any
	Port   string
	Err    error
}

// Walker holds the shared graph-execution algorithm. It owns the
// wave loop, result bookkeeping, cycle-free visitation, and step-cap
// enforcement. The only thing it doesn't own is how to run the nodes
// themselves — that's the NodeExecutor.
type Walker struct {
	Logger   *slog.Logger
	MaxSteps int
}

// NewWalker constructs a Walker with RuntimeOptions sensibly applied.
func NewWalker(opts RuntimeOptions) *Walker {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	maxSteps := opts.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultMaxSteps
	}
	return &Walker{Logger: logger, MaxSteps: maxSteps}
}

// Walk executes the compiled graph through ne. It returns a RunState
// populated with the final state and per-node results (the contract
// the Runtime exposes to callers). Walk always returns a non-nil
// RunState, even on error, so observers can inspect partial results.
//
// The invocation input is the initial state. Each node returns a
// partial update which the walker shallow-merges into the shared
// state before dispatching the next wave.
func (w *Walker) Walk(ctx context.Context, c *Compiled, input map[string]any, ne NodeExecutor) (*RunState, error) {
	rs := NewRunState(input)
	steps := 0
	wave := append([]string(nil), c.Roots...)
	visited := make(map[string]bool, len(c.Nodes))

	for len(wave) > 0 {
		unique := dedupUnvisited(wave, visited)
		if len(unique) == 0 {
			break
		}

		if steps+len(unique) > w.MaxSteps {
			return rs, fmt.Errorf("workflow: exceeded max steps (%d)", w.MaxSteps)
		}

		// Dispatch in a stable order; the same wave must always
		// produce the same activity-call sequence when running under a
		// deterministic runtime (Temporal).
		sort.Strings(unique)
		for _, id := range unique {
			visited[id] = true
		}

		state := rs.snapshotState()
		invs := make([]Invocation, len(unique))
		for i, id := range unique {
			node := c.Nodes[id]
			rs.SetNodeResult(id, &NodeResult{NodeID: id, Status: NodeStatusRunning})
			invs[i] = Invocation{
				Node:   node,
				NodeID: id,
				State:  state,
			}
			steps++
			w.Logger.Info("dispatching node", "node_id", id, "type", node.Type())
		}

		results := ne.ExecuteWave(ctx, invs)

		var waveErrs []error
		var nextCandidates []string
		for _, r := range results {
			if r.Err != nil {
				rs.SetNodeResult(r.NodeID, &NodeResult{
					NodeID: r.NodeID,
					Status: NodeStatusFailed,
					Error:  r.Err.Error(),
				})
				w.Logger.Error("node execution failed", "node_id", r.NodeID, "error", r.Err)
				waveErrs = append(waveErrs, fmt.Errorf("node %q: %w", r.NodeID, r.Err))
				continue
			}
			if r.Output != nil {
				rs.MergeState(r.Output)
			}
			rs.SetNodeResult(r.NodeID, &NodeResult{
				NodeID: r.NodeID,
				Status: NodeStatusCompleted,
				Port:   r.Port,
				Output: r.Output,
			})

			// Conditional edges take precedence over static port edges.
			// The router runs against the latest state (which includes
			// this node's merged output) and returns a label; the
			// walker follows that label's target in the cond mapping.
			if cond, ok := c.Conditionals[r.NodeID]; ok {
				label := cond.Router(rs)
				target, mapped := cond.Targets[label]
				if !mapped {
					w.Logger.Info("conditional edge: unmapped label, branch ends",
						"node_id", r.NodeID, "label", label)
					continue
				}
				if target == EndNode {
					continue
				}
				if !visited[target] {
					nextCandidates = append(nextCandidates, target)
				}
				continue
			}

			key := edgeKey(r.NodeID, r.Port)
			for _, edge := range c.OutEdges[key] {
				if edge.ToNode == EndNode {
					continue
				}
				if !visited[edge.ToNode] {
					nextCandidates = append(nextCandidates, edge.ToNode)
				}
			}
		}
		if len(waveErrs) > 0 {
			return rs, errors.Join(waveErrs...)
		}

		wave = nextCandidates
	}

	// Anything not visited didn't fire — mark it skipped so observers
	// see the full per-node outcome.
	ids := make([]string, 0, len(c.Nodes))
	for id := range c.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !visited[id] {
			rs.SetNodeResult(id, &NodeResult{NodeID: id, Status: NodeStatusSkipped})
		}
	}
	return rs, nil
}

func dedupUnvisited(ids []string, visited map[string]bool) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if visited[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
