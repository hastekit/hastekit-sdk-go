package main

import (
	"context"
	"fmt"
	"log"

	hastekit "github.com/hastekit/hastekit-sdk-go"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

func main() {
	client, err := hastekit.NewWithLegacyOptions(&hastekit.LegacyClientOptions{
		ServerConfig: hastekit.ServerConfig{
			Endpoint:   "https://app.hastekit.ai",
			VirtualKey: "sk-uno-DhzpiCES0X4EMht6G5Reh-fyxp-DpxSYjFfloPYshp0",
			OrgName:    "techinscribed",
		},
		//ProviderConfigs: []gateway.ProviderConfig{
		//	{
		//		ProviderName:  llm.ProviderNameOpenAI,
		//		BaseURL:       "https://app.hastekit.ai/api/gateway/openai",
		//		CustomHeaders: nil,
		//		ApiKeys: []*gateway.APIKeyConfig{
		//			{
		//				ProviderName: llm.ProviderNameOpenAI,
		//				APIKey:       "sk-uno-DhzpiCES0X4EMht6G5Reh-fyxp-DpxSYjFfloPYshp0",
		//				Name:         "key",
		//			},
		//		},
		//	},
		//	{
		//		ProviderName:  llm.ProviderNameAnthropic,
		//		BaseURL:       "https://app.hastekit.ai/api/gateway/anthropic/v1",
		//		CustomHeaders: nil,
		//		ApiKeys: []*gateway.APIKeyConfig{
		//			{
		//				ProviderName: llm.ProviderNameAnthropic,
		//				APIKey:       "sk-uno-DhzpiCES0X4EMht6G5Reh-fyxp-DpxSYjFfloPYshp0",
		//				Name:         "key",
		//			},
		//		},
		//	},
		//},
	})

	stream, err := client.NewStreamingResponses(
		context.Background(),
		&responses.Request{
			//Model: "OpenAI/gpt-4.1-mini",
			Model:        "Anthropic/claude-haiku-4-5",
			Instructions: utils.Ptr("You are helpful assistant. You greet user with a light-joke"),
			Input: responses.InputUnion{
				OfString: utils.Ptr("Hello!"),
			},
			Parameters: responses.Parameters{
				Temperature: utils.Ptr(0.2),
				ExtraFields: map[string]any{
					"additional_headers": map[string]any{
						"X-Custom-Header": "Custom Header Value",
					},
				},
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
