package main

import (
	"context"
	"fmt"
	"log"
	"os"

	hastekit "github.com/hastekit/hastekit-sdk-go"
	"github.com/hastekit/hastekit-sdk-go/internal/utils"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

func main() {
	client, err := hastekit.New(&hastekit.ClientOptions{
		ProviderConfigs: []gateway.ProviderConfig{
			{
				ProviderName:  llm.ProviderNameOpenAI,
				BaseURL:       "",
				CustomHeaders: nil,
				ApiKeys: []*gateway.APIKeyConfig{
					{
						Name:   "Key 1",
						APIKey: os.Getenv("OPENAI_API_KEY"),
					},
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	stream, err := client.NewStreamingResponses(
		context.Background(),
		&responses.Request{
			Model:        "OpenAI/gpt-4.1-mini",
			Instructions: utils.Ptr("You are helpful assistant. You greet user with a light-joke"),
			Input: responses.InputUnion{
				OfString: utils.Ptr("Hello!"),
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	acc := ""
	for chunk := range stream {
		if chunk.OfOutputTextDelta != nil {
			acc += chunk.OfOutputTextDelta.Delta
		}
	}
	fmt.Println(acc)
}
