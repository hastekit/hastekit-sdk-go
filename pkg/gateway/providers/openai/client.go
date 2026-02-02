package openai

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/internal/utils"
	chat_completion2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/chat_completion"
	embeddings2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"
	responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	speech2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/speech"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/base"
	openai_chat_completion2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/openai/openai_chat_completion"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/openai/openai_embeddings"
	openai_responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/openai/openai_responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/openai/openai_speech"
)

type ClientOptions struct {
	// https://api.openai.com/v1
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

	if opts.BaseURL == "" {
		opts.BaseURL = "https://api.openai.com/v1"
	}

	return &Client{
		opts: opts,
	}
}

func (c *Client) NewResponses(ctx context.Context, inp *responses2.Request) (*responses2.Response, error) {
	openAiRequest := openai_responses2.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/responses", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var openAiResponse *openai_responses2.Response
	err = utils.DecodeJSON(res.Body, &openAiResponse)
	if err != nil {
		return nil, err
	}

	if openAiResponse.Error != nil {
		return nil, errors.New(openAiResponse.Error.Message)
	}

	return openAiResponse.ToNativeResponse(), nil
}

func (c *Client) NewStreamingResponses(ctx context.Context, inp *responses2.Request) (chan *responses2.ResponseChunk, error) {
	openAiRequest := openai_responses2.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/responses", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		var errResp map[string]any
		err = utils.DecodeJSON(res.Body, &errResp)
		return nil, errors.New(errResp["error"].(map[string]any)["message"].(string))
	}

	out := make(chan *responses2.ResponseChunk)

	go func() {
		defer res.Body.Close()
		defer close(out)
		reader := bufio.NewReader(res.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimRight(line, "\r\n")
			fmt.Println(line)
			if strings.HasPrefix(line, "data:") {
				openAiResponseChunk := &openai_responses2.ResponseChunk{}
				err = sonic.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), openAiResponseChunk)
				if err != nil {
					slog.WarnContext(ctx, "unable to unmarshal openai response chunk", slog.String("data", line), slog.Any("error", err))
					continue
				}
				out <- openAiResponseChunk.ToNativeResponseChunk()
			}
		}
	}()

	return out, nil
}

func (c *Client) NewEmbedding(ctx context.Context, inp *embeddings2.Request) (*embeddings2.Response, error) {
	openAiRequest := openai_embeddings.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/embeddings", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var errResp map[string]any
		err = utils.DecodeJSON(res.Body, &errResp)
		return nil, errors.New(errResp["error"].(map[string]any)["message"].(string))
	}

	var openAiResponse *openai_embeddings.Response
	err = utils.DecodeJSON(res.Body, &openAiResponse)
	if err != nil {
		return nil, err
	}

	return openAiResponse.ToNativeResponse(), nil
}

func (c *Client) NewChatCompletion(ctx context.Context, inp *chat_completion2.Request) (*chat_completion2.Response, error) {
	openAiRequest := openai_chat_completion2.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/chat/completions", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var errResp map[string]any
		err = utils.DecodeJSON(res.Body, &errResp)
		if err != nil {
			return nil, err
		}
		if errorObj, ok := errResp["error"].(map[string]any); ok {
			if message, ok := errorObj["message"].(string); ok {
				return nil, errors.New(message)
			}
		}
		return nil, errors.New("unknown error occurred")
	}

	var openAiResponse *openai_chat_completion2.Response
	err = utils.DecodeJSON(res.Body, &openAiResponse)
	if err != nil {
		return nil, err
	}

	return openAiResponse.ToNativeResponse(), nil
}

func (c *Client) NewStreamingChatCompletion(ctx context.Context, inp *chat_completion2.Request) (chan *chat_completion2.ResponseChunk, error) {
	openAiRequest := openai_chat_completion2.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/chat/completions", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		var errResp map[string]any
		err = utils.DecodeJSON(res.Body, &errResp)
		if err != nil {
			return nil, err
		}
		if errorObj, ok := errResp["error"].(map[string]any); ok {
			if message, ok := errorObj["message"].(string); ok {
				return nil, errors.New(message)
			}
		}
		return nil, errors.New("unknown error occurred")
	}

	out := make(chan *chat_completion2.ResponseChunk)

	go func() {
		defer res.Body.Close()
		defer close(out)
		reader := bufio.NewReader(res.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimRight(line, "\r\n")
			fmt.Println(line)

			if line == "data: [DONE]" {
				return
			}

			if strings.HasPrefix(line, "data:") {
				openAiChatCompletionChunk := &openai_chat_completion2.ResponseChunk{}
				err = sonic.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), openAiChatCompletionChunk)
				if err != nil {
					slog.WarnContext(ctx, "unable to unmarshal chat completion response chunk", slog.String("data", line), slog.Any("error", err))
					continue
				}
				out <- openAiChatCompletionChunk.ToNativeResponseChunk()
			}
		}
	}()

	return out, nil
}

func (c *Client) NewSpeech(ctx context.Context, in *speech2.Request) (*speech2.Response, error) {
	openAiRequest := openai_speech.NativeRequestToRequest(in)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/audio/speech", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var errResp map[string]any
		err = utils.DecodeJSON(res.Body, &errResp)
		if err != nil {
			return nil, err
		}
		if errorObj, ok := errResp["error"].(map[string]any); ok {
			if message, ok := errorObj["message"].(string); ok {
				return nil, errors.New(message)
			}
		}
		return nil, errors.New("unknown error occurred")
	}

	// Handle gzip compressed response
	var reader io.Reader = res.Body
	if res.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(res.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read the raw audio binary data (decompressed if gzip)
	audioData, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Create response with audio data
	openAiResponse := &openai_speech.Response{
		Response: speech2.Response{
			Audio:       audioData,
			ContentType: res.Header.Get("Content-Type"),
		},
	}

	return openAiResponse.ToNativeResponse(), nil
}

func (c *Client) NewStreamingSpeech(ctx context.Context, in *speech2.Request) (chan *speech2.ResponseChunk, error) {
	openAiRequest := openai_speech.NativeRequestToRequest(in)

	payload, err := sonic.Marshal(openAiRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.opts.BaseURL+"/audio/speech", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.opts.ApiKey)

	res, err := c.opts.transport.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		var errResp map[string]any
		err = utils.DecodeJSON(res.Body, &errResp)
		if err != nil {
			return nil, err
		}
		if errorObj, ok := errResp["error"].(map[string]any); ok {
			if message, ok := errorObj["message"].(string); ok {
				return nil, errors.New(message)
			}
		}
		return nil, errors.New("unknown error occurred")
	}

	out := make(chan *speech2.ResponseChunk)

	go func() {
		defer res.Body.Close()
		defer close(out)
		reader := bufio.NewReader(res.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimRight(line, "\r\n")

			if line == "data: [DONE]" {
				return
			}

			if strings.HasPrefix(line, "data:") {
				openAiSpeechChunk := &openai_speech.ResponseChunk{}
				err = sonic.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), openAiSpeechChunk)
				if err != nil {
					slog.WarnContext(ctx, "unable to unmarshal speech response chunk", slog.String("data", line), slog.Any("error", err))
					continue
				}
				out <- openAiSpeechChunk.ToNativeResponse()
			}
		}
	}()

	return out, nil
}
