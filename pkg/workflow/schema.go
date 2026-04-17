package workflow

import (
	"encoding/json"
	"fmt"
)

// DecodeInput unmarshals the resolved input map into a typed struct
// via JSON round-tripping. The same json tags that govern the
// definition's schema govern deserialisation, so nodes stay
// fully-typed inside Execute.
func DecodeInput[T any](input map[string]any) (*T, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("decode input: marshal: %w", err)
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("decode input: unmarshal: %w", err)
	}
	return &v, nil
}

// EncodeOutput marshals a typed output struct into the map[string]any
// wire format every Node returns. Nodes use this to keep their
// Execute bodies typed while complying with the Node interface's
// untyped output contract.
func EncodeOutput[T any](v *T) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode output: marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("encode output: unmarshal: %w", err)
	}
	return m, nil
}
