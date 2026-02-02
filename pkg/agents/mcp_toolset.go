package agents

import (
	"context"
)

type MCPToolset interface {
	GetName() string
	ListTools(ctx context.Context, runContext map[string]any) ([]Tool, error)
}
