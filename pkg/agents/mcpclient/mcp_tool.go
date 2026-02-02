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

func NewMcpTool(t mcp.Tool, cli *client.Client, Meta *mcp.Meta, requiresApproval bool) *McpTool {
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

func (c *McpTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	ctx, span := tracer.Start(ctx, "McpTool: "+params.Name)
	defer span.End()

	span.SetAttributes(attribute.String("input", params.Arguments))

	var args map[string]any
	if params.Arguments != "" {
		err := sonic.Unmarshal([]byte(params.Arguments), &args)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("output", err.Error()))
			return &responses.FunctionCallOutputMessage{
				ID:     params.ID,
				CallID: params.CallID,
				Output: responses.FunctionCallOutputContentUnion{
					OfString: utils.Ptr(err.Error()),
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
		return &responses.FunctionCallOutputMessage{
			ID:     params.ID,
			CallID: params.CallID,
			Output: responses.FunctionCallOutputContentUnion{
				OfString: utils.Ptr(err.Error()),
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
			return out, nil
		}
	}

	err = errors.New("missing mcp tool result")
	span.RecordError(err)
	return nil, err
}
