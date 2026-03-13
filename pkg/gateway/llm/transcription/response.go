package transcription

type Response struct {
	Text     string          `json:"text"`
	Language *string         `json:"language,omitempty"`
	Duration *float64        `json:"duration,omitempty"`
	Words    []Word          `json:"words,omitempty"`
	Segments []Segment       `json:"segments,omitempty"`
	Usage    *Usage          `json:"usage,omitempty"`
	Raw      map[string]any  `json:"raw,omitempty"`
}

type Word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Segment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
