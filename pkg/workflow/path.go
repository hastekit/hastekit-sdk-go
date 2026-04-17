package workflow

import "strings"

// resolvePath walks a dot-separated path like "user.name.first"
// against the RunState's flat state map. The first segment is a
// top-level state key; subsequent segments traverse nested maps.
//
// Path resolution is the engine's one concession to "data plane"
// plumbing — nodes can use it (or the exported ResolvePath) to pick
// a value out of state by dotted path instead of walking maps by
// hand. It's deliberately cheap and string-typed; richer expression
// languages are a host concern.
func resolvePath(rs *RunState, path string) (any, bool) {
	return resolvePathInState(path, rs.snapshotState())
}

// resolvePathInState is the shape resolvePath takes when the walker
// is working with a plain state snapshot (no RunState). Durable
// runtimes share the same resolver by going through this entry point.
func resolvePathInState(path string, state map[string]any) (any, bool) {
	if path == "" || state == nil {
		return nil, false
	}
	parts := strings.SplitN(path, ".", 2)
	v, ok := state[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return v, true
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return traversePath(m, parts[1])
}

func traversePath(data map[string]any, path string) (any, bool) {
	current := any(data)
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		val, ok := m[part]
		if !ok {
			return nil, false
		}
		current = val
	}
	return current, true
}

// ResolvePath is an exported wrapper over path resolution, handed to
// hosts that want to read state by dotted path without reaching into
// RunState internals.
func ResolvePath(rs *RunState, path string) (any, bool) {
	return resolvePath(rs, path)
}
