package workflow

import "strings"

// resolvePathInState walks a dot-separated path against a flat
// state map. The first segment is a top-level key; subsequent
// segments traverse nested maps.
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

// ResolvePath walks a dotted path against in.RunContext. Exposed so
// hosts that want to read per-node output by dotted path get a
// consistent helper; the shape of RunContext is host-defined (e.g.
// the workflow-builder gateway groups entries under "inputs" /
// "outputs" sub-keys).
func ResolvePath(in *Input, path string) (any, bool) {
	if in == nil {
		return nil, false
	}
	return resolvePathInState(path, in.RunContext)
}
