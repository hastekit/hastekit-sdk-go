package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/prompts"
	"github.com/hastekit/hastekit-sdk-go/pkg/hastekitgateway"
)

func (c *SDK) Prompt(prompt string) *prompts.SimplePrompt {
	return prompts.New(prompt)
}

func (c *SDK) RemotePrompt(name, label string) *prompts.SimplePrompt {
	return prompts.NewWithLoader(hastekitgateway.NewExternalPromptPersistence(c.endpoint, c.projectId, name, label))
}

func (c *SDK) CustomPrompt(loader prompts.PromptLoader) *prompts.SimplePrompt {
	return prompts.NewWithLoader(loader)
}
