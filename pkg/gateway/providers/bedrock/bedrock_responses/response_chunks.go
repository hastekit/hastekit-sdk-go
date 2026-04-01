package bedrock_responses

import (
	"fmt"

	"github.com/bytedance/sonic"
)

// Bedrock ConverseStream API event types.
// The stream uses AWS event stream binary format. The `:event-type` header
// determines the payload type — the payload itself has no discriminating key.

// ConverseStreamEvent represents a single event from the ConverseStream API.
// Only one field will be non-nil per event, determined by the `:event-type` header.
type ConverseStreamEvent struct {
	MessageStart      *StreamMessageStart      `json:"messageStart,omitempty"`
	ContentBlockStart *StreamContentBlockStart `json:"contentBlockStart,omitempty"`
	ContentBlockDelta *StreamContentBlockDelta `json:"contentBlockDelta,omitempty"`
	ContentBlockStop  *StreamContentBlockStop  `json:"contentBlockStop,omitempty"`
	MessageStop       *StreamMessageStop       `json:"messageStop,omitempty"`
	Metadata          *StreamMetadata          `json:"metadata,omitempty"`
}

// UnmarshalEventPayload populates the correct field based on the event-type header.
// The payload is the raw JSON for that specific event type (no wrapper key).
func UnmarshalEventPayload(eventType string, payload []byte) (*ConverseStreamEvent, error) {
	event := &ConverseStreamEvent{}

	switch eventType {
	case "messageStart":
		event.MessageStart = &StreamMessageStart{}
		if err := sonic.Unmarshal(payload, event.MessageStart); err != nil {
			return nil, fmt.Errorf("unmarshaling messageStart: %w", err)
		}
	case "contentBlockStart":
		event.ContentBlockStart = &StreamContentBlockStart{}
		if err := sonic.Unmarshal(payload, event.ContentBlockStart); err != nil {
			return nil, fmt.Errorf("unmarshaling contentBlockStart: %w", err)
		}
	case "contentBlockDelta":
		event.ContentBlockDelta = &StreamContentBlockDelta{}
		if err := sonic.Unmarshal(payload, event.ContentBlockDelta); err != nil {
			return nil, fmt.Errorf("unmarshaling contentBlockDelta: %w", err)
		}
	case "contentBlockStop":
		event.ContentBlockStop = &StreamContentBlockStop{}
		if err := sonic.Unmarshal(payload, event.ContentBlockStop); err != nil {
			return nil, fmt.Errorf("unmarshaling contentBlockStop: %w", err)
		}
	case "messageStop":
		event.MessageStop = &StreamMessageStop{}
		if err := sonic.Unmarshal(payload, event.MessageStop); err != nil {
			return nil, fmt.Errorf("unmarshaling messageStop: %w", err)
		}
	case "metadata":
		event.Metadata = &StreamMetadata{}
		if err := sonic.Unmarshal(payload, event.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown converse stream event type: %q", eventType)
	}

	return event, nil
}

type StreamMessageStart struct {
	Role string `json:"role"` // "assistant"
}

type StreamContentBlockStart struct {
	ContentBlockIndex int                    `json:"contentBlockIndex"`
	Start             *ContentBlockStartData `json:"start,omitempty"`
}

type ContentBlockStartData struct {
	ToolUse          *StreamToolUseStart          `json:"toolUse,omitempty"`
	ReasoningContent *StreamReasoningContentStart `json:"reasoningContent,omitempty"`
}

type StreamReasoningContentStart struct {
	// Bedrock sends an empty reasoningContent object at block start
}

type StreamToolUseStart struct {
	ToolUseId string `json:"toolUseId"`
	Name      string `json:"name"`
}

type StreamContentBlockDelta struct {
	ContentBlockIndex int                    `json:"contentBlockIndex"`
	Delta             *ContentBlockDeltaData `json:"delta,omitempty"`
}

type ContentBlockDeltaData struct {
	Text             *string                      `json:"text,omitempty"`
	ToolUse          *StreamToolUseDelta          `json:"toolUse,omitempty"`
	ReasoningContent *StreamReasoningContentDelta `json:"reasoningContent,omitempty"`
}

type StreamReasoningContentDelta struct {
	Text      string `json:"text,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type StreamToolUseDelta struct {
	Input string `json:"input"` // partial JSON string
}

type StreamContentBlockStop struct {
	ContentBlockIndex int `json:"contentBlockIndex"`
}

type StreamMessageStop struct {
	StopReason string `json:"stopReason"` // "end_turn", "tool_use", "max_tokens", "stop_sequence", "content_filtered"
}

type StreamMetadata struct {
	Usage   *ConverseUsage   `json:"usage,omitempty"`
	Metrics *ConverseMetrics `json:"metrics,omitempty"`
}
