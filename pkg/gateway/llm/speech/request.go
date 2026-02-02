package speech

type Request struct {
	Input          string  `json:"input"`
	Model          string  `json:"model"`
	Voice          string  `json:"voice"`
	Instruction    *string `json:"instruction,omitempty"`
	ResponseFormat *string `json:"response_format,omitempty"`
	Speed          *int    `json:"speed,omitempty"`
	StreamFormat   *string `json:"stream_format,omitempty"`
}

func (r *Request) IsStreamingRequest() bool {
	if r.Speed != nil {
		return true
	}
	return false
}
