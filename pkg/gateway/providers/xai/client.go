package xai

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/base"
	xai_responses2 "github.com/hastekit/hastekit-sdk-go/pkg/gateway/providers/xai/xai_responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type ClientOptions struct {
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
		opts.BaseURL = "https://api.x.ai/v1"
	}

	return &Client{
		opts: opts,
	}
}

func (c *Client) NewResponses(ctx context.Context, inp *responses2.Request) (*responses2.Response, error) {
	xaiRequest := xai_responses2.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(xaiRequest)
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

	var xaiResponse *xai_responses2.Response
	err = utils.DecodeJSON(res.Body, &xaiResponse)
	if err != nil {
		return nil, err
	}

	if xaiResponse.Error != nil {
		return nil, errors.New(xaiResponse.Error.Message)
	}

	return xaiResponse.ToNativeResponse(), nil
}

func (c *Client) NewStreamingResponses(ctx context.Context, inp *responses2.Request) (chan *responses2.ResponseChunk, error) {
	xaiRequest := xai_responses2.NativeRequestToRequest(inp)

	payload, err := sonic.Marshal(xaiRequest)
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
		var errResp string
		err = utils.DecodeJSON(res.Body, &errResp)
		return nil, errors.New(errResp)
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
				xaiResponseChunk := &xai_responses2.ResponseChunk{}
				err = sonic.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), xaiResponseChunk)
				if err != nil {
					slog.WarnContext(ctx, "unable to unmarshal xai response chunk", slog.String("data", line), slog.Any("error", err))
					continue
				}
				out <- xaiResponseChunk.ToNativeResponseChunk()
			}
		}
	}()

	return out, nil
}
