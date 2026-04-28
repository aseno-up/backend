package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/aseno/backend/internal/logger"
)

// LangfuseTracer pushes LLM generation events to LangFuse Cloud (or any
// LangFuse-compatible endpoint) so the operator gets a hosted dashboard
// with cost-per-conversation, prompt diffs, and replay — without having
// to build/host that ourselves. No-op when env keys are absent, so this
// is safe to wire unconditionally.
//
// Setup:
//  1. Create account at https://cloud.langfuse.com
//  2. Project Settings → API Keys → New
//  3. Set env: LANGFUSE_PUBLIC_KEY, LANGFUSE_SECRET_KEY (host defaults to
//     https://cloud.langfuse.com — override via LANGFUSE_HOST for self-hosted).
//  4. Restart backend. Traces appear within seconds of the next request.
type LangfuseTracer struct {
	host       string
	authHeader string
	httpClient *http.Client
	enabled    bool
	wg         sync.WaitGroup // tracks in-flight Trace goroutines for Flush()
}

// NewLangfuseTracer reads env and returns a tracer. When keys are missing
// the tracer is in no-op mode — Trace() returns immediately, no HTTP. The
// tracer never blocks the agent: events are dispatched in a goroutine.
func NewLangfuseTracer() *LangfuseTracer {
	pub := strings.TrimSpace(os.Getenv("LANGFUSE_PUBLIC_KEY"))
	sec := strings.TrimSpace(os.Getenv("LANGFUSE_SECRET_KEY"))
	// Accept both LANGFUSE_BASE_URL (LangFuse's own naming convention) and
	// LANGFUSE_HOST (older docs). Region matters: us.cloud.langfuse.com
	// vs cloud.langfuse.com — using the wrong region returns 401 even
	// with valid keys.
	host := strings.TrimSpace(os.Getenv("LANGFUSE_BASE_URL"))
	if host == "" {
		host = strings.TrimSpace(os.Getenv("LANGFUSE_HOST"))
	}
	if host == "" {
		host = "https://cloud.langfuse.com"
	}

	if pub == "" || sec == "" {
		return &LangfuseTracer{enabled: false}
	}

	auth := base64.StdEncoding.EncodeToString([]byte(pub + ":" + sec))
	return &LangfuseTracer{
		host:       strings.TrimRight(host, "/"),
		authHeader: "Basic " + auth,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		enabled:    true,
	}
}

// LangfuseGenerationEvent describes a single LLM call. Mapped to LangFuse's
// "generation" event type. SessionID is the conversation UUID so all turns
// of one matrícula appear grouped in the dashboard.
type LangfuseGenerationEvent struct {
	SessionID    string
	ClinicID     string
	Model        string
	InputTokens  int
	OutputTokens int
	Prompt       any // system + user messages
	Output       string
	StartTime    time.Time
	EndTime      time.Time
	Metadata     map[string]any
	Err          string // when LLM failed
}

// Trace dispatches the event in a goroutine so the caller never waits on
// LangFuse. Fire-and-forget — best-effort observability that must not
// degrade the user-facing path. When the tracer is disabled this is a
// cheap no-op. WaitGroup tracks in-flight goroutines so callers (eval
// tests, graceful shutdown) can Flush before exit.
func (t *LangfuseTracer) Trace(ctx context.Context, ev LangfuseGenerationEvent) {
	if t == nil || !t.enabled {
		return
	}
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.send(ctx, ev)
	}()
}

// Flush blocks until every in-flight Trace goroutine completes (or hits
// its 5s HTTP timeout). Critical for tests/CI: without it, `go test`
// exits before the HTTP POSTs finish, leaving the dashboard empty even
// when the wiring is correct. Production should call this on graceful
// shutdown so we don't drop the last few events.
func (t *LangfuseTracer) Flush() {
	if t == nil || !t.enabled {
		return
	}
	t.wg.Wait()
}

// send builds the LangFuse ingestion payload and POSTs to /api/public/ingestion.
// Any failure is logged at warning level — never propagated to the caller.
func (t *LangfuseTracer) send(_ context.Context, ev LangfuseGenerationEvent) {
	// Use a fresh background context so a cancelled request context (e.g.
	// the patient stopped waiting) doesn't drop the trace we wanted to
	// keep for debugging.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if ev.EndTime.IsZero() {
		ev.EndTime = time.Now()
	}
	if ev.StartTime.IsZero() {
		ev.StartTime = ev.EndTime
	}

	traceID := uuid.NewString()
	generationID := uuid.NewString()

	level := "DEFAULT"
	statusMessage := ""
	if ev.Err != "" {
		level = "ERROR"
		statusMessage = ev.Err
	}

	// LangFuse ingestion accepts a batch of events. Each "trace-create"
	// becomes a row in the Traces tab; "generation-create" nests under it.
	// SessionID groups all traces of one conversation in the UI.
	body := map[string]any{
		"batch": []any{
			map[string]any{
				"id":        uuid.NewString(),
				"timestamp": ev.StartTime.UTC().Format(time.RFC3339Nano),
				"type":      "trace-create",
				"body": map[string]any{
					"id":        traceID,
					"timestamp": ev.StartTime.UTC().Format(time.RFC3339Nano),
					"name":      "agent_turn",
					"sessionId": ev.SessionID,
					"userId":    ev.ClinicID,
					"metadata":  ev.Metadata,
				},
			},
			map[string]any{
				"id":        uuid.NewString(),
				"timestamp": ev.EndTime.UTC().Format(time.RFC3339Nano),
				"type":      "generation-create",
				"body": map[string]any{
					"id":            generationID,
					"traceId":       traceID,
					"name":          "llm_call",
					"startTime":     ev.StartTime.UTC().Format(time.RFC3339Nano),
					"endTime":       ev.EndTime.UTC().Format(time.RFC3339Nano),
					"model":         ev.Model,
					"input":         ev.Prompt,
					"output":        ev.Output,
					"level":         level,
					"statusMessage": statusMessage,
					"usage": map[string]any{
						"input":  ev.InputTokens,
						"output": ev.OutputTokens,
						"total":  ev.InputTokens + ev.OutputTokens,
						"unit":   "TOKENS",
					},
				},
			},
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		logger.Printf("[Langfuse] marshal failed: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.host+"/api/public/ingestion", bytes.NewReader(raw))
	if err != nil {
		logger.Printf("[Langfuse] request build failed: %v", err)
		return
	}
	req.Header.Set("Authorization", t.authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		logger.Printf("[Langfuse] post failed: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		logger.Printf("[Langfuse] response %d %s — body: %s", resp.StatusCode, resp.Status, string(respBody))
		return
	}

	// 207 Multi-Status is the SUCCESS code for LangFuse's batch ingestion.
	// Body shape: {"successes":[...], "errors":[...]}. Only log when
	// errors[] is non-empty, otherwise the success path stays quiet.
	if resp.StatusCode == 207 {
		var parsed struct {
			Errors []any `json:"errors"`
		}
		if err := json.Unmarshal(respBody, &parsed); err == nil && len(parsed.Errors) > 0 {
			logger.Printf("[Langfuse] partial ingest failure — body: %s", string(respBody))
		}
	}

	if os.Getenv("LANGFUSE_DEBUG") == "1" {
		logger.Printf("[Langfuse] %d %s — body: %s", resp.StatusCode, resp.Status, string(respBody))
	}
}
