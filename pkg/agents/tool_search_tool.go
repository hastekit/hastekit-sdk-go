package agents

import (
	"context"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"go.opentelemetry.io/otel/attribute"
)

type ToolSearchTool struct {
	*BaseTool
	deferredTools []Tool
}

type ToolSearchInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

func NewToolSearchTool(deferredTools []Tool) *ToolSearchTool {
	return &ToolSearchTool{
		BaseTool: &BaseTool{
			ToolUnion: responses.ToolUnion{
				OfFunction: &responses.FunctionTool{
					Name:        "ToolSearch",
					Description: utils.Ptr("Fetches full schema definitions for deferred tools and activates them so they can be called."),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]string{
								"type":        "string",
								"description": "Query to find deferred tools. Use \"select:<tool_name>\" for direct selection, or keywords to search.",
							},
							"max_results": map[string]any{
								"type":        "integer",
								"description": "Maximum number of results to return. Default is 5.",
							},
						},
						"required": []string{"query"},
					},
					Strict: nil,
				},
			},
			RequiresApproval: false,
			Deferred:         false,
		},
		deferredTools: deferredTools,
	}
}

func (t *ToolSearchTool) Execute(ctx context.Context, params *ToolCall) (*ToolCallResponse, error) {
	ctx, span := tracer.Start(ctx, "ToolSearchTool")
	defer span.End()

	span.SetAttributes(attribute.String("args", params.Arguments))

	var in ToolSearchInput
	err := sonic.Unmarshal([]byte(params.Arguments), &in)
	if err != nil {
		return nil, err
	}

	toolsToActivate := []Tool{}
	toolNames := []string{}

	// Absolute selection
	if strings.HasPrefix(in.Query, "select:") {
		toolNames = strings.Split(strings.TrimPrefix(in.Query, "select:"), ",")

		for _, tool := range t.deferredTools {
			if t := tool.Tool(ctx); t.OfFunction != nil && slices.Contains(toolNames, t.OfFunction.Name) {
				toolsToActivate = append(toolsToActivate, tool)
			}
		}
	}

	return &ToolCallResponse{
		FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
			ID:     params.ID,
			CallID: params.CallID,
			Output: responses.FunctionCallOutputContentUnion{
				OfString: utils.Ptr("Activated tools: " + strings.Join(toolNames, ",")),
			},
		},
		StateUpdates: map[string]string{
			"activated_deferred_tools": strings.Join(toolNames, ","),
		},
	}, nil
}
