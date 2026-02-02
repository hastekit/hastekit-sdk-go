package sdk

import (
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/hastekitgateway"
)

func (c *SDK) NewConversationManager(opts ...history.ConversationManagerOptions) *history.CommonConversationManager {
	return history.NewConversationManager(
		c.getConversationPersistence(),
		opts...,
	)
}

func (c *SDK) getConversationPersistence() history.ConversationPersistenceAdapter {
	if c.endpoint == "" {
		return history.NewInMemoryConversationPersistence()
	}

	return hastekitgateway.NewExternalConversationPersistence(c.endpoint, c.projectId)
}
