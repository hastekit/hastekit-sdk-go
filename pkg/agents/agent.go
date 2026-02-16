package agents

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/agentstate"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/constants"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("Agent")
)

type Agent struct {
	Name         string
	output       map[string]any
	history      *history.CommonConversationManager
	instruction  SystemPromptProvider
	tools        []Tool
	mcpServers   []MCPToolset
	llm          LLM
	parameters   responses.Parameters
	runtime      Runtime
	maxLoops     int
	streamBroker StreamBroker
	handoffs     []*Handoff
}

type AgentOptions struct {
	History     *history.CommonConversationManager
	Instruction SystemPromptProvider
	Parameters  responses.Parameters

	Name       string
	LLM        llm.Provider
	Output     map[string]any
	Tools      []Tool
	Handoffs   []*Handoff
	McpServers []MCPToolset
	Runtime    Runtime
	MaxLoops   *int
}

func NewAgent(opts *AgentOptions) *Agent {
	maxLoops := 50
	if opts.MaxLoops != nil && *opts.MaxLoops > 0 {
		maxLoops = *opts.MaxLoops
	}

	if opts.Output != nil {
		format := map[string]any{
			"type":   "json_schema",
			"name":   "structured_output",
			"strict": false,
			"schema": opts.Output,
		}
		opts.Parameters.Text = &responses.TextFormat{
			Format: format,
		}
	}

	if opts.History == nil {
		opts.History = history.NewConversationManager(history.NewInMemoryConversationPersistence())
	}

	return &Agent{
		Name:        opts.Name,
		output:      opts.Output,
		history:     opts.History,
		instruction: opts.Instruction,
		tools:       opts.Tools,
		mcpServers:  opts.McpServers,
		llm:         &WrappedLLM{opts.LLM},
		parameters:  opts.Parameters,
		runtime:     opts.Runtime,
		maxLoops:    maxLoops,
		handoffs:    opts.Handoffs,
	}
}

func (e *Agent) WithLLM(wrappedLLM LLM) *Agent {
	return &Agent{
		Name:         e.Name,
		output:       e.output,
		history:      e.history,
		instruction:  e.instruction,
		tools:        e.tools,
		mcpServers:   e.mcpServers,
		llm:          wrappedLLM,
		parameters:   e.parameters,
		runtime:      e.runtime,
		maxLoops:     e.maxLoops,
		streamBroker: e.streamBroker,
		handoffs:     e.handoffs,
	}
}

func (e *Agent) PrepareMCPTools(ctx context.Context, runContext map[string]any) ([]Tool, error) {
	coreTools := []Tool{}
	if e.mcpServers != nil {
		for _, mcpServer := range e.mcpServers {
			mcpTools, err := mcpServer.ListTools(ctx, runContext)
			if err != nil {
				return nil, fmt.Errorf("failed to list MCP tools: %w", err)
			}

			coreTools = append(coreTools, mcpTools...)
		}
	}

	return coreTools, nil
}

func (e *Agent) PrepareHandoffTools(ctx context.Context) []Tool {
	coreTools := []Tool{}

	if e.handoffs != nil && len(e.handoffs) > 0 {
		coreTools = append(coreTools, NewHandoffTool(&responses.ToolUnion{
			OfFunction: &responses.FunctionTool{
				Name:        "transfer_to_agent",
				Description: utils.Ptr("Transfer the conversation to another agent"),
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"agent_name": map[string]any{
							"type":        "string",
							"description": "Name of the target agent",
						},
					},
					"required": []string{"agent_name"},
				},
			},
		}))
	}

	return coreTools
}

func (e *Agent) GetRunID(ctx context.Context) string {
	return uuid.NewString()
}

type AgentInput struct {
	Namespace         string                               `json:"namespace"`
	PreviousMessageID string                               `json:"previous_message_id"`
	Messages          []responses.InputMessageUnion        `json:"messages"`
	RunContext        map[string]any                       `json:"run_context"`
	Callback          func(chunk *responses.ResponseChunk) `json:"-"`
	StreamBroker      StreamBroker                         `json:"-"`
}

// AgentOutput represents the result of agent execution
type AgentOutput struct {
	RunID            string                          `json:"run_id"`
	Status           agentstate.RunStatus            `json:"status"`
	Output           []responses.InputMessageUnion   `json:"output"`
	PendingApprovals []responses.FunctionCallMessage `json:"pending_approvals"`
}

func (e *Agent) Execute(ctx context.Context, in *AgentInput) (*AgentOutput, error) {
	ctx, span := tracer.Start(ctx, "Agent.Execute")
	defer span.End()

	return e.ExecuteWithoutTrace(ctx, in)
}

func (e *Agent) ExecuteWithoutTrace(ctx context.Context, in *AgentInput) (*AgentOutput, error) {
	if in.Callback == nil {
		in.Callback = NilCallback
	}

	// Delegate to runtime, or use default LocalRuntime if none is set
	runtime := e.runtime
	if runtime != nil {
		return runtime.Run(ctx, e, in)
	}

	// Generate a run ID
	run, err := history.NewRun(ctx, e.history, in.Namespace, in.PreviousMessageID, in.Messages)
	if err != nil {
		return &AgentOutput{Status: agentstate.RunStatusError, RunID: ""}, err
	}

	runId := run.GetMessageID()

	// TODO: what's the implication of obtaining traceid from context in case of durable execution?
	var traceid string
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		traceid = sc.TraceID().String()
	}

	// Emit run.created
	// TODO: make this a durable step to avoid resending on replays
	e.runCreated(ctx, runId, traceid, in.Callback)

	return e.ExecuteWithRun(ctx, in, in.Callback, run)
}

func (e *Agent) ExecuteWithRun(ctx context.Context, in *AgentInput, cb func(chunk *responses.ResponseChunk), run *history.ConversationRunManager) (*AgentOutput, error) {
	handoffTools := e.PrepareHandoffTools(ctx)
	tools := append(e.tools, handoffTools...)

	// Connect to MCP servers, and list the tools
	mcpTools, err := e.PrepareMCPTools(ctx, in.RunContext)
	if err != nil {
		return nil, err
	}

	// Merge MCP tools with other tools
	tools = append(tools, mcpTools...)

	// Create tool schemas for input payload
	var toolDefs []responses.ToolUnion
	if len(tools) > 0 {
		toolDefs = make([]responses.ToolUnion, len(tools))
		for idx, coreTool := range tools {
			toolDefs[idx] = *coreTool.Tool(ctx)
		}
	}

	// Load run state from meta (in-memory, no DB call)
	runId := run.GetMessageID()

	// TODO: what's the implication of obtaining traceid from context in case of durable execution?
	var traceid string
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		traceid = sc.TraceID().String()
	}

	// Collect tool rejections
	var rejectedToolCallIds []string
	if run.RunState.IsPaused() {
		if run.RunState.CurrentStep == agentstate.StepAwaitApproval {
			rejectedToolCallIds = in.Messages[0].OfFunctionCallApprovalResponse.RejectedCallIds
		}
	}

	// Get the prompt
	instruction := "You are a helpful assistant."
	if e.instruction != nil {
		instruction, err = e.instruction.GetPrompt(ctx, &Dependencies{
			RunContext: in.RunContext,
			Handoffs:   e.handoffs,
		})
		if err != nil {
			return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
		}
	}

	// Apply structured output format if configured
	parameters := e.parameters
	if e.output != nil {
		format := map[string]any{
			"type":   "json_schema",
			"name":   "structured_output",
			"strict": false,
			"schema": e.output,
		}
		parameters.Text = &responses.TextFormat{
			Format: format,
		}
	}

	finalOutput := []responses.InputMessageUnion{}

	// Main loop - driven by state machine
	for run.RunState.LoopIteration < e.maxLoops {
		switch run.RunState.NextStep() {

		case agentstate.StepCallLLM:
			convMessages, err := run.GetMessages(ctx)
			if err != nil {
				return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
			}

			resp, err := e.llm.NewStreamingResponses(ctx, &responses.Request{
				Instructions: utils.Ptr(instruction),
				Input: responses.InputUnion{
					OfInputMessageList: convMessages,
				},
				Tools:      toolDefs,
				Parameters: parameters,
			}, cb)
			if err != nil {
				return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
			}

			// Track the LLM's usage
			run.TrackUsage(resp.Usage)

			// Convert output to input messages and add to history
			inputMsgs := []responses.InputMessageUnion{}
			for _, outMsg := range resp.Output {
				inputMsg, err := outMsg.AsInput()
				if err != nil {
					slog.ErrorContext(ctx, "output msg conversion failed", slog.Any("error", err))
					return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
				}
				inputMsgs = append(inputMsgs, inputMsg)
			}

			run.AddMessages(ctx, inputMsgs, resp.Usage)
			finalOutput = append(finalOutput, inputMsgs...)

			// Extract tool calls
			toolCalls := []responses.FunctionCallMessage{}
			for _, msg := range resp.Output {
				if msg.OfFunctionCall != nil {
					toolCalls = append(toolCalls, *msg.OfFunctionCall)
				}
			}

			if len(toolCalls) == 0 {
				// No tools = done
				run.RunState.TransitionToComplete()
			} else {
				// Partition tools by approval requirement
				needsApproval, immediate := partitionByApproval(ctx, tools, toolCalls)

				// Execute immediate tools first (if any), then handle approval
				if len(immediate) > 0 {
					run.RunState.TransitionToExecuteTools(immediate)
					// Store tools needing approval for after immediate execution
					if len(needsApproval) > 0 {
						run.RunState.ToolsAwaitingApproval = needsApproval
					}
				} else if len(needsApproval) > 0 {
					// Only approval-required tools, no immediate ones
					run.RunState.TransitionToAwaitApproval(needsApproval)
				}
			}

		case agentstate.StepExecuteTools:
			// Execute pending tool calls
			var handoffFn func() (*AgentOutput, error)

			for _, toolCall := range run.RunState.PendingToolCalls {
				tool := findTool(ctx, tools, toolCall.Name)
				if tool == nil {
					slog.ErrorContext(ctx, "tool not found", slog.String("tool_name", toolCall.Name))
					continue
				}

				var toolResult *responses.FunctionCallOutputMessage

				if slices.Contains(rejectedToolCallIds, toolCall.CallID) {
					// Tool was rejected by human
					toolResult = &responses.FunctionCallOutputMessage{
						ID:     toolCall.ID,
						CallID: toolCall.CallID,
						Output: responses.FunctionCallOutputContentUnion{
							OfString: utils.Ptr("Request to call this tool has been declined"),
						},
					}
				} else if toolCall.Name == "transfer_to_agent" {
					var param map[string]any
					if err := sonic.Unmarshal([]byte(toolCall.Arguments), &param); err != nil {
						return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
					}

					for _, handoff := range e.handoffs {
						if handoff.Name == param["agent_name"] {
							toolResult = &responses.FunctionCallOutputMessage{
								ID:     toolCall.ID,
								CallID: toolCall.CallID,
								Output: responses.FunctionCallOutputContentUnion{
									OfString: utils.Ptr("Transferred to agent"),
								},
							}

							handoffFn = func() (*AgentOutput, error) {
								return handoff.Agent.ExecuteWithRun(ctx, in, cb, run)
							}
							break
						}
					}

					if handoffFn == nil {
						toolResult = &responses.FunctionCallOutputMessage{
							ID:     toolCall.ID,
							CallID: toolCall.CallID,
							Output: responses.FunctionCallOutputContentUnion{
								OfString: utils.Ptr("Failed to transfer to agent. Target agent not found."),
							},
						}
					}
				} else {
					toolResult, err = tool.Execute(ctx, &ToolCall{
						FunctionCallMessage: &toolCall,
						AgentName:           e.Name,
						Namespace:           in.Namespace,
						ConversationID:      run.GetConversationID(),
						RunContext:          in.RunContext,
					})
					if err != nil {
						return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
					}
				}

				// TODO: Make this a durable step to avoid resending
				cb(&responses.ResponseChunk{
					OfFunctionCallOutput: toolResult,
				})

				toolResultMsg := []responses.InputMessageUnion{
					{OfFunctionCallOutput: toolResult},
				}

				// Add tool result to history
				run.AddMessages(ctx, toolResultMsg, nil)
				finalOutput = append(finalOutput, toolResultMsg...)
			}

			run.RunState.ClearPendingTools()

			// Check if there are tools waiting for approval (queued during immediate execution)
			if run.RunState.HasToolsAwaitingApproval() {
				run.RunState.PromoteAwaitingToApproval()
			} else {
				run.RunState.TransitionToLLM()
			}

			if handoffFn != nil {
				return handoffFn()
			}

		case agentstate.StepAwaitApproval:
			err = run.SaveMessages(ctx, run.RunState.ToMeta(traceid))
			if err != nil {
				return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
			}

			// TODO: make this a durable step to avoid resending on replays
			e.runPaused(ctx, runId, traceid, run.RunState, cb)

			return &AgentOutput{
				RunID:            runId,
				Status:           agentstate.RunStatusPaused,
				PendingApprovals: run.RunState.PendingToolCalls,
			}, nil

		case agentstate.StepComplete:
			err = run.SaveMessages(ctx, run.RunState.ToMeta(traceid))
			if err != nil {
				return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, err
			}

			// TODO: make this a durable step to avoid resending on replays
			e.runCompleted(ctx, runId, traceid, run.RunState, cb)

			return &AgentOutput{
				RunID:  runId,
				Status: agentstate.RunStatusCompleted,
				Output: finalOutput,
			}, nil
		}
	}

	// Max loops exceeded
	return &AgentOutput{Status: agentstate.RunStatusError, RunID: runId}, fmt.Errorf("exceeded maximum loops (%d)", e.maxLoops)
}

func (e *Agent) runCreated(ctx context.Context, runId string, traceId string, cb func(chunk *responses.ResponseChunk)) error {
	cb(&responses.ResponseChunk{
		OfRunCreated: &responses.ChunkRun[constants.ChunkTypeRunCreated]{
			RunState: responses.ChunkRunData{
				Id:      runId,
				Object:  "run",
				Status:  "created",
				TraceID: traceId,
			},
		},
	})

	cb(&responses.ResponseChunk{
		OfRunInProgress: &responses.ChunkRun[constants.ChunkTypeRunInProgress]{
			RunState: responses.ChunkRunData{
				Id:      runId,
				Object:  "run",
				Status:  "in_progress",
				TraceID: traceId,
			},
		},
	})

	return nil
}

func (e *Agent) runPaused(ctx context.Context, runId string, traceId string, runState *agentstate.RunState, cb func(chunk *responses.ResponseChunk)) error {
	cb(&responses.ResponseChunk{
		OfRunPaused: &responses.ChunkRun[constants.ChunkTypeRunPaused]{
			RunState: responses.ChunkRunData{
				Id:               runId,
				Object:           "run",
				Status:           "paused",
				PendingToolCalls: runState.PendingToolCalls,
				Usage:            runState.Usage,
				TraceID:          traceId,
			},
		},
	})

	return nil
}

func (e *Agent) runCompleted(ctx context.Context, runId string, traceId string, runState *agentstate.RunState, cb func(chunk *responses.ResponseChunk)) error {
	cb(&responses.ResponseChunk{
		OfRunCompleted: &responses.ChunkRun[constants.ChunkTypeRunCompleted]{
			RunState: responses.ChunkRunData{
				Id:      runId,
				Object:  "run",
				Status:  "completed",
				Usage:   runState.Usage,
				TraceID: traceId,
			},
		},
	})
	return nil
}
