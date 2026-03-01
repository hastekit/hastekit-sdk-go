package hastekitgateway

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/knowledge/vectorstores"
	"github.com/hastekit/hastekit-sdk-go/pkg/utils"
)

type ExternalKnowledgePersistence struct {
	endpoint  string
	projectId uuid.UUID
	name      string

	httpClient *http.Client
}

func NewExternalKnowledgePersistence(endpoint string, projectId uuid.UUID, name string) *ExternalKnowledgePersistence {
	return &ExternalKnowledgePersistence{
		endpoint:   endpoint,
		projectId:  projectId,
		name:       name,
		httpClient: &http.Client{},
	}
}

func (kp *ExternalKnowledgePersistence) Search(ctx context.Context, query string, limit int) ([]vectorstores.SearchResult, error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/agent-server/knowledges/by-name/%s/search", kp.endpoint, kp.name))
	if err != nil {
		return nil, err
	}

	// 2. Add all query parameters (including project_id and limit)
	q := u.Query()
	q.Set("project_id", kp.projectId.String())
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("query", query)

	u.RawQuery = q.Encode()

	resp, err := kp.httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("knowledgePersistence response code: %d", resp.StatusCode)
	}

	data := Response[[]vectorstores.SearchResult]{}
	if err := utils.DecodeJSON(resp.Body, &data); err != nil {
		return nil, err
	}

	return data.Data, nil
}
