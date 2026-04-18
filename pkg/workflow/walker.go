package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
)

// NodeExecutor is the seam between the walker and a specific
// runtime. It must return one Result per Invocation in order and
// honour ctx cancellation.
type NodeExecutor interface {
	ExecuteWave(ctx context.Context, invocations []Invocation) []Result
}

// Invocation is one unit of work the walker sends to the executor.
type Invocation struct {
	Node   Node
	NodeID string
	Input  *Input
}

// Result is the outcome of one node execution. Output is the
// node's partial RunContext update.
type Result struct {
	NodeID string
	Output map[string]any
	Port   string
	Err    error
}

// Walker drives wave-based graph execution: dedup, result merging,
// cycle-free visitation, and step-cap enforcement. The NodeExecutor
// plugs in per-runtime dispatch.
type Walker struct {
	Logger   *slog.Logger
	MaxSteps int
}

// NewWalker constructs a Walker using opts, filling in defaults
// for unset fields.
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

// Walk executes the compiled graph through ne. It always returns
// the non-nil *Input, even on error, so observers can inspect
// partial state. Each Result.Output is deep-merged into
// in.RunContext; Status is recorded per node.
func (w *Walker) Walk(ctx context.Context, c *Compiled, in *Input, ne NodeExecutor) (*Input, error) {
	in = ensureInit(in)

	steps := 0
	wave := append([]string(nil), c.Roots...)
	visited := make(map[string]bool, len(c.Nodes))

	for len(wave) > 0 {
		unique := dedupUnvisited(wave, visited)
		if len(unique) == 0 {
			break
		}
		if steps+len(unique) > w.MaxSteps {
			return in, fmt.Errorf("workflow: exceeded max steps (%d)", w.MaxSteps)
		}

		// Stable dispatch order so deterministic runtimes (Temporal)
		// replay identically.
		sort.Strings(unique)
		for _, id := range unique {
			visited[id] = true
		}

		invs := make([]Invocation, len(unique))
		for i, id := range unique {
			node := c.Nodes[id]
			in.SetStatus(id, NodeStatusRunning)
			invs[i] = Invocation{Node: node, NodeID: id, Input: in}
			steps++
			w.Logger.Info("dispatching node", "node_id", id, "type", node.Type())
		}

		results := ne.ExecuteWave(ctx, invs)

		var waveErrs []error
		var nextCandidates []string
		for _, r := range results {
			if r.Err != nil {
				in.SetStatus(r.NodeID, NodeStatusFailed)
				w.Logger.Error("node execution failed", "node_id", r.NodeID, "error", r.Err)
				waveErrs = append(waveErrs, fmt.Errorf("node %q: %w", r.NodeID, r.Err))
				continue
			}
			in.MergeContext(r.Output)
			in.SetStatus(r.NodeID, NodeStatusCompleted)

			// Conditional edges take precedence over static port
			// edges.
			if cond, ok := c.Conditionals[r.NodeID]; ok {
				label := cond.Router(in)
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
			return in, errors.Join(waveErrs...)
		}

		wave = nextCandidates
	}

	// Mark unvisited nodes as skipped so observers see a complete
	// per-node picture.
	for id := range c.Nodes {
		if !visited[id] {
			in.SetStatus(id, NodeStatusSkipped)
		}
	}
	return in, nil
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
