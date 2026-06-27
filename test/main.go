package main

import (
	"context"
	"log"
	"os"

	hastekit "github.com/hastekit/hastekit-sdk-go"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/tools"
	"github.com/hastekit/hastekit-sdk-go/pkg/agui/web"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type JokeTool struct {
	*agents.BaseTool
}

func NewJokeTool() *JokeTool {
	return &JokeTool{
		BaseTool: &agents.BaseTool{
			ToolUnion: responses.ToolUnion{
				OfFunction: &responses.FunctionTool{
					Name:        "generate_joke",
					Description: utils.Ptr("Returns a joke"),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"topic": map[string]any{
								"type":        "string",
								"description": "The topic of the joke",
							},
						},
						"required": []string{"topic"},
					},
				},
			},
			RequiresApproval: true,
			Deferred:         true,
		},
	}
}

func (t *JokeTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	return &agents.ToolCallResponse{
		FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
			ID:     params.ID,
			CallID: params.CallID,
			Output: responses.FunctionCallOutputContentUnion{
				OfString: utils.Ptr("I dont know any joke"),
			},
		},
		StateUpdates: map[string]string{},
	}, nil
}

func main() {
	client, err := hastekit.NewWithLegacyOptions(&hastekit.LegacyClientOptions{
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

	model := client.NewLLM(hastekit.LLMOptions{
		Provider: llm.ProviderNameOpenAI,
		Model:    "gpt-4.1-mini",
	})

	//mcpcli, err := mcpclient.NewClient(context.Background(), "https://api.nandi.iota.amagi.tv/mcp-dev/planner-mcp/sse", mcpclient.WithHeaders(map[string]string{
	//	"x-service-id": "planner-mcp",
	//	"x-account-id": "amg70005",
	//	"apikey":       "S3H37kn1bKADprvaqOCTYLMwTUQrVXwxUBsXKN4kdTxv9sSl",
	//}), mcpclient.WithTransport("sse"))
	//if err != nil {
	//	log.Fatal(err)
	//}

	history := client.NewConversationManager()
	agentName := "SampleAgent"
	_ = client.NewAgent(&hastekit.AgentOptions{
		Name:        agentName,
		Instruction: client.Prompt("You are helpful assistant."),
		LLM:         model,
		History:     history,
		Tools: []agents.Tool{
			NewJokeTool(),
			tools.NewImageGenerationTool(),
		},
		McpServers: []agents.MCPToolset{},
	})

	//http.ListenAndServe(":8070", client)
	web.Serve(":8070", client)

	// You can then invoke by hitting POST http://localhost:8070/?agent=SampleAgent with `agents.AgentInput` as your payload
	/*
		  curl -X POST "http://localhost:8070/?agent=SampleAgent" \
		  -H "Content-Type: application/json" \
		  -d '{
			"messages": [
			  {
				"role": "user",
				"content": "Hello!"
			  }
			]
		  }'
	*/
}
