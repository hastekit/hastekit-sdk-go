package mcpclient

import (
	"context"
	"slices"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPClient struct {
	Endpoint string            `json:"-"`
	Headers  map[string]string `json:"-"`

	Client                *client.Client `json:"-"`
	Tools                 []mcp.Tool     `json:"-"`
	Meta                  *mcp.Meta      `json:"-"`
	ToolFilter            []string       `json:"-"`
	ApprovalRequiredTools []string       `json:"-"`
}

func NewInProcessMCPServer(ctx context.Context, client *client.Client, headers map[string]any) (*MCPClient, error) {
	err := client.Start(ctx)
	if err != nil {
		return nil, err
	}

	_, err = client.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{},
		Params:  mcp.InitializeParams{},
	})
	if err != nil {
		return nil, err
	}

	tools, err := client.ListTools(ctx, mcp.ListToolsRequest{
		PaginatedRequest: mcp.PaginatedRequest{},
	})
	if err != nil {
		return nil, err
	}

	return &MCPClient{
		Tools:  tools.Tools,
		Client: client,
		Meta: &mcp.Meta{
			AdditionalFields: headers,
		},
	}, nil
}

func NewSSEClient(ctx context.Context, endpoint string, options ...McpServerOption) (*MCPClient, error) {
	srv := &MCPClient{
		Endpoint: endpoint,
	}

	for _, option := range options {
		option(srv)
	}

	return srv, nil
}

type McpServerOption func(*MCPClient)

func WithHeaders(headers map[string]string) McpServerOption {
	return func(server *MCPClient) {
		server.Headers = headers
	}
}

func WithToolFilter(toolFilter ...string) McpServerOption {
	return func(srv *MCPClient) {
		srv.ToolFilter = toolFilter
	}
}

func WithApprovalRequiredTools(tools ...string) McpServerOption {
	return func(srv *MCPClient) {
		srv.ApprovalRequiredTools = tools
	}
}

func (srv *MCPClient) GetName() string {
	return "MCPClient"
}

func (srv *MCPClient) GetClient(ctx context.Context, runContext map[string]any) (*MCPClient, error) {
	// resolve the headers with run context
	headers := map[string]string{}
	for k, v := range srv.Headers {
		headers[k] = utils.TryAndParseAsTemplate(v, runContext)
	}

	client, err := client.NewSSEMCPClient(
		srv.Endpoint,
		client.WithHeaders(headers),
	)
	if err != nil {
		return nil, err
	}

	err = client.Start(ctx)
	if err != nil {
		return nil, err
	}

	_, err = client.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{},
		Params:  mcp.InitializeParams{},
	})
	if err != nil {
		return nil, err
	}

	tools, err := client.ListTools(ctx, mcp.ListToolsRequest{
		PaginatedRequest: mcp.PaginatedRequest{},
	})
	if err != nil {
		return nil, err
	}

	return &MCPClient{
		Endpoint:              srv.Endpoint,
		Headers:               headers,
		Client:                client,
		Tools:                 tools.Tools,
		Meta:                  srv.Meta,
		ToolFilter:            srv.ToolFilter,
		ApprovalRequiredTools: srv.ApprovalRequiredTools,
	}, nil
}

func (srv *MCPClient) GetTools(opts ...McpServerOption) []agents.Tool {
	mcpTools := []agents.Tool{}

	for _, o := range opts {
		o(srv)
	}

	for _, tool := range srv.Tools {
		if len(srv.ToolFilter) > 0 && !slices.Contains(srv.ToolFilter, tool.Name) {
			continue
		}
		requiresApproval := false
		if len(srv.ApprovalRequiredTools) > 0 && slices.Contains(srv.ApprovalRequiredTools, tool.Name) {
			requiresApproval = true
		}

		mcpTools = append(mcpTools, NewMcpTool(tool, srv.Client, srv.Meta, requiresApproval))
	}

	return mcpTools
}

func (srv *MCPClient) ListTools(ctx context.Context, runContext map[string]any) ([]agents.Tool, error) {
	cli, err := srv.GetClient(ctx, runContext)
	if err != nil {
		return nil, err
	}

	return cli.GetTools(), nil
}
