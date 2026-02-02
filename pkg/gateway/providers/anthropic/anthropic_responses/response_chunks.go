package anthropic_responses

import (
	"errors"

	"github.com/bytedance/sonic"
)

type ResponseChunk struct {
	OfMessageStart *ChunkMessage[ChunkTypeMessageStart] `json:",omitempty"`
	OfMessageDelta *ChunkMessage[ChunkTypeMessageDelta] `json:",omitempty"`
	OfMessageStop  *ChunkMessage[ChunkTypeMessageStop]  `json:",omitempty"`

	OfContentBlockStart *ChunkContentBlock[ChunkTypeContentBlockStart] `json:",omitempty"`
	OfContentBlockDelta *ChunkContentBlock[ChunkTypeContentBlockDelta] `json:",omitempty"`
	OfContentBlockStop  *ChunkContentBlock[ChunkTypeContentBlockStop]  `json:",omitempty"`

	OfPing *ChunkPing `json:",omitempty"`
}

func (u *ResponseChunk) UnmarshalJSON(data []byte) error {
	var msgStart *ChunkMessage[ChunkTypeMessageStart]
	if err := sonic.Unmarshal(data, &msgStart); err == nil {
		u.OfMessageStart = msgStart
		return nil
	}

	var msgDelta *ChunkMessage[ChunkTypeMessageDelta]
	if err := sonic.Unmarshal(data, &msgDelta); err == nil {
		u.OfMessageDelta = msgDelta
		return nil
	}

	var msgStop *ChunkMessage[ChunkTypeMessageStop]
	if err := sonic.Unmarshal(data, &msgStop); err == nil {
		u.OfMessageStop = msgStop
		return nil
	}

	var contentBlockStart *ChunkContentBlock[ChunkTypeContentBlockStart]
	if err := sonic.Unmarshal(data, &contentBlockStart); err == nil {
		u.OfContentBlockStart = contentBlockStart
		return nil
	}

	var contentBlockDelta *ChunkContentBlock[ChunkTypeContentBlockDelta]
	if err := sonic.Unmarshal(data, &contentBlockDelta); err == nil {
		u.OfContentBlockDelta = contentBlockDelta
		return nil
	}

	var contentBlockStop *ChunkContentBlock[ChunkTypeContentBlockStop]
	if err := sonic.Unmarshal(data, &contentBlockStop); err == nil {
		u.OfContentBlockStop = contentBlockStop
		return nil
	}

	var ping *ChunkPing
	if err := sonic.Unmarshal(data, &ping); err == nil {
		u.OfPing = ping
		return nil
	}

	return errors.New("invalid response chunk union")
}

func (u *ResponseChunk) MarshalJSON() ([]byte, error) {
	if u.OfMessageStart != nil {
		return sonic.Marshal(u.OfMessageStart)
	}

	if u.OfMessageDelta != nil {
		return sonic.Marshal(u.OfMessageDelta)
	}

	if u.OfMessageStop != nil {
		return sonic.Marshal(u.OfMessageStop)
	}

	if u.OfContentBlockStart != nil {
		return sonic.Marshal(u.OfContentBlockStart)
	}

	if u.OfContentBlockDelta != nil {
		return sonic.Marshal(u.OfContentBlockDelta)
	}

	if u.OfContentBlockStop != nil {
		return sonic.Marshal(u.OfContentBlockStop)
	}

	if u.OfPing != nil {
		return sonic.Marshal(u.OfPing)
	}

	return nil, nil
}

func (u *ResponseChunk) ChunkType() string {
	if u.OfMessageStart != nil {
		return u.OfMessageStart.Type.Value()
	}

	if u.OfMessageDelta != nil {
		return u.OfMessageDelta.Type.Value()
	}

	if u.OfMessageStop != nil {
		return u.OfMessageStop.Type.Value()
	}

	if u.OfContentBlockStart != nil {
		return u.OfContentBlockStart.Type.Value()
	}

	if u.OfContentBlockDelta != nil {
		return u.OfContentBlockDelta.Type.Value()
	}

	if u.OfContentBlockStop != nil {
		return u.OfContentBlockStop.Type.Value()
	}

	if u.OfPing != nil {
		return u.OfPing.Type
	}

	return ""
}

type ChunkMessage[T any] struct {
	Type    T                 `json:"type"`
	Message *ChunkMessageData `json:"message,omitempty"`

	// On message_delta only
	Usage *ChunkMessageUsage `json:"usage,omitempty"`
	Delta *struct {
		StopReason   interface{} `json:"stop_reason"`
		StopSequence interface{} `json:"stop_sequence"`
	} `json:"delta,omitempty"`
}

type ChunkMessageData struct {
	Model        string             `json:"model"`
	Id           string             `json:"id"`
	Type         string             `json:"type"`
	Role         Role               `json:"role"`
	Content      []interface{}      `json:"content"`
	StopReason   interface{}        `json:"stop_reason"`
	StopSequence interface{}        `json:"stop_sequence"`
	Usage        *ChunkMessageUsage `json:"usage,omitempty"`
}

type ChunkMessageUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreation            *struct {
		Ephemeral5MInputTokens int `json:"ephemeral_5m_input_tokens"`
		Ephemeral1HInputTokens int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation,omitempty"`
	ServiceTier string `json:"service_tier"`
}

type ChunkContentBlock[T any] struct {
	Type  T   `json:"type"`
	Index int `json:"index"`

	// On content_block_start
	ContentBlock *ContentUnion `json:"content_block,omitempty"`

	// On content_block_delta
	Delta *ChunkContentBlockDeltaUnion `json:"delta,omitempty"`
}

type ChunkPing struct {
	Type string `json:"type"`
}

type ChunkContentBlockDeltaUnion struct {
	OfText              *DeltaTextContent              `json:",omitempty"`
	OfInputJSON         *DeltaInputJSONContent         `json:",omitempty"`
	OfThinking          *DeltaThinkingContent          `json:",omitempty"`
	OfThinkingSignature *DeltaThinkingSignatureContent `json:",omitempty"`
	OfCitation          *DeltaCitation                 `json:",omitempty"`
}

func (u *ChunkContentBlockDeltaUnion) UnmarshalJSON(data []byte) error {
	var textContent *DeltaTextContent
	if err := sonic.Unmarshal(data, &textContent); err == nil {
		u.OfText = textContent
		return nil
	}

	var inputJsonContent *DeltaInputJSONContent
	if err := sonic.Unmarshal(data, &inputJsonContent); err == nil {
		u.OfInputJSON = inputJsonContent
		return nil
	}

	var thinkingContent *DeltaThinkingContent
	if err := sonic.Unmarshal(data, &thinkingContent); err == nil {
		u.OfThinking = thinkingContent
		return nil
	}

	var thinkingSignatureContent *DeltaThinkingSignatureContent
	if err := sonic.Unmarshal(data, &thinkingSignatureContent); err == nil {
		u.OfThinkingSignature = thinkingSignatureContent
		return nil
	}

	var citation *DeltaCitation
	if err := sonic.Unmarshal(data, &citation); err == nil {
		u.OfCitation = citation
		return nil
	}

	return errors.New("invalid delta union")
}

func (u *ChunkContentBlockDeltaUnion) MarshalJSON() ([]byte, error) {
	if u.OfText != nil {
		return sonic.Marshal(u.OfText)
	}

	if u.OfInputJSON != nil {
		return sonic.Marshal(u.OfInputJSON)
	}

	if u.OfThinking != nil {
		return sonic.Marshal(u.OfThinking)
	}

	if u.OfThinkingSignature != nil {
		return sonic.Marshal(u.OfThinkingSignature)
	}

	if u.OfCitation != nil {
		return sonic.Marshal(u.OfCitation)
	}

	return nil, nil
}

type DeltaTextContent struct {
	Type ContentTypeDeltaText `json:"type"` // "text_delta"
	Text string               `json:"text"`
}

type DeltaInputJSONContent struct {
	Type        ContentTypeDeltaInputJSON `json:"type"` // "input_json_delta"
	PartialJSON string                    `json:"partial_json"`
}

type DeltaThinkingContent struct {
	Type     ContentTypeDeltaThinking `json:"type"`
	Thinking string                   `json:"thinking"`
}

type DeltaThinkingSignatureContent struct {
	Type      ContentTypeDeltaThinkingSignature `json:"type"`
	Signature string                            `json:"signature"`
}

type DeltaCitation struct {
	Type     ContentTypeDeltaCitation `json:"type"`
	Citation Citation                 `json:"citation"`
}
