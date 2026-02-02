package speech

import (
	"fmt"

	"github.com/bytedance/sonic"
)

type StringConstant interface {
	Value() string
}

func unmarshalConstantString(c StringConstant, buf []byte) error {
	var s string
	if err := sonic.Unmarshal(buf, &s); err != nil {
		return err
	}

	if s != c.Value() {
		return fmt.Errorf("invalid %T: got %q, want %q", c, s, c.Value())
	}

	return nil
}

type ChunkTypeAudioDelta string

func (m *ChunkTypeAudioDelta) Value() string                { return "speech.audio.delta" }
func (m *ChunkTypeAudioDelta) MarshalJSON() ([]byte, error) { return sonic.Marshal(m.Value()) }
func (m *ChunkTypeAudioDelta) UnmarshalJSON(buf []byte) error {
	return unmarshalConstantString(m, buf)
}

type ChunkTypeAudioDone string

func (m *ChunkTypeAudioDone) Value() string                { return "speech.audio.done" }
func (m *ChunkTypeAudioDone) MarshalJSON() ([]byte, error) { return sonic.Marshal(m.Value()) }
func (m *ChunkTypeAudioDone) UnmarshalJSON(buf []byte) error {
	return unmarshalConstantString(m, buf)
}
