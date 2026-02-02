package speech

type Response struct {
	Audio       []byte         `json:"audio"`
	ContentType string         `json:"content_type"`
	Usage       Usage          `json:"usage"`
	RawFields   map[string]any `json:"raw_fields"`
}

type ResponseChunk struct {
	OfAudioDelta *ChunkAudioDelta[ChunkTypeAudioDelta] `json:",omitempty"`
	OfAudioDone  *ChunkAudioDone[ChunkTypeAudioDone]   `json:",omitempty"`
}

type ChunkAudioDelta[T any] struct {
	Type  T      `json:"type"`
	Audio string `json:"audio"`
}

type ChunkAudioDone[T any] struct {
	Type  T     `json:"type"`
	Usage Usage `json:"usage"`
}

type Usage struct {
	InputTokens        int `json:"input_tokens"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokens        int `json:"output_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
	TotalTokens int `json:"total_tokens"`
}
