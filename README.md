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
- **🛑 Cancellation** - Stop in-flight runs cleanly at iteration boundaries via the run handle
- **📝 Structured Output** - JSON schema validation for reliable structured responses

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [LLM Calls](#llm-calls)
  - [Agents](#agents)
  - [AG-UI](#ag-ui)
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
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
    "github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

func main() {
    // Configure an LLM client with one or more providers.
    client := hastekit.NewLLMClient([]hastekit.ProviderConfig{
        {
            ProviderName: hastekit.ProviderOpenAI,
            ApiKeys: []*hastekit.APIKeyConfig{
                {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
            },
        },
    })

    // Bind a model, then make an LLM call.
    model := client.Model("OpenAI/gpt-4o-mini")

    resp, err := model.NewResponses(context.Background(), &responses.Request{
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

    hastekit "github.com/hastekit/hastekit-sdk-go"
    "github.com/hastekit/hastekit-sdk-go/pkg/agents"
    "github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
    "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
    "github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

func main() {
    // Configure an LLM client and bind a model.
    client := hastekit.NewLLMClient([]hastekit.ProviderConfig{
        {
            ProviderName: hastekit.ProviderOpenAI,
            ApiKeys: []*hastekit.APIKeyConfig{
                {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
            },
        },
    })

    // Create agent
    agent := hastekit.NewAgent(&hastekit.AgentConfig{
        Name:        "Assistant",
        Instruction: hastekit.NewPrompt("You are a helpful assistant."),
        LLM:         client.Model("OpenAI/gpt-4o-mini"),
        Parameters: responses.Parameters{
            Temperature: utils.Ptr(0.7),
        },
    })

    // Execute agent — returns a handle for streaming chunks + result.
    handle, err := agent.Execute(context.Background(), &agents.AgentInput{
        Message: history.Message{
            Messages: []responses.InputMessageUnion{
                responses.UserMessage("Hello! Tell me a joke."),
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Result() drains the chunk stream and returns the aggregated output.
    // For live streaming, range over handle.Chunks then call handle.Wait().
    out, err := handle.Result()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(out.Output[0].OfOutputMessage.Content[0].OfOutputText.Text)
}
```

`agent.Execute` is non-blocking and returns an `*AgentHandle`:

```go
type AgentHandle struct {
    StreamID string                          // Broker channel id for this run
    Chunks   <-chan *responses.ResponseChunk // Live chunks; channel closes when run ends
}

func (h *AgentHandle) Stop(ctx context.Context) error    // graceful cancel at next iteration
func (h *AgentHandle) Wait() (*AgentOutput, error)       // pair with manual Chunks draining
func (h *AgentHandle) Result() (*AgentOutput, error)     // drain Chunks + return output
```

## Usage

### LLM Client

`hastekit.NewLLMClient` takes a list of provider configs and returns a client.
Bind a model with `client.Model("Provider/model")` — the returned value satisfies
the `llm.Provider` interface and exposes `NewResponses`, `NewStreamingResponses`,
`NewEmbedding`, `NewSpeech`, and friends.

```go
// Single provider
client := hastekit.NewLLMClient([]hastekit.ProviderConfig{
    {
        ProviderName: hastekit.ProviderOpenAI,
        ApiKeys: []*hastekit.APIKeyConfig{
            {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
        },
    },
})

// Multiple providers — switch by changing the model string
client := hastekit.NewLLMClient([]hastekit.ProviderConfig{
    {
        ProviderName: hastekit.ProviderOpenAI,
        ApiKeys: []*hastekit.APIKeyConfig{
            {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
        },
    },
    {
        ProviderName: hastekit.ProviderAnthropic,
        ApiKeys: []*hastekit.APIKeyConfig{
            {Name: "default", APIKey: os.Getenv("ANTHROPIC_API_KEY")},
        },
    },
})

openai := client.Model("OpenAI/gpt-4o-mini")
claude := client.Model("Anthropic/claude-sonnet-4-5")
```

Provider constants: `hastekit.ProviderOpenAI`, `ProviderAnthropic`,
`ProviderGemini`, `ProviderXAI`, `ProviderBedrock`, `ProviderOllama`,
`ProviderOpenRouter`, `ProviderElevenLabs`.

### LLM Calls

#### Streaming Responses

```go
model := client.Model("OpenAI/gpt-4o-mini")

stream, err := model.NewStreamingResponses(context.Background(), &responses.Request{
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
resp, err := model.NewResponses(ctx, &responses.Request{
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

`hastekit.NewTool` turns any `func(ctx, In) (Out, error)` into a tool. The input
JSON schema is derived from the argument struct, and arguments/results are
marshalled for you:

```go
type WeatherArgs struct {
    Location string `json:"location" jsonschema_description:"City name"`
}

type Weather struct {
    TempC     float64 `json:"temp_c"`
    Condition string  `json:"condition"`
}

func getWeather(ctx context.Context, args WeatherArgs) (Weather, error) {
    // Your logic here
    return Weather{TempC: 22.5, Condition: "Sunny"}, nil
}

weatherTool := hastekit.NewTool(getWeather,
    hastekit.WithName("get_weather"), // optional; defaults to the function name
    hastekit.WithDescription[WeatherArgs, Weather]("Get current weather for a location"),
)

// Use the tool
agent := hastekit.NewAgent(&hastekit.AgentConfig{
    Name:        "Weather Assistant",
    Instruction: hastekit.NewPrompt("You help users check the weather."),
    LLM:         client.Model("OpenAI/gpt-4o-mini"),
    Tools:       []hastekit.Tool{weatherTool},
})
```

> `WithDescription`, `WithNeedsApproval`, and `WithDeferred` take explicit
> `[ArgsType, ReturnType]` type parameters; `WithName` does not.

Tools that implement the `agents.Tool` interface directly (embedding
`agents.BaseTool` and defining `Execute`) also work and can be mixed into the
same `Tools` slice.

#### Streaming Chunks and Cancellation

`agent.Execute` returns a handle. Range over `handle.Chunks` to forward live deltas (UI, SSE, logs); call `handle.Stop(ctx)` to ask the agent to stop at the next iteration boundary — it will record a "Cancelled by user" assistant turn in history and emit `run.completed` cleanly.

```go
handle, err := agent.Execute(ctx, &agents.AgentInput{
    Message: history.Message{
        Messages: []responses.InputMessageUnion{
            responses.UserMessage("Walk me through how to set up Postgres replication."),
        },
    },
})
if err != nil { log.Fatal(err) }

// Cancel after 5 seconds — the agent finishes its current step and stops gracefully.
go func() {
    time.Sleep(5 * time.Second)
    _ = handle.Stop(context.Background())
}()

for chunk := range handle.Chunks {
    if chunk.OfOutputTextDelta != nil {
        fmt.Print(chunk.OfOutputTextDelta.Delta)
    }
}

out, err := handle.Wait()
```

The `StreamID` on the handle (also returned in the `X-Stream-Id` HTTP header when serving over HTTP) lets you re-subscribe to the same broker channel — useful for resuming a stream after a page refresh, or for stopping the run from a different process.

### AG-UI

Agents are served to the browser over the [AG-UI protocol](https://github.com/ag-ui-protocol/ag-ui) — the standard event-stream protocol that frontend agent frameworks (CopilotKit, raw `@ag-ui/client`, etc.) speak. The `pkg/agui` package translates the SDK's streaming chunks into canonical AG-UI events (text messages, reasoning, tool calls, steps, human-in-the-loop interrupts) over SSE:

```go
import "github.com/hastekit/hastekit-sdk-go/pkg/agui"

// Agents register into a package-global registry when created.
hastekit.NewAgent(&hastekit.AgentConfig{
    Name: "Assistant",
    // ...
})

// AgentRegistry exposes the registered agents to the AG-UI handler.
registry := &hastekit.AgentRegistry{}

// Exposes:
//   GET  /agents                                   → registered agent names
//   POST /agents/{agent}/run                       → AG-UI run endpoint (SSE)
//   GET  /agents/{agent}/threads                   → stored conversation threads, newest first
//   GET  /agents/{agent}/threads/{thread}/messages → thread history as AG-UI messages
http.ListenAndServe(":8080", agui.NewHandler(registry))

// Or mount a single agent's run endpoint on an existing mux:
// mux.Handle("POST /assistant/run", agui.AgentHandler(agent))
```

For a zero-setup browser chat UI, the `pkg/agui/web` package embeds a ready-made [CopilotKit](https://copilotkit.ai) chat into your binary with `go:embed` — no Node toolchain or separate frontend deploy needed to *run* it:

```go
import "github.com/hastekit/hastekit-sdk-go/pkg/agui/web"

// Serves the embedded CopilotKit chat UI at / and the AG-UI protocol
// endpoints under /api/agui/*.
if err := web.Serve(":8080", &hastekit.AgentRegistry{}); err != nil {
    log.Fatal(err)
}
```

The embedded UI lists registered agents, shows a sidebar of prior conversations (select one to resume it on the same thread), streams assistant text, reasoning, and tool calls live, and renders CopilotKit's `useInterrupt` approval cards inline when a run pauses for human-in-the-loop tool approval.

Conversation listing works when the agent's persistence adapter implements `history.ThreadLister` — the SDK's built-in in-memory and file adapters both do. For adapters that can't enumerate threads, the listing endpoint answers `501` and the UI hides the picker.

The CopilotKit UI is a Vite/React app under [`pkg/agui/web/ui`](pkg/agui/web/ui); its build output is committed to `pkg/agui/web/static`, so `go build` never needs Node. Rebuild only when changing the UI source (`cd pkg/agui/web/ui && pnpm install && pnpm build`). CopilotKit v2 can't be loaded from a public ESM CDN (its dependency graph breaks esm.sh/jsDelivr), so it's bundled. To keep the embedded weight down to ~1MB (from ~17MB), the build aliases out CopilotKit's heaviest optional dependencies — the markdown renderer's Shiki/Mermaid/Cytoscape stack (swapped for a lightweight `react-markdown` shim), KaTeX's math fonts, and the dev-console web-inspector — none of which the chat needs. An offline, framework-free fallback UI is embedded at `/basic.html`.

Options (shared by `agui.NewHandler`, `agui.AgentHandler`, and `web.Serve`):

```go
web.Serve(":8080", client,
    agui.WithNamespace("user-123"), // conversation namespace (default "default")
    agui.WithSenderID("alice"),     // sender attribution (default "user")
    agui.WithFullHistory(),         // forward the client's full message list
                                    // (only for agents without persistence)
    agui.WithKeepalive(10*time.Second), // SSE keep-alive interval (default 15s)
)
```

Human-in-the-loop: when a run pauses for tool approval, the stream emits a `CUSTOM` event named `on_interrupt` (CopilotKit's `useInterrupt` convention) followed by `RUN_FINISHED` with `result.status: "paused"`. The client resumes by POSTing decisions back on the same thread under `forwardedProps.command.resume.decisions[]` (`{toolCallId, approved}`).

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
agent := hastekit.NewAgent(&hastekit.AgentConfig{
    Name:        "MCP Agent",
    Instruction: hastekit.NewPrompt("You are a helpful assistant."),
    LLM:         model,
    McpServers:  []agents.MCPToolset{mcpClient},
})
```

### Conversation History

Enable conversation memory across interactions:

```go
// Create a file-backed conversation manager
memory := hastekit.NewFileHistory("./conversations")

agent := hastekit.NewAgent(&hastekit.AgentConfig{
    Name:        "Memory Agent",
    Instruction: hastekit.NewPrompt("You are a helpful assistant."),
    LLM:         model,
    History:     memory, // Enable history
})

threadID := uuid.NewString()

// First interaction
handle, err := agent.Execute(context.Background(), &agents.AgentInput{
    Namespace: "user-123", // Bucket conversations by namespace
    ThreadID:  threadID,
    Message: history.Message{
        Messages: []responses.InputMessageUnion{
            responses.UserMessage("My name is Alice."),
        },
    },
})
out, err := handle.Result()

// Continue conversation — pass the same ThreadID to keep context.
handle, err = agent.Execute(context.Background(), &agents.AgentInput{
    Namespace: "user-123",
    ThreadID:  threadID,
    Message: history.Message{
        Messages: []responses.InputMessageUnion{
            responses.UserMessage("What's my name?"),
        },
    },
})
out, err = handle.Result()
```

### Durable Agents

Create fault-tolerant agents that survive crashes and failures:

A durable agent is a regular agent with a durable `Runtime` attached via
`hastekit.WithRuntime`. Create the runtime, build the agent, then start the
runtime; invoke agents over HTTP with `hastekit.NewHTTPHandler()`.

#### Using Restate

```go
client := hastekit.NewLLMClient([]hastekit.ProviderConfig{
    {
        ProviderName: hastekit.ProviderOpenAI,
        ApiKeys: []*hastekit.APIKeyConfig{
            {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
        },
    },
})

// Restate service bind address + Redis for streaming
rt, err := hastekit.NewRestateRuntime("0.0.0.0:9081", "localhost:6379")
if err != nil {
    log.Fatal(err)
}
broker, err := hastekit.NewRedisStreamBroker("localhost:6379")
if err != nil {
    log.Fatal(err)
}

// Create durable agent
agent := hastekit.NewAgent(&hastekit.AgentConfig{
    Name:        "DurableAgent",
    Instruction: hastekit.NewPrompt("You are a helpful assistant."),
    LLM:         client.Model("OpenAI/gpt-4o-mini"),
    History:     hastekit.NewFileHistory("./conversations"),
}, hastekit.WithRuntime(rt, broker))

// Start Restate service, then serve the invoke endpoint
rt.Start()
http.ListenAndServe(":8070", hastekit.NewHTTPHandler())

// Register deployment with Restate server
// restate deployments register http://localhost:9081
```

#### Using Temporal

```go
client := hastekit.NewLLMClient([]hastekit.ProviderConfig{
    {
        ProviderName: hastekit.ProviderOpenAI,
        ApiKeys: []*hastekit.APIKeyConfig{
            {Name: "default", APIKey: os.Getenv("OPENAI_API_KEY")},
        },
    },
})

// Temporal server endpoint + Redis for streaming
rt, err := hastekit.NewTemporalRuntime("localhost:7233", "localhost:6379")
if err != nil {
    log.Fatal(err)
}
broker, err := hastekit.NewRedisStreamBroker("localhost:6379")
if err != nil {
    log.Fatal(err)
}

// Create Temporal agent
agent := hastekit.NewAgent(&hastekit.AgentConfig{
    Name:        "TemporalAgent",
    Instruction: hastekit.NewPrompt("You are a helpful assistant."),
    LLM:         client.Model("OpenAI/gpt-4o-mini"),
}, hastekit.WithRuntime(rt, broker))

// Start the Temporal worker, then serve the invoke endpoint
rt.Start()
http.ListenAndServe(":8070", hastekit.NewHTTPHandler())
```

### Embeddings

Generate text embeddings for semantic search and RAG applications:

```go
import "github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/embeddings"

embedder := client.Model("OpenAI/text-embedding-3-small")

// Single text embedding
resp, err := embedder.NewEmbedding(context.Background(), &embeddings.Request{
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
resp, err = embedder.NewEmbedding(context.Background(), &embeddings.Request{
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
model := client.Model("OpenAI/gpt-4o-mini")

resp, err := model.NewResponses(context.Background(), &responses.Request{
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
model := client.Model("OpenAI/gpt-4o-mini")

resp, err := model.NewResponses(context.Background(), &responses.Request{
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
