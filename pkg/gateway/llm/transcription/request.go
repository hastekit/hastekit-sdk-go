package transcription

type Request struct {
	// Audio is the raw audio data to transcribe
	Audio []byte `json:"audio"`
	// AudioFilename is the filename for the audio file (e.g., "audio.mp3")
	AudioFilename string `json:"audio_filename"`
	// Model is the model to use for transcription (e.g., "whisper-1")
	Model string `json:"model"`
	// Language is the language of the input audio (ISO-639-1 format)
	Language *string `json:"language,omitempty"`
	// Prompt is an optional text to guide the model's style or continue a previous audio segment
	Prompt *string `json:"prompt,omitempty"`
	// ResponseFormat is the format of the output (json, text, srt, verbose_json, vtt)
	ResponseFormat *string `json:"response_format,omitempty"`
	// Temperature is the sampling temperature, between 0 and 1
	Temperature *float64 `json:"temperature,omitempty"`
	// TimestampGranularities specifies the timestamp granularities (word, segment)
	TimestampGranularities []string `json:"timestamp_granularities,omitempty"`
}
