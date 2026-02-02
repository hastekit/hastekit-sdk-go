package hastekitgateway

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	tracer = otel.Tracer("HastekitAdapters")
)

type Response[T any] struct {
	ctx     context.Context
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    T      `json:"data"`
	Status  int    `json:"status"`
}

type ExternalConversationPersistence struct {
	Endpoint  string
	projectID uuid.UUID
}

func NewExternalConversationPersistence(endpoint string, projectID uuid.UUID) *ExternalConversationPersistence {
	return &ExternalConversationPersistence{
		Endpoint:  endpoint,
		projectID: projectID,
	}
}

// NewConversationID generates a unique ID for a conversation
func (p *ExternalConversationPersistence) NewConversationID(ctx context.Context) string {
	return uuid.NewString()
}

// NewRunID generates a unique ID for a run
func (p *ExternalConversationPersistence) NewRunID(ctx context.Context) string {
	return uuid.NewString()
}

// LoadMessages implements core.ChatHistory
func (p *ExternalConversationPersistence) LoadMessages(ctx context.Context, namespace string, previousMessageId string) ([]history.ConversationMessage, error) {
	ctx, span := tracer.Start(ctx, "ExternalConversationPersistence.LoadMessages")
	defer span.End()

	span.SetAttributes(
		attribute.String("namespace", namespace),
		attribute.String("previous_message_id", previousMessageId),
	)

	// If no previous message ID, return empty list
	if previousMessageId == "" {
		return []history.ConversationMessage{}, nil
	}

	url := fmt.Sprintf("%s/api/agent-server/messages/summary?namespace=%s&previous_message_id=%s&project_id=%s", p.Endpoint, namespace, previousMessageId, p.projectID.String())

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := Response[[]history.ConversationMessage]{}
	if err := utils.DecodeJSON(resp.Body, &data); err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Int("conversation_messages_count", len(data.Data)))

	return data.Data, nil
}

type AddMessageRequest struct {
	ProjectID         uuid.UUID                     `json:"project_id"`
	Namespace         string                        `json:"namespace"`
	MessageID         string                        `json:"message_id"`
	PreviousMessageID string                        `json:"previous_message_id"`
	Messages          []responses.InputMessageUnion `json:"messages"`
	Meta              map[string]any                `json:"meta"`
	ConversationID    string                        `json:"conversation_id"`
}

// SaveMessages implements core.ChatHistory
func (p *ExternalConversationPersistence) SaveMessages(ctx context.Context, namespace, msgId, previousMsgId, conversationId string, messages []responses.InputMessageUnion, meta map[string]any) error {
	ctx, span := tracer.Start(ctx, "ExternalConversationPersistence.SaveMessages")
	defer span.End()

	span.SetAttributes(
		attribute.String("namespace", namespace),
		attribute.String("previous_message_id", previousMsgId),
		attribute.String("conversation_id", conversationId),
		attribute.Int("messages_count", len(messages)),
	)

	// Save regular messages
	url := fmt.Sprintf("%s/api/agent-server/messages?project_id=%s", p.Endpoint, p.projectID.String())

	payload := AddMessageRequest{
		Namespace:         namespace,
		MessageID:         msgId,
		PreviousMessageID: previousMsgId,
		Messages:          messages,
		Meta:              meta,
		ConversationID:    conversationId,
	}

	payloadBytes, err := sonic.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		span.RecordError(err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err = fmt.Errorf("failed to save messages: status %d", resp.StatusCode)
		span.RecordError(err)
		return err
	}

	return nil
}

// SaveSummary
func (p *ExternalConversationPersistence) SaveSummary(ctx context.Context, namespace string, summary history.Summary) error {
	ctx, span := tracer.Start(ctx, "ExternalConversationPersistence.SaveSummary")
	defer span.End()

	url := fmt.Sprintf("%s/api/agent-server/summary?project_id=%s&namespace=%s", p.Endpoint, p.projectID.String(), namespace)

	payloadBytes, err := sonic.Marshal(summary)
	if err != nil {
		span.RecordError(err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		span.RecordError(err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		err = fmt.Errorf("failed to save messages: status %d", resp.StatusCode)
		span.RecordError(err)
		return err
	}

	return nil
}
