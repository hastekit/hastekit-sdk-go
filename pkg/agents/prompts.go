package agents

import (
	"context"
)

type Dependencies struct {
	RunContext map[string]any
	Handoffs   []*Handoff
}

type SystemPromptProvider interface {
	GetPrompt(ctx context.Context, data *Dependencies) (string, error)
}
