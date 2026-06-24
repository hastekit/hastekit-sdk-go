package agentstate

import (
	"testing"

	"github.com/bytedance/sonic"
)

// TestContextTokensRoundTrip ensures the non-cumulative ContextTokens signal
// (used by the summarizer to gauge current context size) survives the
// ToMeta -> JSON -> LoadRunStateFromMeta persistence round-trip. The JSON hop
// mirrors how meta is actually stored (numbers come back as float64).
func TestContextTokensRoundTrip(t *testing.T) {
	in := NewRunState()
	in.CurrentStep = StepComplete
	in.ContextTokens = 12345

	// Round-trip through JSON the same way meta is persisted/loaded.
	raw, err := sonic.Marshal(in.ToMeta())
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	var meta map[string]any
	if err := sonic.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}

	out := LoadRunStateFromMeta(meta)
	if out == nil {
		t.Fatal("LoadRunStateFromMeta returned nil")
	}
	if out.ContextTokens != 12345 {
		t.Fatalf("ContextTokens not preserved: got %d, want 12345", out.ContextTokens)
	}
}
