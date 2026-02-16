package prompts

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("PromptManager")

type PromptLoader interface {
	// LoadPrompt loads the prompt from the source and returns it as string
	LoadPrompt(ctx context.Context) (string, error)
}

type PromptResolverFn func(string, map[string]any) (string, error)

type StringLoader struct {
	String string
}

func NewStringLoader(str string) *StringLoader {
	return &StringLoader{
		String: str,
	}
}

func (sl *StringLoader) LoadPrompt(ctx context.Context) (string, error) {
	return sl.String, nil
}

type SimplePrompt struct {
	loader   PromptLoader
	resolver PromptResolverFn
	skills   []agents.Skill
}

func New(prompt string, opts ...PromptOption) *SimplePrompt {
	return NewWithLoader(NewStringLoader(prompt), opts...)
}

func NewWithLoader(loader PromptLoader, opts ...PromptOption) *SimplePrompt {
	sp := &SimplePrompt{
		loader:   loader,
		resolver: DefaultResolver,
	}

	for _, op := range opts {
		op(sp)
	}

	return sp
}

type PromptOption func(*SimplePrompt)

func WithResolver(resolverFn PromptResolverFn) PromptOption {
	return func(sp *SimplePrompt) {
		sp.resolver = resolverFn
	}
}

func WithSkills(skills []agents.Skill) PromptOption {
	return func(sp *SimplePrompt) {
		sp.skills = skills
	}
}

func (sp *SimplePrompt) GetPrompt(ctx context.Context, deps *agents.Dependencies) (string, error) {
	ctx, span := tracer.Start(ctx, "GetPrompt")
	defer span.End()

	prompt, err := sp.loader.LoadPrompt(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	prompt += skillsToPrompts(sp.skills)
	prompt += handoffsToPrompts(deps.Handoffs)

	if deps.RunContext == nil {
		return prompt, nil
	}

	return sp.resolver(prompt, deps.RunContext)
}

func stringToTemplate(promptStr string) (*template.Template, error) {
	re := regexp.MustCompile(`{{(\w.+)}}`)
	promptStr = re.ReplaceAllString(promptStr, "{{ .$1 }}")

	return template.New("file_prompt").Parse(promptStr)
}

func DefaultResolver(prompt string, data map[string]any) (string, error) {
	tmpl, err := stringToTemplate(prompt)
	if err != nil {
		return prompt, err
	}

	return utils.ExecuteTemplate(tmpl, data)
}

func skillsToPrompts(skills []agents.Skill) string {
	if skills == nil || len(skills) == 0 {
		return ""
	}

	var p strings.Builder

	p.WriteString("\n\n" + "## Skills\n\n")
	p.WriteString("Skills contains more specialised context, instructions and scripts that you can use when it is required. They are available to you in the filesystem that you can access using the `execute_bash_commands` tool. Based on the task at hand, access relevant skill.")
	p.WriteString("<available_skills>")
	for _, skill := range skills {
		p.WriteString("<skill>")

		p.WriteString(fmt.Sprintf("<name>%s</name>", skill.Name))
		p.WriteString(fmt.Sprintf("<description>%s</description>", skill.Description))
		p.WriteString(fmt.Sprintf("<location>/skills/%s/SKILL.md</location>", skill.Name))

		p.WriteString("</skill>")
	}

	p.WriteString("</available_skills>")

	return p.String()
}

func handoffsToPrompts(handoffs []*agents.Handoff) string {
	if handoffs == nil || len(handoffs) == 0 {
		return ""
	}

	var p strings.Builder

	p.WriteString("\n\n" + "## Agents\n\n")
	p.WriteString("Agents are specialized in certain tasks or domain. Use the `transfer_to_agent` tool to delegate or transfer to the specialized agents, based on the task at hand.")
	p.WriteString("<available_agents>")
	for _, handoff := range handoffs {
		p.WriteString("<agent>")

		p.WriteString(fmt.Sprintf("<name>%s</name>", handoff.Name))
		p.WriteString(fmt.Sprintf("<description>%s</description>", handoff.Description))

		p.WriteString("</agent>")
	}
	p.WriteString("</available_agents>")

	return p.String()
}
