package bedrock

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	anthropic_responses "github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/anthropic/anthropic_responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/base"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type ClientOptions struct {
	// https://bedrock-runtime.us-east-1.amazonaws.com
	BaseURL string
	ApiKey  string
	Headers map[string]string

	transport *http.Client
}

type Client struct {
	*base.BaseProvider
	opts *ClientOptions
}

func NewClient(opts *ClientOptions) *Client {
	if opts.transport == nil {
		opts.transport = http.DefaultClient
	}

	return &Client{
		opts: opts,
	}
}

// buildInvokeURL constructs the Bedrock invoke URL for the given model.
// Format: {BaseURL}/model/{modelId}/invoke
func buildInvokeURL(baseURL, model string) string {
	return baseURL + "/model/" + url.PathEscape(model) + "/invoke"
}

// buildStreamURL constructs the Bedrock streaming invoke URL for the given model.
// Format: {BaseURL}/model/{modelId}/invoke-with-response-stream
func buildStreamURL(baseURL, model string) string {
	return baseURL + "/model/" + url.PathEscape(model) + "/invoke-with-response-stream"
}

func (c *Client) NewResponses(ctx context.Context, inp *responses2.Request) (*responses2.Response, error) {
	anthropicRequest := anthropic_responses.NativeRequestToRequest(inp)

	// Bedrock does not use streaming for non-streaming requests
	anthropicRequest.Stream = utils.Ptr(false)

	payload, err := sonic.Marshal(anthropicRequest)
	if err != nil {
		return nil, err
	}

	reqURL := buildInvokeURL(c.opts.BaseURL, anthropicRequest.Model)
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Apply custom headers (used for AWS auth headers like Authorization, x-amz-date, etc.)
	for k, v := range c.opts.Headers {
		req.Header.Set(k, v)
	}

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, errors.New("bedrock invoke failed (" + res.Status + "): " + string(body))
	}

	var anthropicResponse *anthropic_responses.Response
	err = utils.DecodeJSON(res.Body, &anthropicResponse)
	if err != nil {
		return nil, err
	}

	if anthropicResponse.Error != nil {
		return nil, errors.New(anthropicResponse.Error.Message)
	}

	return anthropicResponse.ToNativeResponse(), nil
}

func (c *Client) NewStreamingResponses(ctx context.Context, inp *responses2.Request) (chan *responses2.ResponseChunk, error) {
	anthropicRequest := anthropic_responses.NativeRequestToRequest(inp)

	// Streaming must be enabled
	anthropicRequest.Stream = utils.Ptr(true)

	payload, err := sonic.Marshal(anthropicRequest)
	if err != nil {
		return nil, err
	}

	reqURL := buildStreamURL(c.opts.BaseURL, anthropicRequest.Model)
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.amazon.eventstream")

	// Apply custom headers (used for AWS auth headers)
	for k, v := range c.opts.Headers {
		req.Header.Set(k, v)
	}

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return nil, errors.New("bedrock streaming invoke failed (" + res.Status + "): " + string(body))
	}

	out := make(chan *responses2.ResponseChunk)

	go func() {
		defer res.Body.Close()
		defer close(out)

		converter := anthropic_responses.ResponseChunkToNativeResponseChunkConverter{}

		for {
			msg, err := decodeEventStreamMessage(res.Body)
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return
				}
				slog.WarnContext(ctx, "error decoding bedrock event stream message", slog.Any("error", err))
				return
			}

			// Check message type header
			messageType := msg.Headers[":message-type"]
			if messageType == "exception" {
				slog.WarnContext(ctx, "bedrock streaming exception",
					slog.String("exception-type", msg.Headers[":exception-type"]),
					slog.String("payload", string(msg.Payload)),
				)
				return
			}

			// Only process "event" messages with "chunk" event type
			if messageType != "event" {
				continue
			}

			// Decode the Bedrock envelope: {"bytes":"<base64-encoded-anthropic-chunk>"}
			anthropicJSON, err := decodeBedrockChunkPayload(msg.Payload)
			if err != nil {
				slog.WarnContext(ctx, "error decoding bedrock chunk payload", slog.Any("error", err))
				continue
			}

			// The decoded bytes contain the Anthropic SSE data.
			// It may have the "data:" prefix or just be raw JSON.
			chunkData := string(anthropicJSON)

			// Handle SSE-style events: may contain "event:" and "data:" lines
			lines := strings.Split(chunkData, "\n")
			for _, line := range lines {
				line = strings.TrimRight(line, "\r")
				if strings.HasPrefix(line, "data:") {
					line = strings.TrimPrefix(line, "data:")
					line = strings.TrimSpace(line)
				} else if line == "" || strings.HasPrefix(line, "event:") {
					continue
				}

				anthropicChunk := &anthropic_responses.ResponseChunk{}
				if err := sonic.Unmarshal([]byte(line), anthropicChunk); err != nil {
					slog.WarnContext(ctx, "unable to unmarshal bedrock anthropic response chunk",
						slog.String("data", line),
						slog.Any("error", err),
					)
					continue
				}

				for _, nativeChunk := range converter.ResponseChunkToNativeResponseChunk(anthropicChunk) {
					out <- nativeChunk
				}
			}
		}
	}()

	return out, nil
}
