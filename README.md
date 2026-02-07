# HasteKit SDK - Go

[![Go Reference](https://pkg.go.dev/badge/github.com/hastekit/hastekit-sdk-go.svg)](https://pkg.go.dev/github.com/hastekit/hastekit-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/hastekit/hastekit-sdk-go)](https://goreportcard.com/report/github.com/hastekit/hastekit-sdk-go)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A powerful Golang SDK for building AI agents and making LLM calls across multiple providers with a unified API. Switch between OpenAI, Anthropic, Gemini, and more with just a single line change.

## Features

- **🔄 Multi-Provider Support** - Unified API for OpenAI, Anthropic, Gemini, and more
- **🤖 Agent SDK** - Build sophisticated AI agents with tools, memory, and multi-step reasoning
- **👤 Human-in-the-Loop** - Integrate human feedback and approval workflows
- **🛡️ Durable Execution** - Create fault-tolerant agents with Restate or Temporal
- **🔧 Tool Calling** - Function calling and MCP (Model Context Protocol) tool integration
- **💾 Conversation History** - Maintain context across interactions with built-in persistence
- **📊 Embeddings** - Generate text embeddings for semantic search and RAG applications
- **🎨 Image Processing & Generation** - Vision capabilities and image generation tools
- **🌊 Streaming Support** - Real-time streaming responses for better UX
- **📝 Structured Output** - JSON schema validation for reliable structured responses

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [LLM Calls](#llm-calls)
  - [Agents](#agents)
  - [Tools](#tools)
  - [Conversation History](#conversation-history)
  - [Durable Agents](#durable-agents)
  - [Embeddings](#embeddings)
  - [Image Generation](#image-generation)
- [Documentation](#documentation)
- [Examples](#examples)
- [License](#license)

## Installation

```bash
go get -u github.com/hastekit/hastekit-sdk-go
```

**Requirements:**
- Go 1.25.0 or higher

## Quick Start

### Basic LLM Call

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    hastekit "github.com/hastekit/hastekit-sdk-go"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
    "github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

func main() {
    // Initialize the SDK
    client, err := hastekit.New(&hastekit.ClientOptions{
        ProviderConfigs: []gateway.ProviderConfig{
            {
                ProviderName: llm.ProviderNameOpenAI,
                ApiKeys: []*gateway.APIKeyConfig{
                    {
                        Name:   "default",
                        APIKey: os.Getenv("OPENAI_API_KEY"),
                    },
                },
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Make an LLM call
    resp, err := client.NewResponses(context.Background(), &responses.Request{
        Model:        "OpenAI/gpt-4o-mini",
        Instructions: utils.Ptr("You are a helpful assistant."),
        Input: responses.InputUnion{
            OfString: utils.Ptr("What is the capital of France?"),
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Extract the response
    for _, output := range resp.Output {
        if output.OfOutputMessage != nil {
            for _, content := range output.OfOutputMessage.Content {
                if content.OfOutputText != nil {
                    fmt.Println(content.OfOutputText.Text)
                }
            }
        }
    }
}
```

### Simple Agent

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/hastekit/hastekit-sdk-go/pkg/agents"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
    hastekit "github.com/hastekit/hastekit-sdk-go"
    "github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

func main() {
    // Initialize SDK
    client, err := hastekit.New(&hastekit.ClientOptions{
        ProviderConfigs: []gateway.ProviderConfig{
            {
                ProviderName: llm.ProviderNameOpenAI,
                ApiKeys: []*gateway.APIKeyConfig{
                    {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
                },
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create agent
    agent := client.NewAgent(&hastekit.AgentOptions{
        Name:        "Assistant",
        Instruction: client.Prompt("You are a helpful assistant."),
        LLM: client.NewLLM(hastekit.LLMOptions{
            Provider: llm.ProviderNameOpenAI,
            Model:    "gpt-4o-mini",
        }),
        Parameters: responses.Parameters{
            Temperature: utils.Ptr(0.7),
        },
    })

    // Execute agent
    out, err := agent.Execute(context.Background(), &agents.AgentInput{
        Messages: []responses.InputMessageUnion{
            responses.UserMessage("Hello! Tell me a joke."),
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Print response
    fmt.Println(out.Output[0].OfOutputMessage.Content[0].OfOutputText.Text)
}
```

## Usage

### LLM Calls

#### Streaming Responses

```go
stream, err := client.NewStreamingResponses(context.Background(), &responses.Request{
    Model: "OpenAI/gpt-4o-mini",
    Input: responses.InputUnion{
        OfString: utils.Ptr("Write a poem about coding."),
    },
})
if err != nil {
    panic(err)
}

for chunk := range stream {
    if chunk.OfOutputTextDelta != nil {
        fmt.Print(chunk.OfOutputTextDelta.Delta)
    }
}
```

#### Multi-Turn Conversations

```go
resp, err := client.NewResponses(ctx, &responses.Request{
    Model: "OpenAI/gpt-4o-mini",
    Input: responses.InputUnion{
        OfInputMessageList: responses.InputMessageList{
            {
                OfEasyInput: &responses.EasyMessage{
                    Role:    "user",
                    Content: responses.EasyInputContentUnion{OfString: utils.Ptr("Hi!")},
                },
            },
            {
                OfEasyInput: &responses.EasyMessage{
                    Role:    "assistant",
                    Content: responses.EasyInputContentUnion{OfString: utils.Ptr("Hello! How can I help?")},
                },
            },
            {
                OfEasyInput: &responses.EasyMessage{
                    Role:    "user",
                    Content: responses.EasyInputContentUnion{OfString: utils.Ptr("Tell me a joke.")},
                },
            },
        },
    },
})
```

#### Switching Providers

Simply change the model string to switch providers—your code stays the same:

```go
// OpenAI
Model: "OpenAI/gpt-4o-mini"

// Anthropic
Model: "Anthropic/claude-sonnet-4-5"

// Gemini
Model: "Gemini/gemini-2.5-flash"
```

### Agents

#### Agent with Custom Tools

```go
type GetWeatherTool struct {
    *agents.BaseTool
}

func NewGetWeatherTool() *GetWeatherTool {
    return &GetWeatherTool{
        BaseTool: &agents.BaseTool{
            ToolUnion: responses.ToolUnion{
                OfFunction: &responses.FunctionTool{
                    Name:        "get_weather",
                    Description: utils.Ptr("Get current weather for a location"),
                    Parameters: map[string]any{
                        "type": "object",
                        "properties": map[string]any{
                            "location": map[string]any{
                                "type":        "string",
                                "description": "City name",
                            },
                        },
                        "required": []string{"location"},
                    },
                },
            },
        },
    }
}

func (t *GetWeatherTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
    // Parse arguments
    args := map[string]interface{}{}
    json.Unmarshal([]byte(params.Arguments), &args)
    
    location := args["location"].(string)
    
    // Your logic here
    weatherData := fetchWeather(location)
    
    return &responses.FunctionCallOutputMessage{
        ID:     params.ID,
        CallID: params.CallID,
        Output: responses.FunctionCallOutputContentUnion{
            OfString: utils.Ptr(weatherData),
        },
    }, nil
}

// Use the tool
agent := client.NewAgent(&hastekit.AgentOptions{
    Name:        "Weather Assistant",
    Instruction: client.Prompt("You help users check the weather."),
    LLM:         client.NewLLM(hastekit.LLMOptions{
        Provider: llm.ProviderNameOpenAI,
        Model:    "gpt-4o-mini",
    }),
    Tools: []agents.Tool{
        NewGetWeatherTool(),
    },
})
```

### Tools

#### MCP Tools Integration

Connect to MCP servers for access to standardized tools:

```go
import "github.com/hastekit/hastekit-sdk-go/pkg/agents/mcpclient"

// Connect to MCP server
mcpClient, err := mcpclient.NewSSEClient(
    context.Background(), 
    "http://localhost:9001/sse",
    mcpclient.WithHeaders(map[string]string{
        "Authorization": "Bearer token",
    }),
    mcpclient.WithToolFilter("list_users", "get_user"), // Optional: filter tools
)
if err != nil {
    log.Fatal(err)
}

// Create agent with MCP tools
agent := client.NewAgent(&hastekit.AgentOptions{
    Name:        "MCP Agent",
    Instruction: client.Prompt("You are a helpful assistant."),
    LLM:         model,
    McpServers:  []agents.MCPToolset{mcpClient},
})
```

### Conversation History

Enable conversation memory across interactions:

```go
// Create conversation manager
history := client.NewConversationManager()

agent := client.NewAgent(&hastekit.AgentOptions{
    Name:        "Memory Agent",
    Instruction: client.Prompt("You are a helpful assistant."),
    LLM:         model,
    History:     history, // Enable history
})

// First interaction
out, err := agent.Execute(context.Background(), &agents.AgentInput{
    Namespace: "user-123", // Bucket conversations by namespace
    Messages: []responses.InputMessageUnion{
        responses.UserMessage("My name is Alice."),
    },
})

// Continue conversation
out, err = agent.Execute(context.Background(), &agents.AgentInput{
    Namespace:         "user-123",
    PreviousMessageID: out.RunID, // Link to previous message
    Messages: []responses.InputMessageUnion{
        responses.UserMessage("What's my name?"),
    },
})
```

### Durable Agents

Create fault-tolerant agents that survive crashes and failures:

#### Using Restate

```go
// Initialize SDK with Restate
client, err := hastekit.New(&hastekit.ClientOptions{
    ProviderConfigs: []gateway.ProviderConfig{
        {
            ProviderName: llm.ProviderNameOpenAI,
            ApiKeys: []*gateway.APIKeyConfig{
                {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
            },
        },
    },
    RestateConfig: hastekit.RestateConfig{
        Endpoint: "http://localhost:8081", // Restate server
    },
})

// Create durable agent
agent := client.NewRestateAgent(&hastekit.AgentOptions{
    Name:        "DurableAgent",
    Instruction: client.Prompt("You are a helpful assistant."),
    LLM: client.NewLLM(hastekit.LLMOptions{
        Provider: llm.ProviderNameOpenAI,
        Model:    "gpt-4o-mini",
    }),
    History: client.NewConversationManager(),
})

// Start Restate service
client.StartRestateService("0.0.0.0", "9081")

// Register deployment with Restate server
// restate deployments register http://localhost:9081
```

#### Using Temporal

```go
client, err := hastekit.New(&hastekit.ClientOptions{
    ProviderConfigs: []gateway.ProviderConfig{
        {
            ProviderName: llm.ProviderNameOpenAI,
            ApiKeys: []*gateway.APIKeyConfig{
                {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
            },
        },
    },
    TemporalConfig: hastekit.TemporalConfig{
        Endpoint: "localhost:7233", // Temporal server
    },
})

// Create Temporal agent
agent := client.NewTemporalAgent(&hastekit.AgentOptions{
    Name:        "TemporalAgent",
    Instruction: client.Prompt("You are a helpful assistant."),
    LLM: client.NewLLM(hastekit.LLMOptions{
        Provider: llm.ProviderNameOpenAI,
        Model:    "gpt-4o-mini",
    }),
})
```

### Embeddings

Generate text embeddings for semantic search and RAG applications:

```go
import "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"

// Single text embedding
resp, err := client.NewEmbedding(context.Background(), &embeddings.Request{
    Model: "OpenAI/text-embedding-3-small",
    Input: embeddings.InputUnion{
        OfString: utils.Ptr("The food was delicious"),
    },
})
if err != nil {
    log.Fatal(err)
}

// Access embedding vector
for _, data := range resp.Data {
    if data.Embedding.OfFloat != nil {
        fmt.Println("Dimensions:", len(data.Embedding.OfFloat))
        fmt.Println("Vector:", data.Embedding.OfFloat)
    }
}

// Batch embeddings
resp, err = client.NewEmbedding(context.Background(), &embeddings.Request{
    Model: "OpenAI/text-embedding-3-small",
    Input: embeddings.InputUnion{
        OfList: []string{
            "The food was delicious",
            "The service was excellent",
            "Great atmosphere",
        },
    },
})
```

### Image Generation

Process images (vision) and generate new images:

#### Image Processing (Vision)

```go
resp, err := client.NewResponses(context.Background(), &responses.Request{
    Model:        "OpenAI/gpt-4o-mini",
    Instructions: utils.Ptr("Describe this image"),
    Input: responses.InputUnion{
        OfInputMessageList: responses.InputMessageList{
            {
                OfInputMessage: &responses.InputMessage{
                    Role: constants.RoleUser,
                    Content: responses.InputContent{
                        {
                            OfInputImage: &responses.InputImageContent{
                                ImageURL: utils.Ptr("https://example.com/image.jpg"),
                                // Or use base64: "data:image/png;base64,..."
                                Detail: "auto",
                            },
                        },
                    },
                },
            },
        },
    },
})
```

#### Image Generation

```go
resp, err := client.NewResponses(context.Background(), &responses.Request{
    Model: "OpenAI/gpt-4o-mini",
    Input: responses.InputUnion{
        OfString: utils.Ptr("Generate a beautiful sunset over mountains"),
    },
    Tools: []responses.ToolUnion{
        {
            OfImageGeneration: &responses.ImageGenerationTool{},
        },
    },
})

// Process generated image
for _, output := range resp.Output {
    if output.OfImageGenerationCall != nil {
        imgCall := output.OfImageGenerationCall
        
        // Decode base64 image
        imageData, _ := base64.StdEncoding.DecodeString(imgCall.Result)
        
        // Save to file
        filename := fmt.Sprintf("image.%s", imgCall.OutputFormat)
        os.WriteFile(filename, imageData, 0644)
    }
}
```

## Documentation

- **[Full Documentation](https://docs.hastekit.ai/hastekit-sdk/introduction)** - Comprehensive guides and API reference
- **[Getting Started](https://docs.hastekit.ai/hastekit-sdk/setting-up)** - Setup and first steps
- **[Agent Guide](https://docs.hastekit.ai/hastekit-sdk/agents/simple-agent)** - Build AI agents
- **[Tool Integration](https://docs.hastekit.ai/hastekit-sdk/agents/tools/function-tools)** - Custom tools and MCP
- **[Durable Execution](https://docs.hastekit.ai/hastekit-sdk/agents/durable/restate)** - Fault-tolerant agents
- **[API Reference](https://pkg.go.dev/github.com/hastekit/hastekit-sdk-go)** - Go package documentation

## Examples

Explore complete working examples in the [documentation repository](https://github.com/hastekit/hastekit-docs/tree/master/examples):

### Responses API
- [Text Generation](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/1_text_generation)
- [Tool Calling](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/2_tool_calling)
- [Reasoning](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/3_reasoning)
- [Image Processing](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/4_image_processing)
- [Image Generation](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/5_image_generation)
- [Web Search](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/6_web_search)
- [Code Execution](https://github.com/hastekit/hastekit-docs/tree/master/examples/responses/7_code_execution)

### Agents
- [Simple Agent](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/1_simple_agent)
- [Tool Calling Agent](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/2_tool_calling_agent)
- [Multi-Turn Conversation](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/3_agent_multi_turn_conversation)
- [MCP Tools](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/4_agent_with_mcp_tools)
- [Agent as a Tool](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/5_agent_as_a_tool)
- [Human-in-the-Loop](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/6_human_in_the_loop)
- [Serving Agents](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/7_serving_agents)
- [Restate Agent](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/8_restate_agent)
- [Temporal Agent](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/9_temporal_agent)
- [Agent with Sandbox](https://github.com/hastekit/hastekit-docs/tree/master/examples/agents/10_agent_with_sandbox)

### Other
- [Embeddings](https://github.com/hastekit/hastekit-docs/tree/master/examples/embeddings/1_embeddings)
- [Speech](https://github.com/hastekit/hastekit-docs/tree/master/examples/speech/1_speech)

## Supported Providers

| Provider | Chat Completion | Streaming | Tool Calling | Embeddings | Vision | Image Generation |
|----------|:---------------:|:---------:|:------------:|:----------:|:------:|:----------------:|
| **OpenAI** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Anthropic** | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| **Gemini** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **XAI** | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |

## Architecture

```
hastekit-sdk-go/
├── pkg/
│   ├── agents/              # Agent orchestration
│   │   ├── runtime/         # Durable execution runtimes
│   │   │   ├── restate_runtime/
│   │   │   └── temporal_runtime/
│   │   ├── history/         # Conversation management
│   │   ├── mcpclient/       # MCP tool integration
│   │   ├── streambroker/    # Stream brokers (memory, Redis)
│   │   └── tools/           # Built-in tools
│   ├── gateway/             # LLM gateway
│   │   ├── llm/             # LLM request/response types
│   │   └── providers/       # Provider implementations
│   │       ├── openai/
│   │       ├── anthropic/
│   │       ├── gemini/
│   │       └── xai/
│   └── utils/               # Utilities
├── examples/                # Example applications
└── docs/                    # Documentation
```

## Contributing

We welcome contributions! Please see our [contributing guidelines](CONTRIBUTING.md) for details.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [docs.hastekit.ai](https://docs.hastekit.ai)
- **Issues**: [GitHub Issues](https://github.com/hastekit/hastekit-sdk-go/issues)
- **Discussions**: [GitHub Discussions](https://github.com/hastekit/hastekit-sdk-go/discussions)

## Related Projects

- [HasteKit Gateway](https://github.com/hastekit/hastekit-ai-gateway) - LLM Gateway with observability and agent builder
- [HasteKit Docs](https://github.com/hastekit/hastekit-docs) - Documentation and examples

---
