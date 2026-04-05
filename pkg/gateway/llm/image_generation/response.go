package image_generation

type Response struct {
	Images    []Image        `json:"images"`
	Usage     *Usage         `json:"usage,omitempty"`
	RawFields map[string]any `json:"raw_fields,omitempty"`
}

type Image struct {
	B64JSON       *string `json:"b64_json,omitempty"`
	URL           *string `json:"url,omitempty"`
	RevisedPrompt *string `json:"revised_prompt,omitempty"`
	MimeType      *string `json:"mime_type,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
