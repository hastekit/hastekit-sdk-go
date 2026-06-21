package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/hastekitgateway"
)

type LLMOptions struct {
	Provider llm.ProviderName
	Model    string
}

// NewLLM creates a new LLMClient that provides access to multiple LLM providers.
func (c *SDK) NewLLM(opts LLMOptions) llm.Provider {
	return gateway.NewLLMClient(
		c.getGatewayAdapter(),
		c.llmConfigs,
		gateway.WithKey(c.virtualKey),
		gateway.WithModel(opts.Provider, opts.Model),
	)
}

// setLLMClient creates a new LLMClient that provides access to multiple LLM providers.
func (c *SDK) setLLMClient() {
	var opts []gateway.LLMClientOption
	if c.virtualKey != "" {
		opts = append(opts, gateway.WithKey(c.virtualKey))
	}

	c.LLMClient = gateway.NewLLMClient(
		c.getGatewayAdapter(),
		c.llmConfigs,
		opts...,
	)
}

func (c *SDK) getGatewayAdapter() gateway.LLMGatewayAdapter {
	if c.directMode {
		return gateway.NewInternalLLMGateway(gateway.NewLLMGateway(c.llmConfigs))
	}

	return hastekitgateway.NewExternalLLMGateway(c.endpoint, c.httpClient)
}
