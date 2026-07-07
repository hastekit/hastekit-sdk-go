package sdk

import (
	"context"
	"reflect"
	"runtime"
	"strings"

	json "github.com/bytedance/sonic"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/gateway/llm/responses"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type Tool = agents.Tool

type FunctionTool[T any, S any] struct {
	name          string
	description   string
	needsApproval bool
	deferred      bool
	fn            ToolFunc[T, S]
}

func (t *FunctionTool[T, S]) SetName(name string) {
	t.name = name
}

func (t *FunctionTool[T, S]) SetDescription(description string) {
	t.description = description
}

func (t *FunctionTool[T, S]) SetNeedsApproval(needsApproval bool) {
	t.needsApproval = needsApproval
}

func (t *FunctionTool[T, S]) SetDeferred(deferred bool) {
	t.deferred = deferred
}

type ToolConfig interface {
	SetName(string)
	SetDescription(string)
	SetNeedsApproval(bool)
	SetDeferred(bool)
}

type ToolFunc[T any, S any] func(ctx context.Context, in T) (S, error)

func NewTool[T any, S any](fn ToolFunc[T, S], opts ...ToolOption) *FunctionTool[T, S] {
	fnVal := reflect.ValueOf(fn)

	ft := &FunctionTool[T, S]{
		name: strings.SplitN(runtime.FuncForPC(fnVal.Pointer()).Name(), ".", 2)[1],
		fn:   fn,
	}

	for _, opt := range opts {
		opt(ft)
	}

	return ft
}

type ToolOption func(ToolConfig)

func WithName(name string) ToolOption {
	return func(cfg ToolConfig) {
		cfg.SetName(name)
	}
}

func WithDescription(desc string) ToolOption {
	return func(ft ToolConfig) {
		ft.SetDescription(desc)
	}
}

func WithNeedsApproval(needsApproval bool) ToolOption {
	return func(ft ToolConfig) {
		ft.SetNeedsApproval(needsApproval)
	}
}

func WithDeferred(deferred bool) ToolOption {
	return func(ft ToolConfig) {
		ft.SetDeferred(deferred)
	}
}

func (t *FunctionTool[T, S]) Execute(ctx context.Context, params *agents.ToolCall) (*agents.ToolCallResponse, error) {
	var in T
	err := json.Unmarshal([]byte(params.Arguments), &in)
	if err != nil {
		return nil, err
	}

	out, err := t.fn(ctx, in)
	if err != nil {
		return nil, err
	}

	s, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	return &agents.ToolCallResponse{
		FunctionCallOutputMessage: &responses.FunctionCallOutputMessage{
			ID:     params.ID,
			CallID: params.CallID,
			Output: responses.FunctionCallOutputContentUnion{
				OfString: utils.Ptr(string(s)),
			},
		},
	}, nil
}

func (t *FunctionTool[T, S]) Tool(ctx context.Context) *responses.ToolUnion {
	var in T

	return &responses.ToolUnion{
		OfFunction: &responses.FunctionTool{
			Name:        t.name,
			Description: utils.Ptr(t.description),
			Parameters:  NewOutputSchema(in),
			Strict:      utils.Ptr(false),
		},
	}
}

func (t *FunctionTool[T, S]) NeedApproval() bool {
	return t.needsApproval
}

func (t *FunctionTool[T, S]) IsDeferred() bool {
	return t.deferred
}
