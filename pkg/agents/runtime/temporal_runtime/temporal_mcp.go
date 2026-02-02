package temporal_runtime

import (
	"context"
	"fmt"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"go.temporal.io/sdk/workflow"
)

type TemporalMCPServer struct {
	wrappedMcpServer agents.MCPToolset
}

func NewTemporalMCPServer(wrappedMcpServer agents.MCPToolset) *TemporalMCPServer {
	return &TemporalMCPServer{
		wrappedMcpServer: wrappedMcpServer,
	}
}

func (t *TemporalMCPServer) ListTools(ctx context.Context, runContext map[string]any) ([]agents.BaseTool, error) {
	mcpTools, err := t.wrappedMcpServer.ListTools(ctx, runContext)
	if err != nil {
		return nil, err
	}

	var tools []agents.BaseTool
	for _, tool := range mcpTools {
		tools = append(tools, agents.BaseTool{
			ToolUnion:        *tool.Tool(ctx),
			RequiresApproval: tool.NeedApproval(),
		})
	}

	return tools, nil
}

func (t *TemporalMCPServer) ExecuteTool(ctx context.Context, params *agents.ToolCall, runContext map[string]any) (*responses.FunctionCallOutputMessage, error) {
	// TODO: directly call the tool without listing
	mcpTools, err := t.wrappedMcpServer.ListTools(ctx, runContext)
	if err != nil {
		return nil, err
	}

	for _, tool := range mcpTools {
		if t := tool.Tool(ctx); t != nil && t.OfFunction != nil && params.Name == t.OfFunction.Name {
			return tool.Execute(ctx, params)
		}
	}

	return nil, fmt.Errorf("no tool found with name %s", params.Name)
}

type TemporalMCPProxy struct {
	workflowCtx workflow.Context
	prefix      string
}

func NewTemporalMCPProxy(workflowCtx workflow.Context, prefix string) *TemporalMCPProxy {
	return &TemporalMCPProxy{
		workflowCtx: workflowCtx,
		prefix:      prefix,
	}
}

func (t *TemporalMCPProxy) GetName() string {
	return t.prefix
}

func (t *TemporalMCPProxy) ListTools(ctx context.Context, runContext map[string]any) ([]agents.Tool, error) {
	var toolDefs []agents.BaseTool
	err := workflow.ExecuteActivity(t.workflowCtx, t.prefix+"_ListMCPToolsActivity", runContext).Get(t.workflowCtx, &toolDefs)
	if err != nil {
		return nil, err
	}

	var toolList []agents.Tool
	for _, toolDef := range toolDefs {
		toolList = append(toolList, NewTemporalMCPToolProxy(t.workflowCtx, t.prefix, runContext, toolDef))
	}

	return toolList, nil
}

type TemporalMCPToolProxy struct {
	workflowCtx workflow.Context
	prefix      string
	runContext  map[string]any
	*agents.BaseTool
}

func NewTemporalMCPToolProxy(workflowCtx workflow.Context, prefix string, runContext map[string]any, baseTool agents.BaseTool) *TemporalMCPToolProxy {
	return &TemporalMCPToolProxy{
		workflowCtx: workflowCtx,
		prefix:      prefix,
		runContext:  runContext,
		BaseTool:    &baseTool,
	}
}

func (t *TemporalMCPToolProxy) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	var output *responses.FunctionCallOutputMessage
	err := workflow.ExecuteActivity(t.workflowCtx, t.prefix+"_ExecuteMCPToolActivity", params, t.runContext).Get(t.workflowCtx, &output)
	if err != nil {
		return nil, err
	}

	return output, nil
}
