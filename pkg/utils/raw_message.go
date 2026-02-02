package utils

// RawMessage mirrors the behavior of encoding/json.RawMessage while keeping the
// project on the sonic JSON implementation.
type RawMessage []byte

func (m *RawMessage) MarshalJSON() ([]byte, error) {
	return *m, nil
}

func (m *RawMessage) UnmarshalJSON(data []byte) error {
	*m = append((*m)[:0], data...)
	return nil
}
