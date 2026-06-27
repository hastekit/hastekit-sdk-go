package agui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	assert.Error(t, (&RunAgentInput{}).Validate())
	assert.Error(t, (&RunAgentInput{ThreadID: "t"}).Validate())
	assert.Error(t, (&RunAgentInput{ThreadID: "t", Messages: []Message{{Content: "no role"}}}).Validate())

	assert.NoError(t, (&RunAgentInput{
		ThreadID: "t",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}).Validate())

	// Approval-only POST (the HITL resume shape) is valid without messages.
	assert.NoError(t, (&RunAgentInput{
		ThreadID: "t",
		ForwardedProps: map[string]any{
			"command": map[string]any{
				"resume": map[string]any{
					"decisions": []any{map[string]any{"toolCallId": "call_1", "approved": true}},
				},
			},
		},
	}).Validate())
}

func TestExtractApprovals(t *testing.T) {
	canonical := &RunAgentInput{ForwardedProps: map[string]any{
		"command": map[string]any{
			"resume": map[string]any{
				"decisions": []any{
					map[string]any{"toolCallId": "call_1", "approved": true},
					map[string]any{"toolCallId": "call_2", "approved": false},
					map[string]any{"approved": true}, // missing id — dropped
				},
			},
		},
	}}
	decisions := canonical.ExtractApprovals()
	require.Len(t, decisions, 2)
	assert.Equal(t, ApprovalDecision{ToolCallID: "call_1", Approved: true}, decisions[0])
	assert.Equal(t, ApprovalDecision{ToolCallID: "call_2", Approved: false}, decisions[1])

	alias := &RunAgentInput{ForwardedProps: map[string]any{
		"hastekitApprovals": []any{map[string]any{"toolCallId": "call_3", "approved": true}},
	}}
	require.Len(t, alias.ExtractApprovals(), 1)

	assert.Empty(t, (&RunAgentInput{}).ExtractApprovals())
	assert.Empty(t, (&RunAgentInput{ForwardedProps: "garbage"}).ExtractApprovals())
}

func TestNewTurnSDKMessagesExtractsTrailingTurn(t *testing.T) {
	in := &RunAgentInput{
		ThreadID: "t",
		Messages: []Message{
			{Role: RoleUser, Content: "first question"},
			{Role: RoleAssistant, Content: "first answer"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "f", Arguments: "{}"}}}},
			{Role: RoleTool, ToolCallID: "call_1", Content: "result"},
			{Role: RoleUser, Content: "follow-up"},
			{Role: RoleUser, Content: "and one more thing"},
		},
	}

	out := in.NewTurnSDKMessages()
	require.Len(t, out, 2)
	require.NotNil(t, out[0].OfInputMessage)
	assert.Equal(t, "follow-up", out[0].OfInputMessage.Content[0].OfInputText.Text)
	require.NotNil(t, out[1].OfInputMessage)
	assert.Equal(t, "and one more thing", out[1].OfInputMessage.Content[0].OfInputText.Text)
}

func TestNewTurnSDKMessagesFirstTurnTakesAll(t *testing.T) {
	in := &RunAgentInput{
		ThreadID: "t",
		Messages: []Message{
			{Role: RoleSystem, Content: "be helpful"},
			{Role: RoleUser, Content: "hi"},
		},
	}
	assert.Len(t, in.NewTurnSDKMessages(), 2)
}

func TestNewTurnSDKMessagesApprovalOnly(t *testing.T) {
	in := &RunAgentInput{
		ThreadID: "t",
		Messages: []Message{
			{Role: RoleUser, Content: "old"},
			{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "call_1", Type: "function"}}},
		},
		ForwardedProps: map[string]any{
			"command": map[string]any{
				"resume": map[string]any{
					"decisions": []any{map[string]any{"toolCallId": "call_1", "approved": true}},
				},
			},
		},
	}

	out := in.NewTurnSDKMessages()
	// History tail is an assistant message → no new turn messages;
	// only the approval response is forwarded, and it goes first.
	require.Len(t, out, 1)
	require.NotNil(t, out[0].OfFunctionCallApprovalResponse)
	assert.Equal(t, []string{"call_1"}, out[0].OfFunctionCallApprovalResponse.ApprovedCallIds)
}

func TestMessageIDsAreProviderPrefixed(t *testing.T) {
	// CopilotKit's HttpAgent assigns bare UUIDs; the OpenAI Responses
	// API rejects message ids that don't begin with "msg". The
	// conversion must coerce them.
	in := &RunAgentInput{
		ThreadID: "t",
		Messages: []Message{
			{ID: "e3e5ee9f-49b0-439b-8ad5-730a4d1a1dc9", Role: RoleUser, Content: "hi"},
			{ID: "", Role: RoleUser, Content: "no id"},
			{ID: "msg_keepme", Role: RoleAssistant, Content: "kept"},
		},
	}

	out := in.ToSDKMessages()
	require.Len(t, out, 3)

	// Bare UUID → prefixed, preserving the original for correlation.
	require.NotNil(t, out[0].OfInputMessage)
	assert.Equal(t, "msg_e3e5ee9f-49b0-439b-8ad5-730a4d1a1dc9", out[0].OfInputMessage.ID)

	// Empty → freshly minted, still prefixed.
	require.NotNil(t, out[1].OfInputMessage)
	assert.True(t, strings.HasPrefix(out[1].OfInputMessage.ID, "msg_"))

	// Already-prefixed assistant id passes through untouched.
	require.NotNil(t, out[2].OfOutputMessage)
	assert.Equal(t, "msg_keepme", out[2].OfOutputMessage.ID)
}

func TestToSDKMessagesFullConversion(t *testing.T) {
	in := &RunAgentInput{
		ThreadID: "t",
		Messages: []Message{
			{Role: RoleUser, Content: "q"},
			{Role: RoleAssistant, Content: "a", ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: ToolCallFunction{Name: "f", Arguments: "{}"}}}},
			{Role: RoleTool, ToolCallID: "call_1", Content: "result"},
			{Role: "made-up-role", Content: "dropped silently"},
		},
	}

	out := in.ToSDKMessages()
	require.Len(t, out, 4)
	assert.NotNil(t, out[0].OfInputMessage)
	assert.NotNil(t, out[1].OfOutputMessage)
	require.NotNil(t, out[2].OfFunctionCall)
	assert.Equal(t, "call_1", out[2].OfFunctionCall.CallID)
	require.NotNil(t, out[3].OfFunctionCallOutput)
	assert.Equal(t, "result", *out[3].OfFunctionCallOutput.Output.OfString)
}
