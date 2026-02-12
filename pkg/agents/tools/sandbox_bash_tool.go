package tools

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/sandbox"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	tracer = otel.Tracer("Tools")
)

type SandboxTool struct {
	*agents.BaseTool
	sandboxManager sandbox.Manager
	image          string
	env            map[string]string
}

type Input struct {
	Code string `json:"code"`
}

func NewSandboxTool(svc sandbox.Manager, image string, env map[string]string) *SandboxTool {
	return &SandboxTool{
		BaseTool: &agents.BaseTool{
			ToolUnion: responses.ToolUnion{
				OfFunction: &responses.FunctionTool{
					Name:        "execute_bash_commands",
					Description: utils.Ptr("Execute bash command and get the output"),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"code": map[string]any{
								"type":        "string",
								"description": "bash command to be executed",
							},
						},
						"required": []string{"code"},
					},
				},
			},
			RequiresApproval: false,
		},
		sandboxManager: svc,
		image:          image,
		env:            env,
	}
}

func (t *SandboxTool) Execute(ctx context.Context, params *agents.ToolCall) (*responses.FunctionCallOutputMessage, error) {
	ctx, span := tracer.Start(ctx, "SandboxTool")
	defer span.End()

	span.SetAttributes(attribute.String("args.code", params.Arguments))

	var in Input
	err := sonic.Unmarshal([]byte(params.Arguments), &in)
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for k, v := range t.env {
		env[k] = utils.TryAndParseAsTemplate(v, params.RunContext)
	}

	sb, err := t.sandboxManager.Create(ctx, &sandbox.CreateSandboxRequest{
		SessionID: params.ConversationID,
		Image:     t.image,
		AgentName: params.AgentName,
		Namespace: params.Namespace,
		Env:       env,
	})
	if err != nil {
		return nil, err
	}

	// Create a sandbox daemon client
	cli := sandbox.NewDaemonClient(sb)

	// Run bash command
	res, err := cli.RunBashCommand(ctx, &sandbox.BashExecRequest{
		Command:        in.Code,
		Args:           nil,
		Script:         "",
		TimeoutSeconds: 0,
		Workdir:        "",
		Env:            nil,
	})
	if err != nil {
		return nil, err
	}

	// Serialize the output
	txt, _ := sonic.Marshal(res)

	return &responses.FunctionCallOutputMessage{
		ID:     params.ID,
		CallID: params.CallID,
		Output: responses.FunctionCallOutputContentUnion{
			OfString: utils.Ptr(string(txt)),
		},
	}, nil
}
