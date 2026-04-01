package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/sandbox"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/tools"
	"github.com/hastekit/hastekit-sdk-go/pkg/hastekitgateway"
)

func (c *SDK) NewSandboxManager() sandbox.Manager {
	if c.endpoint == "" {
		return nil
	}

	return hastekitgateway.NewSandboxClient(c.endpoint, c.httpClient)
}

func (c *SDK) NewKnowledgeManager(name string) tools.KnowledgePersistence {
	if c.endpoint == "" {
		return nil
	}

	return hastekitgateway.NewExternalKnowledgePersistence(c.endpoint, c.projectId, name, c.httpClient)
}
