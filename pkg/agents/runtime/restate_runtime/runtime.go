package restate_runtime

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/ingress"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// WorkflowInput is the input structure for the Restate workflow.
type WorkflowInput struct {
	AgentName string `json:"agent_name"`

	Namespace         string
	PreviousMessageID string
	Messages          []responses.InputMessageUnion
	RunContext        map[string]any
}

// RestateRuntime executes agents via Restate workflows for durability.
// It registers the agent in the global registry and invokes a Restate workflow
// that reconstructs the agent with RestateExecutor for crash recovery.
type RestateRuntime struct {
	client *ingress.Client
	broker agents.StreamBroker
}

// NewRestateRuntime creates a new Restate runtime.
// The agentName is used to look up the agent config inside the workflow.
func NewRestateRuntime(endpoint string, broker agents.StreamBroker) *RestateRuntime {
	client := ingress.NewClient(endpoint, restate.WithHttpClient(&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}))
	return &RestateRuntime{
		client: client,
		broker: broker,
	}
}

// Run registers the agent in the global registry and invokes the Restate workflow.
func (r *RestateRuntime) Run(ctx context.Context, agent *agents.Agent, in *agents.AgentInput) (*agents.AgentOutput, error) {
	// Invoke workflow with agent name and messages
	runID := uuid.NewString()
	input := &WorkflowInput{
		AgentName:         agent.Name,
		Namespace:         in.Namespace,
		PreviousMessageID: in.PreviousMessageID,
		Messages:          in.Messages,
		RunContext:        in.RunContext,
	}

	if r.broker != nil && in.Callback != nil {
		// Handle streaming via callback
		go func() {
			fmt.Println("Subscribing to stream for run ID:", runID)
			stream, err := r.broker.Subscribe(ctx, runID)
			if err != nil {
				fmt.Println("Error subscribing to stream for run ID:", runID, "error:", err)
				return
			}

			for chunk := range stream {
				fmt.Println("Received chunk for run ID:", runID, "chunk:", chunk)
				in.Callback(chunk)
			}
		}()
	}

	return ingress.Workflow[*WorkflowInput, *agents.AgentOutput](
		r.client,
		"AgentWorkflow",
		runID,
		"Run",
	).Request(ctx, input)
}
