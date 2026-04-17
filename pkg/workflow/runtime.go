package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Runtime executes a compiled workflow. Implementations decide how
// node execution is scheduled, whether it persists across restarts,
// and how parallelism is expressed. Concretely, a Runtime today is a
// thin shell over the shared Walker — it plugs in its own
// NodeExecutor and lets the walker drive the graph. pkg/workflow
// ships one Runtime (InProcessRuntime); Temporal-, Restate-, or any
// other backing is supplied by the host in a sibling package.
//
// A Runtime MUST return the RunState even on error so callers can
// inspect partial results.
type Runtime interface {
	Execute(ctx context.Context, c *Compiled, input map[string]any, opts RuntimeOptions) (*RunState, error)
}

// RuntimeOptions carries run-level configuration into a Runtime
// call. Callers typically set it through InvokeOption helpers
// (WithLogger, WithMaxSteps) rather than constructing it directly;
// a zero value is valid and picks the runtime's own defaults.
type RuntimeOptions struct {
	Logger   *slog.Logger
	MaxSteps int
}

// invokeConfig is the full, resolved configuration of a single
// Invoke call: which Runtime to dispatch through, plus the options
// that Runtime receives. Callers never construct this directly; they
// compose InvokeOption values.
type invokeConfig struct {
	runtime     Runtime
	runtimeOpts RuntimeOptions
}

// InvokeOption configures a single Invoke call. Pass the result of
// WithRuntime / WithLogger / WithMaxSteps (or custom options) to
// Compiled.Invoke or Graph.Invoke.
type InvokeOption func(*invokeConfig)

// WithRuntime selects the Runtime used for the invocation. If no
// WithRuntime is supplied, Invoke defaults to InProcessRuntime.
func WithRuntime(rt Runtime) InvokeOption {
	return func(c *invokeConfig) { c.runtime = rt }
}

// WithLogger overrides the default slog logger used during the run.
func WithLogger(l *slog.Logger) InvokeOption {
	return func(c *invokeConfig) { c.runtimeOpts.Logger = l }
}

// WithMaxSteps caps how many node executions a single run may
// perform. The step count is checked before each wave, so the actual
// ceiling is (current count + next wave size) ≤ n.
func WithMaxSteps(n int) InvokeOption {
	return func(c *invokeConfig) { c.runtimeOpts.MaxSteps = n }
}

// ConditionalEdge attaches a runtime router to a node. When the node
// completes, Router is called with the latest RunState and must
// return one of the keys in Targets; the walker then dispatches the
// mapped node as the next step. Returning a key not in Targets — or a
// key mapped to EndNode — ends the branch.
type ConditionalEdge struct {
	Router  Router
	Targets map[string]string
}

// Compiled is a validated, prepared graph ready for execution. It
// captures everything a Runtime (and thus the Walker) needs: the
// Node instances, the outgoing-edge index keyed by "nodeID:port",
// the set of conditional edges keyed by source node id, and the list
// of root node ids (the walker's starting wave).
//
// Compiled is read-only to Runtimes; mutable execution state lives
// on RunState.
type Compiled struct {
	Nodes        map[string]Node
	OutEdges     map[string][]Edge
	Conditionals map[string]ConditionalEdge
	Roots        []string
}

// Invoke executes the compiled graph with input as the initial
// state. Any node can read the invocation input (and any updates
// merged in by earlier nodes) through the RunState passed to
// Node.Execute.
//
// The Runtime is supplied through WithRuntime; if omitted, Invoke
// defaults to InProcessRuntime. Use WithLogger / WithMaxSteps to
// tune observability and the step cap.
func (c *Compiled) Invoke(ctx context.Context, input map[string]any, opts ...InvokeOption) (*RunState, error) {
	cfg := invokeConfig{runtime: InProcessRuntime{}}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.runtime == nil {
		return nil, fmt.Errorf("workflow: Invoke given a nil Runtime via WithRuntime")
	}
	return cfg.runtime.Execute(ctx, c, input, cfg.runtimeOpts)
}

// InProcessRuntime runs graphs through the shared Walker using a
// goroutine-per-node executor. Each wave's nodes execute in
// parallel; the Walker owns ordering, routing, and cycle detection.
type InProcessRuntime struct{}

const defaultMaxSteps = 500

// Execute implements Runtime by delegating to the shared Walker.
// The only thing InProcessRuntime contributes beyond the walker is
// the NodeExecutor — which runs each node in a goroutine and calls
// Node.Execute directly.
func (InProcessRuntime) Execute(ctx context.Context, c *Compiled, input map[string]any, opts RuntimeOptions) (*RunState, error) {
	w := NewWalker(opts)
	return w.Walk(ctx, c, input, inProcessExecutor{logger: w.Logger})
}

// inProcessExecutor runs each invocation in its own goroutine and
// collects the results. The first failing node cancels peers via
// the shared wave context.
type inProcessExecutor struct {
	logger *slog.Logger
}

func (e inProcessExecutor) ExecuteWave(ctx context.Context, invs []Invocation) []Result {
	results := make([]Result, len(invs))
	waveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(invs))
	for i, inv := range invs {
		go func(i int, inv Invocation) {
			defer wg.Done()
			results[i] = runInvocation(waveCtx, inv, cancel)
		}(i, inv)
	}
	wg.Wait()
	return results
}

// runInvocation builds a per-node RunState (seeded with the shared
// state snapshot so nodes can read state via rs.State() / rs.Get()),
// calls Node.Execute, and returns the node's partial state update.
//
// The per-node RunState is intentional: it keeps the in-process
// executor byte-for-byte compatible with a durable executor whose
// activities only see a state snapshot. A node whose Execute works
// under InProcessRuntime also works under any Runtime that follows
// this contract.
func runInvocation(ctx context.Context, inv Invocation, onFail context.CancelFunc) Result {
	local := NewRunState(inv.State)

	output, port, err := inv.Node.Execute(ctx, local)
	if err != nil {
		onFail()
		return Result{NodeID: inv.NodeID, Err: fmt.Errorf("node %q failed: %w", inv.NodeID, err)}
	}
	return Result{NodeID: inv.NodeID, Output: output, Port: port}
}
