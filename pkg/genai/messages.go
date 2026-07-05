package genai

import (
	"encoding/json"

	"github.com/bytedance/sonic"
)

// This file reshapes our native OpenAI-shaped message JSON (Responses API and
// Chat Completions API) into the OpenTelemetry GenAI semantic-convention
// structure used by gen_ai.input.messages / gen_ai.output.messages: an array
// of { "role", "parts": [ ... ], "finish_reason"? }.
//
// Why it exists: AWS CloudWatch GenAI Observability (and its evaluations)
// parse exactly this shape. If a destination receives the raw OpenAI wire form
// instead (type:"message" + content:[{type:"input_text",text}], function_call
// items, choices, …) the console shows the attribute in the raw JSON view but
// leaves the Input/Output columns blank and evals have no content to score.
//
// Spans are emitted carrying the semconv shape directly: emission sites call
// InputMessages / OutputMessages (typed) which reshape once, at emission, so
// there is no per-exporter transform. The Reshape* functions take the native
// JSON string and return the semconv JSON string; ok is false when there is
// nothing worth recording.
//
// Message/Part are the public semconv message model — the contract between the
// reshapers here (producers) and destination-specific shapers such as
// pkg/genai/agentcore (consumers, via DecodeMessages).
//
// Reference: https://github.com/open-telemetry/semantic-conventions-genai

// Semconv part types.
const (
	PartText             = "text"
	PartReasoning        = "reasoning"
	PartToolCall         = "tool_call"
	PartToolCallResponse = "tool_call_response"
)

// Message is one semantic-convention message. FinishReason is only set on
// output messages.
type Message struct {
	Role         string `json:"role"`
	Parts        []Part `json:"parts"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// Part is one content part. Only the fields relevant to Type are populated.
type Part struct {
	Type string `json:"type"`
	// text / reasoning
	Content string `json:"content,omitempty"`
	// tool_call / tool_call_response
	ID string `json:"id,omitempty"`
	// tool_call
	Name      string `json:"name,omitempty"`
	Arguments any    `json:"arguments,omitempty"`
	// tool_call_response
	Response any `json:"response,omitempty"`
}

// DecodeMessages parses a semconv messages JSON string (as produced by the
// Reshape* functions) back into the message model. ok is false when the JSON
// is empty or unparseable.
func DecodeMessages(semconvJSON string) ([]Message, bool) {
	if semconvJSON == "" {
		return nil, false
	}
	var msgs []Message
	if err := sonic.Unmarshal([]byte(semconvJSON), &msgs); err != nil || len(msgs) == 0 {
		return nil, false
	}
	return msgs, true
}

// InputMessages / OutputMessages / ChatInputMessages / ChatChoices build the
// semconv gen_ai.input.messages / gen_ai.output.messages value directly from
// the typed message objects at emission time, so spans carry semconv natively
// (no per-exporter transform). They marshal the value to its native OpenAI wire
// JSON and reshape it; our LLM libraries produce OpenAI-shaped types, so that
// one conversion is unavoidable — but it happens once, here.
//
// NOTE: for a single union VALUE whose MarshalJSON has a pointer receiver
// (e.g. responses.InputUnion), pass its address, else the marshal skips
// MarshalJSON and leaks the raw union fields.

// InputMessages builds gen_ai.input.messages from Responses-API input items.
func InputMessages(v any) (string, bool) { return marshalReshapeResponses(v, false) }

// OutputMessages builds gen_ai.output.messages from Responses-API output items.
func OutputMessages(v any) (string, bool) { return marshalReshapeResponses(v, true) }

// ChatInputMessages builds gen_ai.input.messages from Chat-Completions messages.
func ChatInputMessages(v any) (string, bool) {
	b, err := sonic.Marshal(v)
	if err != nil {
		return "", false
	}
	return ReshapeChatMessages(string(b))
}

// ChatChoices builds gen_ai.output.messages from Chat-Completions choices.
func ChatChoices(v any) (string, bool) {
	b, err := sonic.Marshal(v)
	if err != nil {
		return "", false
	}
	return ReshapeChatChoices(string(b))
}

func marshalReshapeResponses(v any, isOutput bool) (string, bool) {
	b, err := sonic.Marshal(v)
	if err != nil {
		return "", false
	}
	return ReshapeResponses(string(b), isOutput)
}

// ReshapeResponses converts native Responses-API item JSON (input or output)
// to the semconv messages JSON. isOutput stamps a best-effort finish_reason.
func ReshapeResponses(nativeJSON string, isOutput bool) (string, bool) {
	var items []json.RawMessage
	if err := sonic.Unmarshal([]byte(nativeJSON), &items); err != nil {
		// The Responses `input` may be a bare string ("input":"hello") rather
		// than an item list; wrap it as a single user text message.
		var s string
		if !isOutput && sonic.Unmarshal([]byte(nativeJSON), &s) == nil && s != "" {
			return marshalMessages([]Message{{Role: "user", Parts: []Part{{Type: PartText, Content: s}}}})
		}
		return "", false
	}
	if len(items) == 0 {
		return "", false
	}
	msgs := make([]Message, 0, len(items))
	for _, it := range items {
		var r rawRespItem
		if err := sonic.Unmarshal(it, &r); err != nil {
			continue
		}
		if m, ok := respItemToMessage(r, isOutput); ok {
			msgs = append(msgs, m)
		}
	}
	return marshalMessages(msgs)
}

// ReshapeChatMessages converts native Chat-Completions request-message JSON
// (an array of role/content messages) to the semconv messages JSON.
func ReshapeChatMessages(nativeJSON string) (string, bool) {
	var msgs []rawChatMsg
	if err := sonic.Unmarshal([]byte(nativeJSON), &msgs); err != nil || len(msgs) == 0 {
		return "", false
	}
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if sm, ok := chatMsgToMessage(m, ""); ok {
			out = append(out, sm)
		}
	}
	return marshalMessages(out)
}

// ReshapeChatChoices converts native Chat-Completions response-choice JSON
// (an array of {finish_reason, message}) to the semconv messages JSON.
func ReshapeChatChoices(nativeJSON string) (string, bool) {
	var choices []struct {
		FinishReason string     `json:"finish_reason"`
		Message      rawChatMsg `json:"message"`
	}
	if err := sonic.Unmarshal([]byte(nativeJSON), &choices); err != nil || len(choices) == 0 {
		return "", false
	}
	out := make([]Message, 0, len(choices))
	for _, c := range choices {
		if sm, ok := chatMsgToMessage(c.Message, c.FinishReason); ok {
			out = append(out, sm)
		}
	}
	return marshalMessages(out)
}

// AssistantText wraps a plain assistant string (e.g. the accumulated text of a
// streamed chat-completion response, which is stored raw rather than as choice
// JSON) as a single semconv output message.
func AssistantText(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	return marshalMessages([]Message{{
		Role:         "assistant",
		Parts:        []Part{{Type: PartText, Content: text}},
		FinishReason: "stop",
	}})
}

// --- Responses API ---

type rawRespItem struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ID        string          `json:"id"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Output    json.RawMessage `json:"output"`
	Summary   []struct {
		Text string `json:"text"`
	} `json:"summary"`
}

func respItemToMessage(r rawRespItem, isOutput bool) (Message, bool) {
	switch r.Type {
	case "function_call":
		id := r.CallID
		if id == "" {
			id = r.ID
		}
		m := Message{Role: "assistant", Parts: []Part{{
			Type: PartToolCall, ID: id, Name: r.Name, Arguments: parseArgs(r.Arguments),
		}}}
		if isOutput {
			m.FinishReason = "tool_calls"
		}
		return m, true

	case "function_call_output":
		return Message{Role: "tool", Parts: []Part{{
			Type: PartToolCallResponse, ID: r.CallID, Response: rawToAny(r.Output),
		}}}, true

	case "reasoning":
		var parts []Part
		for _, s := range r.Summary {
			if s.Text != "" {
				parts = append(parts, Part{Type: PartReasoning, Content: s.Text})
			}
		}
		if len(parts) == 0 {
			return Message{}, false
		}
		return Message{Role: "assistant", Parts: parts}, true

	case "message", "":
		role := r.Role
		if role == "" {
			role = "user"
		}
		parts := textPartsFromContent(r.Content)
		if len(parts) == 0 {
			return Message{}, false
		}
		m := Message{Role: role, Parts: parts}
		if isOutput && role == "assistant" {
			m.FinishReason = "stop"
		}
		return m, true

	default:
		// Unknown item kind (image_generation_call, web_search_call, …).
		// Keep a breadcrumb so nothing is silently dropped.
		return Message{Role: "assistant", Parts: []Part{{
			Type: PartText, Content: "[" + r.Type + "]",
		}}}, true
	}
}

// --- Chat Completions API ---

type rawChatMsg struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Refusal    string          `json:"refusal"`
	ToolCallID string          `json:"tool_call_id"`
	ToolCalls  []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func chatMsgToMessage(m rawChatMsg, finishReason string) (Message, bool) {
	role := m.Role
	if role == "" {
		role = "assistant"
	}

	var parts []Part
	if role == "tool" && m.ToolCallID != "" {
		parts = append(parts, Part{Type: PartToolCallResponse, ID: m.ToolCallID, Response: rawToAny(m.Content)})
	} else {
		parts = append(parts, textPartsFromContent(m.Content)...)
		if m.Refusal != "" {
			parts = append(parts, Part{Type: PartText, Content: m.Refusal})
		}
	}
	for _, tc := range m.ToolCalls {
		parts = append(parts, Part{Type: PartToolCall, ID: tc.ID, Name: tc.Function.Name, Arguments: parseArgs(tc.Function.Arguments)})
	}

	if len(parts) == 0 {
		return Message{}, false
	}
	return Message{Role: role, Parts: parts, FinishReason: finishReason}, true
}

// --- shared helpers ---

// textPartsFromContent turns a message "content" field — which may be a plain
// string or an array of content parts (Responses input_text/output_text or
// Chat text/image parts) — into text parts. Non-text parts (images, files)
// become a short placeholder so their presence is visible without dumping
// binary/URL data.
func textPartsFromContent(raw json.RawMessage) []Part {
	if len(raw) == 0 {
		return nil
	}

	var s string
	if err := sonic.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil
		}
		return []Part{{Type: PartText, Content: s}}
	}

	var arr []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL any    `json:"image_url"`
	}
	if err := sonic.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	var parts []Part
	for _, c := range arr {
		switch {
		case c.Text != "":
			parts = append(parts, Part{Type: PartText, Content: c.Text})
		case c.Type == "input_image" || c.Type == "image_url" || c.ImageURL != nil:
			parts = append(parts, Part{Type: PartText, Content: "[image]"})
		case c.Type == "input_file" || c.Type == "file":
			parts = append(parts, Part{Type: PartText, Content: "[file]"})
		}
	}
	return parts
}

// parseArgs decodes a tool-call arguments string into an object so it renders
// structurally; on failure it falls back to the raw string.
func parseArgs(s string) any {
	if s == "" {
		return nil
	}
	var v any
	if err := sonic.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// rawToAny decodes a raw JSON value (a tool result, which may be a string or a
// structured payload) into a Go value; on failure it falls back to the raw
// bytes as a string.
func rawToAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := sonic.Unmarshal(raw, &v); err == nil {
		return v
	}
	return string(raw)
}

func marshalMessages(msgs []Message) (string, bool) {
	if len(msgs) == 0 {
		return "", false
	}
	b, err := sonic.Marshal(msgs)
	if err != nil {
		return "", false
	}
	return string(b), true
}
