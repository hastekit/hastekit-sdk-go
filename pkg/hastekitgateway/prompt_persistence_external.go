package hastekitgateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/internal/utils"
)

type ExternalPromptPersistence struct {
	Endpoint  string
	projectID uuid.UUID
	name      string
	label     string
}

func NewExternalPromptPersistence(endpoint string, projectID uuid.UUID, name string, label string) *ExternalPromptPersistence {
	return &ExternalPromptPersistence{
		Endpoint:  endpoint,
		projectID: projectID,
		name:      name,
		label:     label,
	}
}

// PromptVersion represents a version of a prompt template
type PromptVersion struct {
	ID            uuid.UUID `json:"id" db:"id"`
	PromptID      uuid.UUID `json:"prompt_id" db:"prompt_id"`
	Version       int       `json:"version" db:"version"`
	Template      string    `json:"template" db:"template"`
	CommitMessage string    `json:"commit_message" db:"commit_message"`
	Label         *string   `json:"label,omitempty" db:"label"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// PromptVersionWithPrompt combines a prompt version with its prompt information
type PromptVersionWithPrompt struct {
	PromptVersion
	PromptName string `json:"prompt_name" db:"prompt_name"`
}

func (p *ExternalPromptPersistence) LoadPrompt(ctx context.Context) (string, error) {
	// Read the prompt from file
	url := fmt.Sprintf("%s/api/agent-server/prompts/%s/label/%s?project_id=%s", p.Endpoint, p.name, p.label, p.projectID)

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data := Response[PromptVersionWithPrompt]{}
	if err := utils.DecodeJSON(resp.Body, &data); err != nil {
		return "", err
	}

	return data.Data.Template, nil
}
