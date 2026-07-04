package agui

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// HistoryToMessages flattens stored conversation rows (one per turn,
// each carrying sender-attributed bundles of SDK messages) into the
// flat AG-UI message list clients hydrate a chat from when resuming
// a thread.
//
// Conversions:
//   - user/system/developer input messages → AG-UI user/system/developer
//   - assistant output messages with text   → AG-UI assistant
//   - function calls → toolCalls[] on the preceding assistant message
//     when it has no text (a tool-call-only turn), otherwise a fresh
//     assistant carrier message — mirrors how AG-UI clients nest them
//   - function call outputs → AG-UI role="tool" messages
//   - reasoning, approval responses, and other variants are skipped —
//     hydration is best-effort; the agent's next LLM call loads the
//     authoritative history server-side anyway
func HistoryToMessages(rows []history.ConversationMessage) []Message {
	out := []Message{}

	// uniq returns a non-empty id unique within this message list.
	// CopilotKit keys messages by id, and the SDK can store a
	// function_call and its function_call_output under the same id —
	// emitting both verbatim would collide and the chat would drop one
	// (the tool call/result wouldn't render). Regenerate on collision.
	seen := map[string]bool{}
	uniq := func(id string) string {
		if id == "" || seen[id] {
			id = "m_" + uuid.NewString()
		}
		seen[id] = true
		return id
	}

	for _, row := range rows {
		for _, bundle := range row.Messages {
			for _, msg := range bundle.Messages {
				switch {
				case msg.OfEasyInput != nil:
					m := msg.OfEasyInput
					text := easyInputText(m.Content)
					if text == "" {
						continue
					}
					out = append(out, Message{
						ID:      uniq(m.ID),
						Role:    roleOrUser(string(m.Role)),
						Content: text,
					})

				case msg.OfInputMessage != nil:
					m := msg.OfInputMessage
					text := inputContentText(m.Content)
					if text == "" {
						continue
					}
					out = append(out, Message{
						ID:      uniq(m.ID),
						Role:    roleOrUser(string(m.Role)),
						Content: text,
					})

				case msg.OfOutputMessage != nil:
					m := msg.OfOutputMessage
					text := outputContentText(m.Content)
					if text == "" {
						continue
					}
					out = append(out, Message{
						ID:      uniq(m.ID),
						Role:    RoleAssistant,
						Content: text,
					})

				case msg.OfFunctionCall != nil:
					m := msg.OfFunctionCall
					tc := ToolCall{
						ID:   m.CallID,
						Type: "function",
						Function: ToolCallFunction{
							Name:      m.Name,
							Arguments: m.Arguments,
						},
					}
					// Coalesce onto the previous assistant message when
					// it's a tool-call-only carrier; otherwise emit a
					// fresh one.
					if n := len(out); n > 0 && out[n-1].Role == RoleAssistant && out[n-1].Content == "" {
						out[n-1].ToolCalls = append(out[n-1].ToolCalls, tc)
					} else {
						out = append(out, Message{
							ID:        uniq(m.ID),
							Role:      RoleAssistant,
							ToolCalls: []ToolCall{tc},
						})
					}

				case msg.OfFunctionCallOutput != nil:
					m := msg.OfFunctionCallOutput
					out = append(out, Message{
						ID:         uniq(m.ID),
						Role:       RoleTool,
						ToolCallID: m.CallID,
						Content:    functionOutputText(m.Output),
					})

				case msg.OfImageGenerationCall != nil:
					// A generated image is stored with its base64 result.
					// Surface it as an assistant message carrying a
					// markdown image (data URL) — the same shape the live
					// stream emits — so it renders on history reload.
					m := msg.OfImageGenerationCall
					if m.Result == "" {
						continue
					}
					out = append(out, Message{
						ID:      uniq(m.ID),
						Role:    RoleAssistant,
						Content: imageMarkdown(m.Result, m.OutputFormat),
					})
				}
			}
		}
	}
	return out
}

func roleOrUser(role string) Role {
	switch Role(strings.ToLower(role)) {
	case RoleUser, RoleSystem, RoleDeveloper, RoleAssistant:
		return Role(strings.ToLower(role))
	default:
		return RoleUser
	}
}

func easyInputText(content responses.EasyInputContentUnion) string {
	if content.OfString != nil {
		return *content.OfString
	}
	return inputContentText(content.OfInputMessageList)
}

func inputContentText(content responses.InputContent) string {
	parts := []string{}
	for _, c := range content {
		// File persistence round-trips an assistant OutputMessage into
		// the EasyInput arm, where its text content lands under
		// OfOutputText rather than OfInputText. Read both so assistant
		// turns aren't dropped.
		switch {
		case c.OfInputText != nil && c.OfInputText.Text != "":
			parts = append(parts, c.OfInputText.Text)
		case c.OfOutputText != nil && c.OfOutputText.Text != "":
			parts = append(parts, c.OfOutputText.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func outputContentText(content *responses.OutputContent) string {
	if content == nil {
		return ""
	}
	parts := []string{}
	for _, c := range *content {
		if c.OfOutputText != nil && c.OfOutputText.Text != "" {
			parts = append(parts, c.OfOutputText.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func functionOutputText(output responses.FunctionCallOutputContentUnion) string {
	if output.OfString != nil {
		return *output.OfString
	}
	if output.OfList != nil {
		if b, err := json.Marshal(output.OfList); err == nil {
			return string(b)
		}
	}
	return ""
}
