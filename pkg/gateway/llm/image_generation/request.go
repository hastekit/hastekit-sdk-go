package image_generation

type Request struct {
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model"`
	N              *int    `json:"n,omitempty"`               // Number of images to generate
	Size           *string `json:"size,omitempty"`            // e.g., "1024x1024"
	Quality        *string `json:"quality,omitempty"`         // e.g., "auto", "high", "medium", "low"
	Style          *string `json:"style,omitempty"`           // e.g., "natural", "vivid"
	ResponseFormat *string `json:"response_format,omitempty"` // "b64_json" or "url"
	Background     *string `json:"background,omitempty"`      // "opaque", "transparent", "auto"
	OutputFormat   *string `json:"output_format,omitempty"`   // "png", "jpeg", "webp"
	AspectRatio    *string `json:"aspect_ratio,omitempty"`    // e.g., "1:1", "16:9", "9:16"
	Resolution     *string `json:"resolution,omitempty"`      // e.g., "1k", "2k", "4k"
}
