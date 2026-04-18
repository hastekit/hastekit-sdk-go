package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DefaultPort is the conventional name for a node's primary output
// port. AddEdge uses it; branching nodes (if_else, switch_case,
// api_call error path, …) emit other port names.
const DefaultPort = "default"

// StartNode and EndNode are reserved, implicit node IDs that bookend
// a workflow. Neither needs to be registered via AddNode:
//
//   - StartNode is the entry sentinel. AddEdge("START", "first") marks
//     "first" as a root. Invocation input is seeded as the initial
//     state (not under any namespace), so nodes read it directly.
//   - EndNode is the terminal sentinel. AddEdge("last", "END") marks
//     a branch as finished; the walker records END edges but never
//     schedules END for execution.
const (
	StartNode = "START"
	EndNode   = "END"
)

// Edge connects two nodes. The FromPort field determines which output
// of the source node activates this edge (e.g. "true"/"false" for
// if-else, DefaultPort for linear flow, or a case label for switch).
type Edge struct {
	FromNode string
	FromPort string
	ToNode   string
}

func edgeKey(nodeID, port string) string { return nodeID + ":" + port }

// Graph is a fluent builder for workflow graphs — the langgraph-style
// programmatic API.
//
//	g := workflow.NewGraph("hello")
//	g.AddNode("greet", newGreetNode())
//	g.AddEdge("START", "greet")
//	g.AddEdge("greet", "END")
//
//	rs, err := g.Execute(ctx, input)
//
// Every mutating method returns the graph for chaining. Builder-time
// errors (empty id, nil node, duplicate id) are collected rather than
// panicked; Compile surfaces them as a joined error so a long chain
// never blows up mid-construction.
//
// Identity belongs to the graph, not the node — a Node describes
// behaviour, the graph assigns it an id. There is no declarative
// input wiring: nodes read whatever they need directly from the
// shared state via RunState.
type Graph struct {
	id           string
	nodes        map[string]Node
	edges        []Edge
	conditionals map[string]ConditionalEdge

	buildErrs []error
}

// Router inspects the current RunState after a node completes and
// returns a label that the graph's ConditionalEdge mapping resolves
// into the next node. Returning a label not in the mapping (or one
// mapped to EndNode) ends that branch.
type Router func(rs *RunState) string

// NewGraph constructs an empty Graph. The id is carried through into
// Compiled and used in error messages.
func NewGraph(id string) *Graph {
	return &Graph{
		id:           id,
		nodes:        make(map[string]Node),
		conditionals: make(map[string]ConditionalEdge),
	}
}

// ID returns the graph's identifier.
func (g *Graph) ID() string { return g.id }

// AddNode installs node under id. id must be non-empty and unique
// within the graph; issues are recorded and surfaced by Compile.
func (g *Graph) AddNode(id string, node Node) *Graph {
	if id == "" {
		g.buildErrs = append(g.buildErrs, errors.New("AddNode: id is required"))
		return g
	}
	if node == nil {
		g.buildErrs = append(g.buildErrs, fmt.Errorf("AddNode %q: node is nil", id))
		return g
	}
	if _, exists := g.nodes[id]; exists {
		g.buildErrs = append(g.buildErrs, fmt.Errorf("AddNode: node %q already added", id))
		return g
	}
	g.nodes[id] = node
	return g
}

// AddEdge wires src → dst on src's DefaultPort. Use AddEdgeOnPort for
// branching nodes.
func (g *Graph) AddEdge(src, dst string) *Graph {
	return g.AddEdgeOnPort(src, DefaultPort, dst)
}

// AddEdgeOnPort wires src's named output port to dst.
func (g *Graph) AddEdgeOnPort(src, port, dst string) *Graph {
	g.edges = append(g.edges, Edge{FromNode: src, FromPort: port, ToNode: dst})
	return g
}

// Invoke compiles this graph and executes it with input as the
// initial state. The Runtime defaults to InProcessRuntime; override
// with WithRuntime. It is a convenience wrapper: when the caller
// plans to run the same graph many times, compile once with
// g.Compile() and call Compiled.Execute repeatedly instead.
func (g *Graph) Invoke(ctx context.Context, input map[string]any, opts ...InvokeOption) (*RunState, error) {
	c, err := g.Compile()
	if err != nil {
		return nil, err
	}
	return c.Execute(ctx, input, opts...)
}

// AddConditionalEdge registers a runtime router on src. When src
// completes, router is called with the current RunState and must
// return a key in targets; the walker dispatches targets[key] as the
// next node (or ends the branch if the returned key is not in the
// map, or is mapped to EndNode).
//
// A node may have either static edges (AddEdge / AddEdgeOnPort) or a
// conditional edge — not both. When a conditional edge is present,
// the router runs instead of the node's returned port being consulted
// for routing. All entries in targets must be known node ids (or
// EndNode); targets are included in cycle detection.
func (g *Graph) AddConditionalEdge(src string, router Router, targets map[string]string) *Graph {
	if src == "" {
		g.buildErrs = append(g.buildErrs, errors.New("AddConditionalEdge: src is required"))
		return g
	}
	if router == nil {
		g.buildErrs = append(g.buildErrs, fmt.Errorf("AddConditionalEdge %q: router is nil", src))
		return g
	}
	if len(targets) == 0 {
		g.buildErrs = append(g.buildErrs, fmt.Errorf("AddConditionalEdge %q: targets is empty", src))
		return g
	}
	if _, exists := g.conditionals[src]; exists {
		g.buildErrs = append(g.buildErrs, fmt.Errorf("AddConditionalEdge: node %q already has a conditional edge", src))
		return g
	}
	copied := make(map[string]string, len(targets))
	for k, v := range targets {
		copied[k] = v
	}
	g.conditionals[src] = ConditionalEdge{Router: router, Targets: copied}
	return g
}

// Compile validates the graph and produces a runnable *Compiled. It
// surfaces every accumulated build-time error plus:
//
//   - empty graph id,
//   - nodes whose own Validate() fails,
//   - edges referencing unknown nodes,
//   - cycles.
func (g *Graph) Compile() (*Compiled, error) {
	var errs []error
	errs = append(errs, g.buildErrs...)

	if g.id == "" {
		errs = append(errs, errors.New("workflow: graph id is required"))
	}
	if len(g.nodes) == 0 {
		errs = append(errs, fmt.Errorf("workflow %s: graph has no nodes", g.id))
	}

	for id, n := range g.nodes {
		if err := n.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("workflow %s: node %q validation: %w", g.id, id, err))
		}
	}

	for _, e := range g.edges {
		if e.FromNode != StartNode {
			if _, ok := g.nodes[e.FromNode]; !ok {
				errs = append(errs, fmt.Errorf("workflow %s: edge %s->%s references unknown from_node %q", g.id, e.FromNode, e.ToNode, e.FromNode))
			}
		}
		if e.ToNode != EndNode {
			if _, ok := g.nodes[e.ToNode]; !ok {
				errs = append(errs, fmt.Errorf("workflow %s: edge %s->%s references unknown to_node %q", g.id, e.FromNode, e.ToNode, e.ToNode))
			}
		}
	}
	if _, clash := g.nodes[StartNode]; clash {
		errs = append(errs, fmt.Errorf("workflow %s: %q is a reserved node id", g.id, StartNode))
	}
	if _, clash := g.nodes[EndNode]; clash {
		errs = append(errs, fmt.Errorf("workflow %s: %q is a reserved node id", g.id, EndNode))
	}

	// Conditional edge validation: src must be a known node, each
	// target must be a known node or EndNode, and src cannot also have
	// static outgoing edges (the two routing mechanisms are mutually
	// exclusive).
	staticSources := make(map[string]bool, len(g.edges))
	for _, e := range g.edges {
		staticSources[e.FromNode] = true
	}
	for src, cond := range g.conditionals {
		if _, ok := g.nodes[src]; !ok {
			errs = append(errs, fmt.Errorf("workflow %s: conditional edge on unknown source %q", g.id, src))
		}
		if staticSources[src] {
			errs = append(errs, fmt.Errorf("workflow %s: node %q has both static and conditional edges", g.id, src))
		}
		for label, target := range cond.Targets {
			if target == EndNode {
				continue
			}
			if target == StartNode {
				errs = append(errs, fmt.Errorf("workflow %s: conditional edge %q[%q] cannot target START", g.id, src, label))
				continue
			}
			if _, ok := g.nodes[target]; !ok {
				errs = append(errs, fmt.Errorf("workflow %s: conditional edge %q[%q] references unknown target %q", g.id, src, label, target))
			}
		}
	}

	// Roots = targets of StartNode edges, if any. Otherwise fall back
	// to nodes with no incoming edges. Cycle detection and the walker
	// both use this entry set.
	var roots []string
	startTargets := make(map[string]bool)
	for _, e := range g.edges {
		if e.FromNode == StartNode {
			if _, ok := g.nodes[e.ToNode]; ok && !startTargets[e.ToNode] {
				startTargets[e.ToNode] = true
				roots = append(roots, e.ToNode)
			}
		}
	}
	if len(roots) == 0 {
		hasIncoming := make(map[string]bool, len(g.nodes))
		for _, e := range g.edges {
			if e.FromNode == StartNode {
				continue
			}
			if _, ok := g.nodes[e.ToNode]; ok {
				hasIncoming[e.ToNode] = true
			}
		}
		for _, cond := range g.conditionals {
			for _, target := range cond.Targets {
				if target == EndNode {
					continue
				}
				if _, ok := g.nodes[target]; ok {
					hasIncoming[target] = true
				}
			}
		}
		for id := range g.nodes {
			if !hasIncoming[id] {
				roots = append(roots, id)
			}
		}
	}

	// Skip cycle detection if edges reference unknown nodes — the
	// earlier error is the real problem.
	if len(errs) == 0 {
		nodeIDs := make([]string, 0, len(g.nodes))
		for id := range g.nodes {
			nodeIDs = append(nodeIDs, id)
		}
		if err := detectCycle(g.id, nodeIDs, g.edges, g.conditionals, roots); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	outEdges := make(map[string][]Edge, len(g.edges))
	for _, e := range g.edges {
		k := edgeKey(e.FromNode, e.FromPort)
		outEdges[k] = append(outEdges[k], e)
	}

	var conditionals map[string]ConditionalEdge
	if len(g.conditionals) > 0 {
		conditionals = make(map[string]ConditionalEdge, len(g.conditionals))
		for k, v := range g.conditionals {
			conditionals[k] = v
		}
	}

	return &Compiled{
		Nodes:        g.nodes,
		OutEdges:     outEdges,
		Conditionals: conditionals,
		Roots:        roots,
	}, nil
}

// detectCycle runs a DFS from each root (and then any remaining
// unvisited nodes) and returns an error if any node is reachable
// from itself. A cycle through any port is still a cycle; conditional
// edges contribute all declared targets to the adjacency list.
func detectCycle(graphID string, nodeIDs []string, edges []Edge, conditionals map[string]ConditionalEdge, roots []string) error {
	adj := make(map[string][]string, len(nodeIDs))
	for _, e := range edges {
		adj[e.FromNode] = append(adj[e.FromNode], e.ToNode)
	}
	for src, cond := range conditionals {
		for _, target := range cond.Targets {
			adj[src] = append(adj[src], target)
		}
	}

	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(nodeIDs))
	stack := make([]string, 0, len(nodeIDs))

	var visit func(id string) error
	visit = func(id string) error {
		switch state[id] {
		case onStack:
			start := 0
			for i, s := range stack {
				if s == id {
					start = i
					break
				}
			}
			loop := append([]string{}, stack[start:]...)
			loop = append(loop, id)
			return fmt.Errorf("workflow %s: cycle detected: %s", graphID, strings.Join(loop, " → "))
		case done:
			return nil
		}
		state[id] = onStack
		stack = append(stack, id)
		for _, next := range adj[id] {
			if err := visit(next); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return nil
	}

	for _, r := range roots {
		if err := visit(r); err != nil {
			return err
		}
	}
	for _, id := range nodeIDs {
		if state[id] == unvisited {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}
