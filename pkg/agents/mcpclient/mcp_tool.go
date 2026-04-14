package mcpclient

import (
	"context"
	"errors"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	tracer = otel.Tracer("MCPTool")
)

type McpTool struct {
	*agents.BaseTool
	Client *client.Client `json:"-"`
	Meta   *mcp.Meta      `json:"-"`
}

func NewMcpTool(t mcp.Tool, cli *client.Client, Meta *mcp.Meta, requiresApproval bool, deferred bool) *McpTool {
	inputSchema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	inputSchemaBytes, err := sonic.Marshal(t.InputSchema)
	if err == nil {
		_ = sonic.Unmarshal(inputSchemaBytes, &inputSchema)
	}

	outputSchema := map[string]any{}
	outputSchemaBytes, err := t.RawOutputSchema.MarshalJSON()
	if err == nil {
		_ = sonic.Unmarshal(outputSchemaBytes, &outputSchema)
	}

	return &McpTool{
		BaseTool: &agents.BaseTool{
			RequiresApproval: requiresApproval,
			Deferred:         deferred,
			ToolUnion: responses.ToolUnion{
				OfFunction: &responses.FunctionTool{
					Name:        t.Name,
					Description: utils.Ptr(t.Description),
					Parameters:  inputSchema,
					Strict:      utils.Ptr(false),
				},
			},
		},
		Client: cli,
		Meta:   Meta,
	}
}

func (c *McpTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	ctx, span := tracer.Start(ctx, "McpTool: "+params.Name)
	defer span.End()

	span.SetAttributes(attribute.String("input", params.Arguments))

	var args map[string]any
	if params.Arguments != "" {
		err := sonic.Unmarshal([]byte(params.Arguments), &args)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("output", err.Error()))
			return &agents.ToolCallResponse{
				FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
					ID:     params.ID,
					CallID: params.CallID,
					Output: responses.FunctionCallOutputContentUnion{
						OfString: utils.Ptr(err.Error()),
					},
				},
			}, nil
		}
	}

	// Call the MCP tool
	res, err := c.Client.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{},
		Params: mcp.CallToolParams{
			Name:      params.Name,
			Arguments: args,
			Meta:      c.Meta,
		},
	})
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("output", err.Error()))
		return &agents.ToolCallResponse{
			FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
				ID:     params.ID,
				CallID: params.CallID,
				Output: responses.FunctionCallOutputContentUnion{
					OfString: utils.Ptr(err.Error()),
				},
			},
		}, nil
	}

	// Return the tool result
	for _, r := range res.Content {
		switch r.(type) {
		case mcp.TextContent:
			out := &responses.FunctionCallOutputMessage{
				ID:     params.ID,
				CallID: params.CallID,
				Output: responses.FunctionCallOutputContentUnion{
					OfString: utils.Ptr(r.(mcp.TextContent).Text),
				},
			}
			outStr, _ := sonic.Marshal(out)
			span.SetAttributes(attribute.String("output", string(outStr)))
			return &agents.ToolCallResponse{FunctionCallOutputMessage: out}, nil
		}
	}

	err = errors.New("missing mcp tool result")
	span.RecordError(err)
	return nil, err
}

// LazyMcpTool holds a cached tool schema but defers MCP connection to Execute() time.
// This allows ListTools() to return tool definitions without establishing a live connection.
type LazyMcpTool struct {
	*agents.BaseTool
	endpoint        string
	transportType   string
	resolvedHeaders map[string]string
	meta            *mcp.Meta
	toolName        string
}

func NewLazyMcpTool(t mcp.Tool, endpoint, transportType string, resolvedHeaders map[string]string, meta *mcp.Meta, requiresApproval bool, deferred bool) *LazyMcpTool {
	inputSchema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	inputSchemaBytes, err := sonic.Marshal(t.InputSchema)
	if err == nil {
		_ = sonic.Unmarshal(inputSchemaBytes, &inputSchema)
	}

	outputSchema := map[string]any{}
	outputSchemaBytes, err := t.RawOutputSchema.MarshalJSON()
	if err == nil {
		_ = sonic.Unmarshal(outputSchemaBytes, &outputSchema)
	}

	return &LazyMcpTool{
		BaseTool: &agents.BaseTool{
			RequiresApproval: requiresApproval,
			Deferred:         deferred,
			ToolUnion: responses.ToolUnion{
				OfFunction: &responses.FunctionTool{
					Name:        t.Name,
					Description: utils.Ptr(t.Description),
					Parameters:  inputSchema,
					Strict:      utils.Ptr(false),
				},
			},
		},
		endpoint:        endpoint,
		transportType:   transportType,
		resolvedHeaders: resolvedHeaders,
		meta:            meta,
		toolName:        t.Name,
	}
}

func (c *LazyMcpTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	ctx, span := tracer.Start(ctx, "LazyMcpTool: "+params.Name)
	defer span.End()

	span.SetAttributes(attribute.String("input", params.Arguments))

	var args map[string]any
	if params.Arguments != "" {
		err := sonic.Unmarshal([]byte(params.Arguments), &args)
		if err != nil {
			span.RecordError(err)
			return &agents.ToolCallResponse{
				FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
					ID:     params.ID,
					CallID: params.CallID,
					Output: responses.FunctionCallOutputContentUnion{
						OfString: utils.Ptr(err.Error()),
					},
				},
			}, nil
		}
	}

	// Get a connection from the pool (or create a new one)
	cli, err := globalPool.Checkout(ctx, c.endpoint, c.transportType, c.resolvedHeaders)
	if err != nil {
		span.RecordError(err)
		return &agents.ToolCallResponse{
			FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
				ID:     params.ID,
				CallID: params.CallID,
				Output: responses.FunctionCallOutputContentUnion{
					OfString: utils.Ptr(err.Error()),
				},
			},
		}, nil
	}

	// Call the MCP tool directly by name — no ListTools needed
	res, err := cli.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{},
		Params: mcp.CallToolParams{
			Name:      params.Name,
			Arguments: args,
			Meta:      c.meta,
		},
	})
	if err != nil {
		// Connection might be dead — remove from pool and retry once
		globalPool.Remove(c.endpoint, c.transportType, c.resolvedHeaders)
		cli, retryErr := globalPool.Checkout(ctx, c.endpoint, c.transportType, c.resolvedHeaders)
		if retryErr != nil {
			span.RecordError(err)
			return &agents.ToolCallResponse{
				FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
					ID:     params.ID,
					CallID: params.CallID,
					Output: responses.FunctionCallOutputContentUnion{
						OfString: utils.Ptr(err.Error()),
					},
				},
			}, nil
		}
		res, err = cli.CallTool(ctx, mcp.CallToolRequest{
			Request: mcp.Request{},
			Params: mcp.CallToolParams{
				Name:      params.Name,
				Arguments: args,
				Meta:      c.meta,
			},
		})
		if err != nil {
			span.RecordError(err)
			return &agents.ToolCallResponse{
				FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
					ID:     params.ID,
					CallID: params.CallID,
					Output: responses.FunctionCallOutputContentUnion{
						OfString: utils.Ptr(err.Error()),
					},
				},
			}, nil
		}
	}

	// Return the tool result
	for _, r := range res.Content {
		switch r.(type) {
		case mcp.TextContent:
			out := &responses.FunctionCallOutputMessage{
				ID:     params.ID,
				CallID: params.CallID,
				Output: responses.FunctionCallOutputContentUnion{
					OfString: utils.Ptr(r.(mcp.TextContent).Text),
				},
			}
			outStr, _ := sonic.Marshal(out)
			span.SetAttributes(attribute.String("output", string(outStr)))
			return &agents.ToolCallResponse{FunctionCallOutputMessage: out}, nil
		}
	}

	err = errors.New("missing mcp tool result")
	span.RecordError(err)
	return nil, err
}
