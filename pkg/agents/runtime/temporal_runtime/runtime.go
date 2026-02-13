package temporal_runtime

import (
	"context"
	"fmt"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"go.temporal.io/sdk/client"
)

type TemporalRuntime struct {
	client client.Client
	broker agents.StreamBroker
}

func NewTemporalRuntime(c client.Client, broker agents.StreamBroker) *TemporalRuntime {
	return &TemporalRuntime{
		client: c,
		broker: broker,
	}
}

func (r *TemporalRuntime) Run(ctx context.Context, agent *agents.Agent, in *agents.AgentInput) (*agents.AgentOutput, error) {
	if r.client == nil {
		return nil, fmt.Errorf("no temporal client available")
	}

	run, err := r.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue: "AgentWorkflowTaskQueue",
	}, agent.Name+"_AgentWorkflow", in)
	if err != nil {
		return nil, err
	}

	runID := run.GetID()

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

	// Wait for result
	var result agents.AgentOutput
	if err := run.Get(ctx, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
