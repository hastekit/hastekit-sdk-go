package agui

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/history"
	"github.com/hastekit/hastekit-sdk-go/pkg/agents/messages"
)

// Registry is the minimal view of an SDK client the AG-UI handler
// needs. *hastekit.SDK satisfies it.
type Registry interface {
	Agent(name string) (*agents.Agent, bool)
	AgentNames() []string
}

type options struct {
	namespace   string
	senderID    string
	fullHistory bool
	keepalive   time.Duration
}

// Option configures the AG-UI handler.
type Option func(*options)

// WithNamespace sets the conversation namespace (default "default").
func WithNamespace(ns string) Option {
	return func(o *options) { o.namespace = ns }
}

// WithSenderID sets the sender attribution for messages POSTed by
// AG-UI clients (default "user").
func WithSenderID(id string) Option {
	return func(o *options) { o.senderID = id }
}

// WithFullHistory forwards the client's complete message list into
// the run instead of extracting only the new trailing turn. Use this
// when the agent has no conversation persistence and the AG-UI
// client is the source of truth for history. With persistence
// enabled (the SDK default) this would duplicate prior turns in the
// thread on every POST.
func WithFullHistory() Option {
	return func(o *options) { o.fullHistory = true }
}

// WithKeepalive sets the SSE keep-alive comment interval (default
// 15s; below the common 30-60s idle timeout of reverse proxies).
func WithKeepalive(d time.Duration) Option {
	return func(o *options) { o.keepalive = d }
}

func buildOptions(opts []Option) options {
	o := options{
		namespace: "default",
		senderID:  "user",
		keepalive: 15 * time.Second,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// NewHandler exposes every agent registered on the client over the
// AG-UI protocol:
//
//	GET  /agents                                  → {"agents": ["name", ...]}
//	POST /agents/{agent}/run                      → run the agent; SSE stream of AG-UI events
//	GET  /agents/{agent}/threads                  → stored conversation threads, newest first
//	GET  /agents/{agent}/threads/{thread}/messages → thread history as AG-UI messages
//
// The run endpoint accepts the canonical AG-UI RunAgentInput body and
// streams back the canonical event wire format, so any AG-UI client
// (CopilotKit's HttpAgent, raw @ag-ui/client, the embedded UI in
// pkg/agui/web) can point at it directly:
//
//	http.ListenAndServe(":8080", agui.NewHandler(client))
//
// The threads endpoints power conversation pickers. Listing requires
// the agent's persistence adapter to implement history.ThreadLister
// (the SDK's in-memory and file adapters do); when it doesn't, the
// listing endpoint answers 501 so clients can hide the picker.
func NewHandler(registry Registry, opts ...Option) http.Handler {
	o := buildOptions(opts)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"agents": registry.AgentNames()})
	})

	mux.HandleFunc("POST /agents/{agent}/run", func(w http.ResponseWriter, r *http.Request) {
		agent, ok := registry.Agent(r.PathValue("agent"))
		if !ok {
			writeJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		serveRun(w, r, agent, o)
	})

	mux.HandleFunc("GET /agents/{agent}/threads", func(w http.ResponseWriter, r *http.Request) {
		agent, ok := registry.Agent(r.PathValue("agent"))
		if !ok {
			writeJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		serveThreads(w, r, agent, o)
	})

	mux.HandleFunc("GET /agents/{agent}/threads/{thread}/messages", func(w http.ResponseWriter, r *http.Request) {
		agent, ok := registry.Agent(r.PathValue("agent"))
		if !ok {
			writeJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		serveThreadMessages(w, r, agent, r.PathValue("thread"), o)
	})

	return mux
}

// serveThreads lists the agent's stored threads in the handler's
// namespace, newest first. Answers 501 when the agent's persistence
// adapter can't enumerate threads.
func serveThreads(w http.ResponseWriter, r *http.Request, agent *agents.Agent, o options) {
	lister := threadLister(agent)
	if lister == nil {
		writeJSONError(w, http.StatusNotImplemented, "the agent's persistence adapter does not support thread listing")
		return
	}
	threads, err := lister.ListThreads(r.Context(), o.namespace)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "unable to list threads: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"threads": threads})
}

// serveThreadMessages returns a thread's stored history converted to
// the AG-UI message shape, ready for a client to hydrate a chat from.
func serveThreadMessages(w http.ResponseWriter, r *http.Request, agent *agents.Agent, threadID string, o options) {
	manager := agent.History()
	if manager == nil || manager.ConversationPersistenceAdapter == nil {
		writeJSONError(w, http.StatusNotImplemented, "the agent has no conversation persistence")
		return
	}
	rows, err := manager.ConversationPersistenceAdapter.LoadMessages(r.Context(), o.namespace, threadID, "")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "unable to load messages: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"threadId": threadID,
		"messages": HistoryToMessages(rows),
	})
}

// threadLister returns the agent's persistence adapter as a
// history.ThreadLister, or nil when listing isn't supported.
func threadLister(agent *agents.Agent) history.ThreadLister {
	manager := agent.History()
	if manager == nil {
		return nil
	}
	if lister, ok := manager.ConversationPersistenceAdapter.(history.ThreadLister); ok {
		return lister
	}
	return nil
}

// AgentHandler exposes a single agent's AG-UI run endpoint. Every
// POST, regardless of path, runs the agent — so it can be mounted
// anywhere on an existing mux:
//
//	mux.Handle("POST /my-agent/run", agui.AgentHandler(agent))
func AgentHandler(agent *agents.Agent, opts ...Option) http.Handler {
	o := buildOptions(opts)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "POST a RunAgentInput to run the agent")
			return
		}
		serveRun(w, r, agent, o)
	})
}

// serveRun decodes a RunAgentInput, executes the agent, and pumps the
// chunk stream through the translator onto the response as AG-UI SSE.
// RUN_STARTED is emitted first; RUN_FINISHED/RUN_ERROR is always last
// (synthesised if the stream closes without a terminal chunk).
func serveRun(w http.ResponseWriter, r *http.Request, agent *agents.Agent, o options) {
	var input RunAgentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid AG-UI request body: "+err.Error())
		return
	}
	if err := input.Validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	// runID identifies the AG-UI logical run, used in AG-UI event
	// payloads. Distinct from the broker StreamID, which Execute
	// generates per invocation.
	runID := input.RunID
	if runID == "" {
		runID = uuid.NewString()
	}

	sdkMessages := input.NewTurnSDKMessages()
	if o.fullHistory {
		sdkMessages = input.ToSDKMessages()
	}

	in := &agents.AgentInput{
		Namespace: o.namespace,
		ThreadID:  input.ThreadID,
		Message:   messages.New(o.senderID, sdkMessages),
		// Fold AG-UI context into the prompt RunContext. forwardedProps
		// and state land at top-level keys so prompt templates can
		// reach them via {{State.x}} / {{ForwardedProps.y}}.
		RunContext: map[string]any{
			"Context":        contextFromAGUI(input.Context),
			"ForwardedProps": input.ForwardedProps,
			"State":          input.State,
			"Header":         collectHeaders(r.Header),
		},
	}

	ctx := r.Context()
	handle, err := agent.Execute(ctx, in)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "unable to execute agent: "+err.Error())
		return
	}

	// SSE headers MUST be set before the first write — anything added
	// after the first flush is silently dropped.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx
	w.Header().Set("X-Stream-Id", handle.StreamID)
	w.Header().Set("X-Agui-Run-Id", runID)
	w.Header().Set("X-Agui-Thread-Id", input.ThreadID)
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	enc := NewEncoder(w)
	translator := NewTranslator(input.ThreadID, runID)

	// Emit RUN_STARTED before any chunk-derived event. Also ship a
	// CUSTOM event carrying the broker StreamID so a client that
	// wants to correlate with the SDK's streaming surface can.
	if err := enc.EncodeAll(ctx, translator.Start()); err != nil {
		return
	}
	_ = enc.Encode(ctx, &CustomEvent{
		BaseEvent: baseNow(),
		Name:      CustomNameStreamID,
		Value: map[string]any{
			"streamId": handle.StreamID,
			"runId":    runID,
			"threadId": input.ThreadID,
		},
	})

	// Keep-alive ticker keeps idle SSE connections from being reaped
	// by reverse proxies.
	keepalive := time.NewTicker(o.keepalive)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			if err := enc.Comment("keepalive"); err != nil {
				return
			}
		case chunk, ok := <-handle.Chunks:
			if !ok {
				// Stream closed without a terminal chunk. Surface the
				// run error if there is one, otherwise synthesise a
				// RUN_FINISHED so the AG-UI client doesn't hang.
				if _, err := handle.Wait(); err != nil {
					_ = enc.EncodeAll(ctx, translator.Error(err, "run_error"))
				} else {
					_ = enc.EncodeAll(ctx, translator.Finish())
				}
				return
			}
			if events := translator.Translate(chunk); len(events) > 0 {
				if err := enc.EncodeAll(ctx, events); err != nil {
					return
				}
			}
			// Run terminated — close the connection. The translator
			// has already emitted RUN_FINISHED (completed or paused).
			if chunk.OfRunCompleted != nil || chunk.OfRunPaused != nil {
				return
			}
		}
	}
}

// contextFromAGUI turns the AG-UI context list into a description→value
// map suitable for prompt template substitution.
func contextFromAGUI(items []InputContext) map[string]any {
	out := make(map[string]any, len(items))
	for _, c := range items {
		if c.Description == "" {
			continue
		}
		out[c.Description] = c.Value
	}
	return out
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func collectHeaders(headers http.Header) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if strings.HasPrefix(k, "X-") || k == "Authorization" {
			out[k] = v[0]
		}
	}
	return out
}
