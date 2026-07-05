package agents

// DurableStep runs a side effect exactly once per logical occurrence, even
// when the agent loop replays under a durable runtime.
//
// The agent loop emits fire-and-forget side effects at run-lifecycle
// boundaries — publishing run.created / run.completed / run.paused and tool
// output chunks to the stream broker. Under a durable runtime (Temporal,
// Restate) the loop re-executes from the top on every replay, so without
// protection each of these would be resent many times. Wrapping them in a
// DurableStep collapses them to a single occurrence:
//
//   - Local: runs fn immediately (no replay, nothing to guard).
//   - Temporal: runs fn only at the live edge via workflow.IsReplaying — the
//     canonical Temporal pattern for once-only external actions like logging.
//   - Restate: runs fn inside restate.RunVoid, a journaled step skipped on replay.
//
// fn must not perform durable-runtime commands (schedule activities, await
// futures, etc.) — it is for external, best-effort side effects only. Steps
// are correlated by execution order, which the agent loop keeps deterministic.
type DurableStep interface {
	Do(fn func())
}

// localDurableStep runs every step immediately. It is the default, used by the
// in-process runtime where there is no replay.
type localDurableStep struct{}

func (localDurableStep) Do(fn func()) { fn() }
