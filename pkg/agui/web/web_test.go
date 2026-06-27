package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registry map[string]*agents.Agent

func (r registry) Agent(name string) (*agents.Agent, bool) {
	a, ok := r[name]
	return a, ok
}

func (r registry) AgentNames() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	return names
}

func TestHandlerServesEmbeddedUIAndAPI(t *testing.T) {
	agent := agents.NewAgent(&agents.AgentOptions{Name: "Helper"})
	server := httptest.NewServer(Handler(registry{"Helper": agent}))
	defer server.Close()

	// Embedded UI at the root.
	res, err := http.Get(server.URL + "/")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "HasteKit"), "embedded index.html should render")

	// Protocol endpoints under /api/agui.
	res, err = http.Get(server.URL + APIPrefix + "/agents")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var payload struct {
		Agents []string `json:"agents"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&payload))
	assert.Equal(t, []string{"Helper"}, payload.Agents)

	// Thread listing reachable through the same mount.
	res, err = http.Get(server.URL + APIPrefix + "/agents/Helper/threads")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
}
