package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/sandbox"
	"github.com/hastekit/hastekit-sdk-go/pkg/hastekitgateway"
)

func (c *SDK) NewSandboxManager() sandbox.Manager {
	if c.endpoint == "" {
		return nil
	}

	return hastekitgateway.NewSandboxClient(c.endpoint)
}
