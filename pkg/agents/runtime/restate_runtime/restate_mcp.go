package restate_runtime

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	restate "github.com/restatedev/sdk-go"
)

type RestateMCPServer struct {
	restateCtx       restate.WorkflowContext
	wrappedMcpServer agents.MCPToolset
}

func NewRestateMCPServer(restateCtx restate.WorkflowContext, wrappedMcpServer agents.MCPToolset) *RestateMCPServer {
	return &RestateMCPServer{
		restateCtx:       restateCtx,
		wrappedMcpServer: wrappedMcpServer,
	}
}

func (t *RestateMCPServer) GetName() string {
	return ""
}

func (t *RestateMCPServer) ListTools(ctx context.Context, runContext map[string]any) ([]agents.Tool, error) {
	// ListTools uses the schema cache in MCPClient — no live connection needed on cache hit.
	toolDefs, err := restate.Run(t.restateCtx, func(ctx restate.RunContext) ([]agents.BaseTool, error) {
		mcpTools, err := t.wrappedMcpServer.ListTools(ctx, runContext)
		if err != nil {
			return nil, err
		}

		var tools []agents.BaseTool
		for _, tool := range mcpTools {
			tools = append(tools, agents.BaseTool{
				ToolUnion:        *tool.Tool(ctx),
				RequiresApproval: tool.NeedApproval(),
				Deferred:         tool.IsDeferred(),
			})
		}

		return tools, nil
	}, restate.WithName("MCPListTools"))
	if err != nil {
		return nil, err
	}

	var tools []agents.Tool
	for _, tool := range toolDefs {
		tools = append(tools, NewRestateMCPTool(t.restateCtx, t.wrappedMcpServer, runContext, tool))
	}

	return tools, nil
}

type RestateMCPTool struct {
	restateCtx       restate.WorkflowContext
	runContext       map[string]any
	wrappedMcpServer agents.MCPToolset
	*agents.BaseTool
}

func NewRestateMCPTool(restateCtx restate.WorkflowContext, wrappedMcpServer agents.MCPToolset, runContext map[string]any, baseTool agents.BaseTool) *RestateMCPTool {
	return &RestateMCPTool{
		restateCtx:       restateCtx,
		runContext:       runContext,
		wrappedMcpServer: wrappedMcpServer,
		BaseTool:         &baseTool,
	}
}

func (t *RestateMCPTool) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	// Execute via restate.Run for determinism. The underlying MCPClient.CallToolDirect
	// uses the connection pool — no ListTools call needed.
	return restate.Run(t.restateCtx, func(ctx restate.RunContext) (*agents.ToolCallResponse, error) {
		// Use CallToolDirect on the wrapped MCPToolset if it supports it,
		// otherwise fall back to ListTools + find (for non-MCPClient implementations).
		type directCaller interface {
			CallToolDirect(ctx context.Context, runContext map[string]any, params *agents.ToolCall) (*agents.ToolCallResponse, error)
		}

		if dc, ok := t.wrappedMcpServer.(directCaller); ok {
			return dc.CallToolDirect(ctx, t.runContext, params)
		}

		// Fallback: ListTools uses schema cache so this is still fast
		mcpTools, err := t.wrappedMcpServer.ListTools(ctx, t.runContext)
		if err != nil {
			return nil, err
		}
		for _, tool := range mcpTools {
			if td := tool.Tool(ctx); td != nil && td.OfFunction != nil && params.Name == td.OfFunction.Name {
				return tool.Execute(ctx, params)
			}
		}
		return nil, err
	}, restate.WithName("MCPToolCall"))
}
