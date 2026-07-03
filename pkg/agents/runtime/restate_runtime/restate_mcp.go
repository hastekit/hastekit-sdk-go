package restate_runtime

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/genai"
	restate "github.com/restatedev/sdk-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	return restate.Run(t.restateCtx, func(runCtx restate.RunContext) (*agents.ToolCallResponse, error) {
		// GenAI execute_tool span, created inside restate.Run so it fires
		// exactly once (on real execution) and never on replay.
		ctx, span := tracer.Start(runCtx, genai.OpExecuteTool+" "+params.Name)
		defer span.End()
		span.SetAttributes(
			attribute.String(genai.AttrOperationName, genai.OpExecuteTool),
			attribute.String(genai.AttrToolName, params.Name),
			attribute.String(genai.AttrToolCallID, params.CallID),
			attribute.String(genai.AttrToolArguments, params.Arguments),
		)
		if t.BaseTool != nil && t.ToolUnion.OfFunction != nil && t.ToolUnion.OfFunction.Description != nil {
			span.SetAttributes(attribute.String(genai.AttrToolDescription, *t.ToolUnion.OfFunction.Description))
		}

		resp, err := t.callTool(ctx, params)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else if resp != nil && resp.FunctionCallOutputMessage != nil {
			if out, mErr := sonic.Marshal(resp.Output); mErr == nil {
				span.SetAttributes(attribute.String(genai.AttrToolResult, string(out)))
			}
		}

		return resp, err
	}, restate.WithName("MCPToolCall"))
}

// callTool invokes the MCP tool on the wrapped toolset.
func (t *RestateMCPTool) callTool(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
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
}
