package anthropic_responses

import (
	"errors"

	"github.com/bytedance/sonic"
)

type Request struct {
	MaxTokens int            `json:"max_tokens"`
	Model     string         `json:"model"`
	Messages  []MessageUnion `json:"messages"`

	System       []TextContent     `json:"system,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	TopK         *int64            `json:"top_k,omitempty"`
	TopP         *float64          `json:"top_p,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Thinking     *ThinkingParam    `json:"thinking,omitempty"`
	Tools        []ToolUnion       `json:"tools,omitempty"`
	Stream       *bool             `json:"stream,omitempty"`
	OutputFormat map[string]any    `json:"output_format,omitempty"`
}

type ThinkingParam struct {
	Type         *string `json:"type"` // "enabled" or "disabled"
	BudgetTokens *int    `json:"budget_tokens"`
}

type MessageUnion struct {
	Role    Role     `json:"role"` // "user" or "assistant"
	Content Contents `json:"content"`
}

type Contents []ContentUnion

type ContentUnion struct {
	OfText                        *TextContent                    `json:",omitempty"`
	OfToolUse                     *ToolUseContent                 `json:",omitempty"`
	OfToolResult                  *ToolUseResultContent           `json:",omitempty"`
	OfThinking                    *ThinkingContent                `json:",omitempty"`
	OfRedactedThinking            *RedactedThinkingContent        `json:",omitempty"`
	OfServerToolUse               *ServerToolUseContent           `json:",omitempty"`
	OfWebSearchResult             *WebSearchResultContent         `json:",omitempty"`
	OfBashCodeExecutionToolResult *BashCodeExecutionResultContent `json:",omitempty"`
}

func (u *ContentUnion) UnmarshalJSON(data []byte) error {
	var textContext TextContent
	if err := sonic.Unmarshal(data, &textContext); err == nil {
		u.OfText = &textContext
		return nil
	}

	var toolUseContent ToolUseContent
	if err := sonic.Unmarshal(data, &toolUseContent); err == nil {
		u.OfToolUse = &toolUseContent
		return nil
	}

	var toolUseResultContent ToolUseResultContent
	if err := sonic.Unmarshal(data, &toolUseResultContent); err == nil {
		u.OfToolResult = &toolUseResultContent
		return nil
	}

	var thinkingContent ThinkingContent
	if err := sonic.Unmarshal(data, &thinkingContent); err == nil {
		u.OfThinking = &thinkingContent
		return nil
	}

	var redactedThinkingContent RedactedThinkingContent
	if err := sonic.Unmarshal(data, &redactedThinkingContent); err == nil {
		u.OfRedactedThinking = &redactedThinkingContent
		return nil
	}

	var serverToolUseContent ServerToolUseContent
	if err := sonic.Unmarshal(data, &serverToolUseContent); err == nil {
		u.OfServerToolUse = &serverToolUseContent
		return nil
	}

	var webSearchResultContent WebSearchResultContent
	if err := sonic.Unmarshal(data, &webSearchResultContent); err == nil {
		u.OfWebSearchResult = &webSearchResultContent
		return nil
	}

	var bashCodeExecutionToolResult BashCodeExecutionResultContent
	if err := sonic.Unmarshal(data, &bashCodeExecutionToolResult); err == nil {
		u.OfBashCodeExecutionToolResult = &bashCodeExecutionToolResult
		return nil
	}

	return errors.New("invalid input content union")
}

func (u *ContentUnion) MarshalJSON() ([]byte, error) {
	if u.OfText != nil {
		return sonic.Marshal(*u.OfText)
	}

	if u.OfToolUse != nil {
		return sonic.Marshal(*u.OfToolUse)
	}

	if u.OfToolResult != nil {
		return sonic.Marshal(*u.OfToolResult)
	}

	if u.OfThinking != nil {
		return sonic.Marshal(*u.OfThinking)
	}

	if u.OfRedactedThinking != nil {
		return sonic.Marshal(*u.OfRedactedThinking)
	}

	if u.OfServerToolUse != nil {
		return sonic.Marshal(*u.OfServerToolUse)
	}

	if u.OfWebSearchResult != nil {
		return sonic.Marshal(*u.OfWebSearchResult)
	}

	if u.OfBashCodeExecutionToolResult != nil {
		return sonic.Marshal(*u.OfBashCodeExecutionToolResult)
	}

	return nil, nil
}

type TextContent struct {
	Type      ContentTypeText `json:"type"` // "text"
	Text      string          `json:"text"`
	Citations []Citation      `json:"citations,omitempty"`
}

type Citation struct {
	Type           string `json:"type"` // web_search_result_location
	Url            string `json:"url"`
	Title          string `json:"title"`
	EncryptedIndex string `json:"encrypted_index"`
	CitedText      string `json:"cited_text"`
}

type ToolUseContent struct {
	Type  ContentTypeToolUse `json:"type"`
	ID    string             `json:"id"`
	Name  string             `json:"name"`
	Input any                `json:"input"`
}

type ToolUseResultContent struct {
	Type      ContentTypeToolUseResult `json:"type"` // "tool_result"
	ToolUseID string                   `json:"tool_use_id"`
	Content   []ContentUnion           `json:"content"`
	IsError   *bool                    `json:"is_error,omitempty"`
}

type ThinkingContent struct {
	Type      ContentTypeThinking `json:"type"`
	Thinking  string              `json:"thinking"`
	Signature string              `json:"signature"`
}

type RedactedThinkingContent struct {
	Type ContentTypeRedactedThinking `json:"type"`
	Data string                      `json:"data"`
}

type ServerToolUseContent struct {
	Type  ContentTypeServerToolUse `json:"type"`
	Id    string                   `json:"id"`
	Name  string                   `json:"name"` // "web_search", "bash_code_execution"
	Input struct {
		Query   string `json:"query"`
		Command string `json:"command"`
	} `json:"input"`
}

type WebSearchResultContent struct {
	Type      ContentTypeWebSearchResultContent `json:"type"`
	ToolUseId string                            `json:"tool_use_id"`
	Content   []WebSearchResultContentParam     `json:"content"`
}

type WebSearchResultContentParam struct {
	Type             string `json:"type"` // "web_search_result"
	Url              string `json:"url"`
	Title            string `json:"title"`
	EncryptedContent string `json:"encrypted_content"`
	PageAge          string `json:"page_age"`
}

type WebSearchToolUserLocationParam struct {
	Type     string `json:"type"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Timezone string `json:"timezone"`
}

type BashCodeExecutionResultContent struct {
	Type      ContentTypeBashCodeExecutionToolResultContent `json:"type"`
	ToolUseId string                                        `json:"tool_use_id"`
	Content   BashCodeExecutionResultParam                  `json:"content"`
}

type BashCodeExecutionResultParam struct {
	Type       string `json:"type"` // "bash_code_execution_result"
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ReturnCode int    `json:"return_code"`
	Content    []any  `json:"content"`
}

type ToolUnion struct {
	OfCustomTool        *CustomTool        `json:",omitempty"`
	OfWebSearchTool     *WebSearchTool     `json:",omitempty"`
	OfCodeExecutionTool *CodeExecutionTool `json:",omitempty"`
}

func (u *ToolUnion) UnmarshalJSON(data []byte) error {
	var customTool CustomTool
	if err := sonic.Unmarshal(data, &customTool); err == nil {
		u.OfCustomTool = &customTool
		return nil
	}

	var webSearchTool WebSearchTool
	if err := sonic.Unmarshal(data, &webSearchTool); err == nil {
		u.OfWebSearchTool = &webSearchTool
		return nil
	}

	var codeExecutionTool CodeExecutionTool
	if err := sonic.Unmarshal(data, &codeExecutionTool); err == nil {
		u.OfCodeExecutionTool = &codeExecutionTool
		return nil
	}

	return errors.New("invalid tool union")
}

func (u *ToolUnion) MarshalJSON() ([]byte, error) {
	if u.OfCustomTool != nil {
		return sonic.Marshal(u.OfCustomTool)
	}

	if u.OfWebSearchTool != nil {
		return sonic.Marshal(u.OfWebSearchTool)
	}

	if u.OfCodeExecutionTool != nil {
		return sonic.Marshal(u.OfCodeExecutionTool)
	}

	return nil, nil
}

type CustomTool struct {
	Type        ToolTypeCustomTool `json:"type"` // "custom"
	Name        string             `json:"name"`
	Description *string            `json:"description,omitempty"`
	InputSchema map[string]any     `json:"input_schema"`
}

type WebSearchTool struct {
	Type           ToolTypeWebSearchTool           `json:"type"`
	Name           string                          `json:"name"` // "web_search"
	MaxUses        int                             `json:"max_uses"`
	AllowedDomains []string                        `json:"allowed_domains,omitempty"`
	BlockedDomains []string                        `json:"blocked_domains,omitempty"`
	UserLocation   *WebSearchToolUserLocationParam `json:"user_location,omitempty"`
}

type CodeExecutionTool struct {
	Type ToolTypeCodeExecutionTool `json:"type"`
	Name string                    `json:"name"` // "code_execution"
}
