package genai

import (
	"testing"

	"github.com/bytedance/sonic"
)

// decode is a small helper to inspect the produced semconv JSON.
func decode(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := sonic.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("produced invalid JSON: %v\n%s", err, s)
	}
	return out
}

// TestResponsesInput mirrors the exact wire shape seen on the invoke_agent
// span: type:"message" with a nested input_text content part.
func TestResponsesInput(t *testing.T) {
	raw := `[{"type":"message","id":"msg_1","role":"user","content":[{"type":"input_text","text":"hey"}]}]`

	s, ok := ReshapeResponses(raw, false)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d: %s", len(msgs), s)
	}
	if msgs[0]["role"] != "user" {
		t.Errorf("role = %v, want user", msgs[0]["role"])
	}
	parts, _ := msgs[0]["parts"].([]any)
	if len(parts) != 1 {
		t.Fatalf("want 1 part, got %v", msgs[0]["parts"])
	}
	p := parts[0].(map[string]any)
	if p["type"] != "text" || p["content"] != "hey" {
		t.Errorf("part = %v, want text/hey", p)
	}
	if _, hasFinish := msgs[0]["finish_reason"]; hasFinish {
		t.Errorf("input message must not carry finish_reason: %s", s)
	}
}

// TestResponsesOutputToolCall mirrors the function_call output item, which
// must become an assistant tool_call part with parsed arguments + a
// finish_reason.
func TestResponsesOutputToolCall(t *testing.T) {
	raw := `[{"type":"function_call","id":"fc_1","call_id":"call_abc","name":"messenger-agent","arguments":"{\"thread_id\":\"\",\"message\":\"hey\"}"}]`

	s, ok := ReshapeResponses(raw, true)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	if msgs[0]["role"] != "assistant" {
		t.Errorf("role = %v, want assistant", msgs[0]["role"])
	}
	if msgs[0]["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls", msgs[0]["finish_reason"])
	}
	p := msgs[0]["parts"].([]any)[0].(map[string]any)
	if p["type"] != "tool_call" || p["name"] != "messenger-agent" || p["id"] != "call_abc" {
		t.Errorf("part = %v", p)
	}
	args, ok := p["arguments"].(map[string]any)
	if !ok || args["message"] != "hey" {
		t.Errorf("arguments not parsed to object: %v", p["arguments"])
	}
}

// TestResponsesOutputText covers an assistant text output message.
func TestResponsesOutputText(t *testing.T) {
	raw := `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello there","annotations":[]}]}]`
	s, ok := ReshapeResponses(raw, true)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	if msgs[0]["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", msgs[0]["finish_reason"])
	}
	p := msgs[0]["parts"].([]any)[0].(map[string]any)
	if p["content"] != "hello there" {
		t.Errorf("content = %v", p["content"])
	}
}

// TestResponsesInputBareString covers Responses `input` given as a bare string.
func TestResponsesInputBareString(t *testing.T) {
	s, ok := ReshapeResponses(`"just a string"`, false)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	p := msgs[0]["parts"].([]any)[0].(map[string]any)
	if msgs[0]["role"] != "user" || p["content"] != "just a string" {
		t.Errorf("bare string not wrapped as user message: %s", s)
	}
	// A bare string on the OUTPUT side is not meaningful → not-ok.
	if _, ok := ReshapeResponses(`"x"`, true); ok {
		t.Error("bare string output should be not-ok")
	}
}

// TestChatInput covers a plain-string chat message and a multi-part one.
func TestChatInput(t *testing.T) {
	raw := `[{"role":"system","content":"be nice"},{"role":"user","content":[{"type":"text","text":"hi"}]}]`
	s, ok := ReshapeChatMessages(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	if len(msgs) != 2 || msgs[0]["role"] != "system" || msgs[1]["role"] != "user" {
		t.Fatalf("unexpected: %s", s)
	}
	if msgs[0]["parts"].([]any)[0].(map[string]any)["content"] != "be nice" {
		t.Errorf("system content wrong: %s", s)
	}
}

// TestChatChoices covers an assistant choice carrying a tool call.
func TestChatChoices(t *testing.T) {
	raw := `[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]}}]`
	s, ok := ReshapeChatChoices(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	if msgs[0]["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v", msgs[0]["finish_reason"])
	}
	p := msgs[0]["parts"].([]any)[0].(map[string]any)
	if p["type"] != "tool_call" || p["name"] != "lookup" {
		t.Errorf("part = %v", p)
	}
}

// TestChatToolResponse covers a tool-role message becoming a tool_call_response.
func TestChatToolResponse(t *testing.T) {
	raw := `[{"role":"tool","tool_call_id":"call_1","content":"42"}]`
	s, ok := ReshapeChatMessages(raw)
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	p := msgs[0]["parts"].([]any)[0].(map[string]any)
	if msgs[0]["role"] != "tool" || p["type"] != "tool_call_response" || p["id"] != "call_1" {
		t.Errorf("msg = %v", msgs[0])
	}
}

func TestAssistantText(t *testing.T) {
	s, ok := AssistantText("streamed answer")
	if !ok {
		t.Fatal("expected ok")
	}
	msgs := decode(t, s)
	if msgs[0]["role"] != "assistant" || msgs[0]["finish_reason"] != "stop" {
		t.Errorf("msg = %v", msgs[0])
	}
}

func TestEmptyInputs(t *testing.T) {
	if _, ok := ReshapeResponses("[]", false); ok {
		t.Error("empty array should be not-ok")
	}
	if _, ok := AssistantText(""); ok {
		t.Error("empty text should be not-ok")
	}
	if _, ok := ReshapeChatChoices(""); ok {
		t.Error("empty string should be not-ok")
	}
	if _, ok := ReshapeResponses("not json", false); ok {
		t.Error("invalid JSON should be not-ok")
	}
}
