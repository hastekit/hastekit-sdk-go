package utils

import (
	"fmt"
	"strings"
)

// ParseDataURL splits a data URL into its content-type and the raw base64 string.
func ParseDataURL(dataURL string) (contentType string, base64Data string, err error) {
	// 1. Basic validation: check for "data:" prefix
	if !strings.HasPrefix(dataURL, "data:") {
		return "", "", fmt.Errorf("invalid data URL: missing data: prefix")
	}

	// 2. Find the comma that separates the header from the data
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid data URL: missing comma separator")
	}

	header := parts[0]
	base64Data = parts[1]

	// 3. Extract content-type from the header (e.g., "data:image/png;base64")
	// Remove the "data:" prefix
	header = strings.TrimPrefix(header, "data:")

	// Split by semicolon to isolate the MIME type from the encoding (e.g., ";base64")
	headerParts := strings.Split(header, ";")
	if len(headerParts) > 0 {
		contentType = headerParts[0]
	}

	return contentType, base64Data, nil
}
