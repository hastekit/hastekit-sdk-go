package tools

import (
	"context"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
)

type ImageGenerationTool struct {
	*agents.BaseTool
}

func NewImageGenerationTool() *ImageGenerationTool {
	return &ImageGenerationTool{}
}

func (t *ImageGenerationTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	return nil, nil
}

func (t *ImageGenerationTool) Tool(ctx context.Context) *responses.ToolUnion {
	return &responses.ToolUnion{OfImageGeneration: &responses.ImageGenerationTool{}}
}
