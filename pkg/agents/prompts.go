package agents

import (
	"context"
)

type SystemPromptProvider interface {
	GetPrompt(ctx context.Context, data map[string]any) (string, error)
}
