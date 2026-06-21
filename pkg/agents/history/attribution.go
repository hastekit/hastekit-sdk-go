package history

import (
	"strings"

	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/constants"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

// attributeMessages flattens sender-grouped bundles into the flat provider
// message list the LLM consumes, applying multi-participant attribution
// based on each bundle's sender_id and the running agent's id.
func (cm *ConversationRunManager) attributeMessages(msgList []Message, agentID string) []responses.InputMessageUnion {
	out := make([]responses.InputMessageUnion, 0, len(msgList))
	for _, bundle := range msgList {
		own := bundle.SenderID == "" || bundle.SenderID == agentID

		for _, msg := range bundle.Messages {
			switch {
			case own:
				out = append(out, msg)

			case isAssistantMessage(msg):
				out = append(out, userTextMessage("(Agent) "+bundle.SenderID+" said: "+assistantText(msg)))

			case isUserMessage(msg):
				out = append(out, prependUserText(msg, "(Human) "+bundle.SenderID+" said: "))

			default:
				out = append(out, msg)
			}
		}
	}
	return out
}

// isAssistantMessage reports whether msg is an assistant turn — a provider
// output message, or an input/easy message explicitly roled assistant.
func isAssistantMessage(msg responses.InputMessageUnion) bool {
	switch {
	case msg.OfOutputMessage != nil:
		return true
	case msg.OfInputMessage != nil:
		return msg.OfInputMessage.Role == constants.RoleAssistant
	case msg.OfEasyInput != nil:
		return msg.OfEasyInput.Role == constants.RoleAssistant
	}
	return false
}

// isUserMessage reports whether msg is a user turn. An easy message with no
// explicit role defaults to user.
func isUserMessage(msg responses.InputMessageUnion) bool {
	switch {
	case msg.OfInputMessage != nil:
		return msg.OfInputMessage.Role == constants.RoleUser
	case msg.OfEasyInput != nil:
		return msg.OfEasyInput.Role == constants.RoleUser || msg.OfEasyInput.Role == ""
	}
	return false
}

// assistantText concatenates the text segments of an assistant message.
func assistantText(msg responses.InputMessageUnion) string {
	var b strings.Builder
	switch {
	case msg.OfOutputMessage != nil && msg.OfOutputMessage.Content != nil:
		for _, c := range *msg.OfOutputMessage.Content {
			if c.OfOutputText != nil {
				b.WriteString(c.OfOutputText.Text)
			}
		}
	case msg.OfInputMessage != nil:
		writeInputContentText(&b, msg.OfInputMessage.Content)
	case msg.OfEasyInput != nil:
		if msg.OfEasyInput.Content.OfString != nil {
			b.WriteString(*msg.OfEasyInput.Content.OfString)
		} else {
			writeInputContentText(&b, msg.OfEasyInput.Content.OfInputMessageList)
		}
	}
	return b.String()
}

func writeInputContentText(b *strings.Builder, content responses.InputContent) {
	for _, c := range content {
		switch {
		case c.OfInputText != nil:
			b.WriteString(c.OfInputText.Text)
		case c.OfOutputText != nil:
			b.WriteString(c.OfOutputText.Text)
		}
	}
}

// userTextMessage builds a fresh user-role input message carrying text.
func userTextMessage(text string) responses.InputMessageUnion {
	return responses.InputMessageUnion{
		OfInputMessage: &responses.InputMessage{
			Role:    constants.RoleUser,
			Content: responses.InputContent{{OfInputText: &responses.InputTextContent{Text: text}}},
		},
	}
}

// prependUserText returns a copy of a user message with prefix prepended to
// its first text segment (or, when it has no text segment, as a new leading
// text item). The original message and its content are never mutated.
func prependUserText(msg responses.InputMessageUnion, prefix string) responses.InputMessageUnion {
	switch {
	case msg.OfInputMessage != nil:
		im := *msg.OfInputMessage
		im.Content = prefixInputContent(im.Content, prefix)
		msg.OfInputMessage = &im

	case msg.OfEasyInput != nil:
		ei := *msg.OfEasyInput
		if ei.Content.OfString != nil {
			s := prefix + *ei.Content.OfString
			ei.Content = responses.EasyInputContentUnion{OfString: &s}
		} else {
			ei.Content = responses.EasyInputContentUnion{
				OfInputMessageList: prefixInputContent(ei.Content.OfInputMessageList, prefix),
			}
		}
		msg.OfEasyInput = &ei
	}
	return msg
}

// prefixInputContent returns a copy of content with prefix folded into its
// first text item, or prepended as a new leading text item when none exists.
func prefixInputContent(content responses.InputContent, prefix string) responses.InputContent {
	for i, c := range content {
		if c.OfInputText != nil {
			out := make(responses.InputContent, len(content))
			copy(out, content)
			tc := *out[i].OfInputText
			tc.Text = prefix + tc.Text
			out[i].OfInputText = &tc
			return out
		}
	}

	out := make(responses.InputContent, 0, len(content)+1)
	out = append(out, responses.InputContentUnion{OfInputText: &responses.InputTextContent{Text: prefix}})
	return append(out, content...)
}
