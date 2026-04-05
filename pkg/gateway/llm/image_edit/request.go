package image_edit

// ImageInput represents a single image to be edited
type ImageInput struct {
	// Data is the raw image bytes
	Data []byte `json:"data"`
	// Filename is the filename for the image (e.g., "image.png") used for MIME type detection
	Filename string `json:"filename,omitempty"`
}

type Request struct {
	// Prompt describes the desired edit or the full desired final image
	Prompt string `json:"prompt"`
	// Model is the model to use for image editing
	Model string `json:"model"`
	// Images is the list of images to edit (OpenAI supports up to 16, Gemini and xAI support 1)
	Images []ImageInput `json:"images"`
	// N is the number of images to generate
	N *int `json:"n,omitempty"`
	// Size specifies the output image size (e.g., "1024x1024")
	Size *string `json:"size,omitempty"`
	// Quality specifies the output quality (e.g., "auto", "high", "medium", "low")
	Quality *string `json:"quality,omitempty"`
	// ResponseFormat specifies the format of the response ("b64_json" or "url")
	ResponseFormat *string `json:"response_format,omitempty"`
	// OutputFormat specifies the output image format ("png", "jpeg", "webp"). Default is "png"
	OutputFormat *string `json:"output_format,omitempty"`
	// AspectRatio specifies the desired aspect ratio (e.g., "1:1", "16:9")
	AspectRatio *string `json:"aspect_ratio,omitempty"`
	// Resolution specifies the output resolution (e.g., "1k", "2k")
	Resolution *string `json:"resolution,omitempty"`
}
