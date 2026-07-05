package agentcore

import "testing"

func TestInputMessageEvents(t *testing.T) {
	// system + user, semconv shape.
	sc := `[{"role":"system","parts":[{"type":"text","content":"be nice"}]},{"role":"user","parts":[{"type":"text","content":"this is a test"}]}]`
	events := InputMessageEvents(sc)
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].EventName != EventSystemMessage {
		t.Errorf("event[0] = %s, want %s", events[0].EventName, EventSystemMessage)
	}
	if events[1].EventName != EventUserMessage {
		t.Errorf("event[1] = %s, want %s", events[1].EventName, EventUserMessage)
	}
	content, _ := events[1].Body["content"].([]map[string]any)
	if len(content) != 1 || content[0]["text"] != "this is a test" {
		t.Errorf("user body content wrong: %#v", events[1].Body)
	}
	if _, hasRole := events[1].Body["role"]; hasRole {
		t.Error("user message body should not carry a role field")
	}
}

func TestOutputMessageEventsChoice(t *testing.T) {
	sc := `[{"role":"assistant","parts":[{"type":"text","content":"Hi!"}],"finish_reason":"end_turn"}]`
	events := OutputMessageEvents(sc)
	if len(events) != 1 || events[0].EventName != EventChoice {
		t.Fatalf("want 1 gen_ai.choice, got %#v", events)
	}
	body := events[0].Body
	if body["index"].(int) != 0 || body["finish_reason"] != "end_turn" {
		t.Errorf("choice envelope wrong: %#v", body)
	}
	msg := body["message"].(map[string]any)
	if msg["role"] != "assistant" {
		t.Errorf("choice message role = %v", msg["role"])
	}
	content := msg["content"].([]map[string]any)
	if content[0]["text"] != "Hi!" {
		t.Errorf("choice content = %#v", content)
	}
}

func TestAssistantToolCallEvent(t *testing.T) {
	sc := `[{"role":"assistant","parts":[{"type":"tool_call","id":"c1","name":"lookup","arguments":{"q":"x"}}]}]`
	events := InputMessageEvents(sc)
	if len(events) != 1 || events[0].EventName != EventAssistantMessage {
		t.Fatalf("want assistant message, got %#v", events)
	}
	tcs := events[0].Body["tool_calls"].([]map[string]any)
	if len(tcs) != 1 || tcs[0]["id"] != "c1" {
		t.Errorf("tool_calls wrong: %#v", tcs)
	}
	fn := tcs[0]["function"].(map[string]any)
	if fn["name"] != "lookup" {
		t.Errorf("function name = %v", fn["name"])
	}
}

func TestToolMessageEvent(t *testing.T) {
	sc := `[{"role":"tool","parts":[{"type":"tool_call_response","id":"c1","response":"42"}]}]`
	events := InputMessageEvents(sc)
	if len(events) != 1 || events[0].EventName != EventToolMessage {
		t.Fatalf("want tool message, got %#v", events)
	}
	if events[0].Body["content"] != "42" || events[0].Body["role"] != "tool" {
		t.Errorf("tool body wrong: %#v", events[0].Body)
	}
}

func TestSystemMessageEventAndDedup(t *testing.T) {
	ev, ok := SystemMessageEvent("You are helpful")
	if !ok || ev.EventName != EventSystemMessage {
		t.Fatal("expected a system message event")
	}
	if _, ok := SystemMessageEvent(""); ok {
		t.Error("empty instructions should be not-ok")
	}
	if !HasSystemMessage([]LogEvent{{EventName: EventSystemMessage}}) {
		t.Error("HasSystemMessage should detect a system event")
	}
	if HasSystemMessage([]LogEvent{{EventName: EventUserMessage}}) {
		t.Error("HasSystemMessage false positive")
	}
}

func TestInferenceDetails(t *testing.T) {
	in := `[{"role":"system","parts":[{"type":"text","content":"be nice"}]},{"role":"user","parts":[{"type":"text","content":"this is a test"}]}]`
	out := `[{"role":"assistant","parts":[{"type":"text","content":"hello"}],"finish_reason":"end_turn"}]`
	body, ok := InferenceDetails(in, out)
	if !ok {
		t.Fatal("expected ok")
	}

	inMsgs := body["input"].(map[string]any)["messages"].([]map[string]any)
	if inMsgs[0]["role"] != "system" || inMsgs[0]["content"] != "be nice" {
		t.Errorf("system message wrong: %#v", inMsgs[0])
	}
	userContent := inMsgs[1]["content"].(map[string]any)["content"].([]map[string]any)
	if userContent[0]["text"] != "this is a test" {
		t.Errorf("user nested content wrong: %#v", inMsgs[1])
	}

	outMsgs := body["output"].(map[string]any)["messages"].([]map[string]any)
	oc := outMsgs[0]["content"].(map[string]any)
	if oc["message"] != "hello" || oc["finish_reason"] != "end_turn" {
		t.Errorf("output content wrong: %#v", outMsgs[0])
	}
}

func TestEmptyMessageEvents(t *testing.T) {
	if InputMessageEvents("") != nil || InputMessageEvents("[]") != nil {
		t.Error("empty input should yield nil events")
	}
	if OutputMessageEvents("not json") != nil {
		t.Error("bad json should yield nil events")
	}
	if _, ok := InferenceDetails("", ""); ok {
		t.Error("empty inference details should be not-ok")
	}
}
