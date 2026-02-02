package anthropic_responses

type Response struct {
	Model        string             `json:"model"`
	Id           string             `json:"id"`
	Type         string             `json:"type"` // "message"
	Role         Role               `json:"role"`
	Content      Contents           `json:"content"`
	StopReason   StopReason         `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence"`
	Usage        *ChunkMessageUsage `json:"usage"`
	Error        *Error             `json:"error"`
	ServiceTier  string             `json:"service_tier"`
}

type Error struct {
	Message string `json:"message"`
}
