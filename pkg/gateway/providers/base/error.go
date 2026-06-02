package base

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
)

// ParseErrorResponse turns a non-2xx provider HTTP response into a
// descriptive error. It reads (and so allows the caller to close) the
// response body and never panics on an unexpected body shape: when the
// provider's usual {"error":{"message":...}} envelope is absent — e.g. a
// proxy returns HTML, a gateway returns plain text, or the body is empty —
// it falls back to the HTTP status code and the raw body.
//
// Both the object form ({"error":{"message":...}}) and the array form
// ([{"error":{"message":...}}], used by some Google/Gemini endpoints) are
// recognized.
func ParseErrorResponse(res *http.Response) error {
	body, _ := io.ReadAll(res.Body)

	if msg := extractErrorMessage(body); msg != "" {
		return errors.New(msg)
	}

	if len(body) > 0 {
		return fmt.Errorf("request failed with status %d: %s", res.StatusCode, string(body))
	}
	return fmt.Errorf("request failed with status %d", res.StatusCode)
}

func extractErrorMessage(body []byte) string {
	var asObject map[string]any
	if err := sonic.Unmarshal(body, &asObject); err == nil {
		if msg := messageFromErrorObject(asObject["error"]); msg != "" {
			return msg
		}
	}

	var asArray []map[string]any
	if err := sonic.Unmarshal(body, &asArray); err == nil && len(asArray) > 0 {
		if msg := messageFromErrorObject(asArray[0]["error"]); msg != "" {
			return msg
		}
	}

	return ""
}

func messageFromErrorObject(v any) string {
	errObj, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if msg, ok := errObj["message"].(string); ok {
		return msg
	}
	return ""
}
