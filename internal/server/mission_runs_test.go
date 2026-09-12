package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

// blockingCancelTestClient stands in for a long-running LLM turn: it blocks
// until the request context is cancelled and reports that cancellation.
type blockingCancelTestClient struct {
	started chan struct{}
}

func (c *blockingCancelTestClient) CreateChatCompletion(ctx context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	select {
	case c.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return openai.ChatCompletionResponse{}, ctx.Err()
}

func (c *blockingCancelTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("streaming is not expected in this test")
}

// TestHandleChatCompletionsMissionRunIsCancellable pins the sync-branch wiring
// end to end: a mission-tagged sync request registers its run, cancelling the
// registry entry aborts the in-flight LLM call, and the handler answers with
// the agent-error response the mission callback classifies as cancelled.
func TestHandleChatCompletionsMissionRunIsCancellable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.Model = "cancel-test-model"
	cfg.Providers = []config.ProviderEntry{{
		ID:              "main",
		Type:            "ollama",
		BaseURL:         "http://127.0.0.1:11434/v1",
		Model:           cfg.LLM.Model,
		ContextWindow:   8192,
		MaxOutputTokens: 1024,
	}}
	cfg.LLM.BaseURL = cfg.Providers[0].BaseURL
	client := &blockingCancelTestClient{started: make(chan struct{}, 1)}
	s := &Server{
		Cfg:            cfg,
		Logger:         logger,
		LLMClient:      client,
		ShortTermMem:   stm,
		HistoryManager: memory.NewEphemeralHistoryManager(),
		Registry:       tools.NewProcessRegistry(logger),
		internalToken:  "mission-cancel-test-token",
	}

	payload, err := json.Marshal(openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Count slowly from 1 to 200."}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:43210"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-FollowUp", "true")
	req.Header.Set("X-Internal-Token", s.internalToken)
	req.Header.Set("X-Mission-ID", "m_e2e")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		handleChatCompletions(s, nil).ServeHTTP(rec, req)
	}()

	select {
	case <-client.started:
	case <-done:
		t.Fatalf("handler returned before the LLM call started: status %d body %s", rec.Code, rec.Body.String())
	case <-time.After(15 * time.Second):
		t.Fatal("LLM call did not start")
	}
	if !s.missionRunTracker().cancel("m_e2e") {
		t.Fatal("mission-tagged sync request must register a cancellable run")
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("handler did not return after the run was cancelled")
	}
	if rec.Header().Get("X-Aurago-Agent-Error") != "true" {
		t.Fatalf("cancelled run must produce the agent-error response; status %d headers %v body %s", rec.Code, rec.Header(), rec.Body.String())
	}
	if !s.missionRunTracker().consumeCancelled("m_e2e") {
		t.Fatal("cancelled flag must survive handler return so the mission callback can classify the failure")
	}
	if s.missionRunTracker().cancel("m_e2e") {
		t.Fatal("registry entry must be cleared after the cancellation was consumed")
	}
}

func TestMissionRunRegistryCancelsActiveRun(t *testing.T) {
	reg := newMissionRunRegistry()
	if reg.cancel("m1") {
		t.Fatalf("cancel without an active run must return false")
	}
	ctx, release := reg.begin("m1")
	defer release()
	if ctx.Err() != nil {
		t.Fatalf("fresh run context must not be cancelled")
	}
	if !reg.cancel("m1") {
		t.Fatalf("cancel of an active run must return true")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatalf("run context was not cancelled")
	}
	if !reg.consumeCancelled("m1") {
		t.Fatalf("consumeCancelled must report the cancellation once")
	}
	if reg.consumeCancelled("m1") {
		t.Fatalf("consumeCancelled must clear the flag")
	}
}

func TestMissionRunRegistryReleaseRemovesEntry(t *testing.T) {
	reg := newMissionRunRegistry()
	_, release := reg.begin("m1")
	release()
	if reg.cancel("m1") {
		t.Fatalf("released run must not be cancellable")
	}
	if reg.consumeCancelled("m1") {
		t.Fatalf("released run must not report a cancellation")
	}
}

func TestMissionRunRegistryReleaseKeepsCancelledFlagUntilConsumed(t *testing.T) {
	reg := newMissionRunRegistry()
	_, release := reg.begin("m1")
	reg.cancel("m1")
	release()
	if !reg.consumeCancelled("m1") {
		t.Fatalf("cancelled flag must survive release so the callback can classify the failure")
	}
}

func TestMissionRunRegistrySecondBeginCancelsPrevious(t *testing.T) {
	reg := newMissionRunRegistry()
	first, releaseFirst := reg.begin("m1")
	defer releaseFirst()
	second, releaseSecond := reg.begin("m1")
	defer releaseSecond()
	if first.Err() == nil {
		t.Fatalf("starting a second run for the same mission must cancel the stale first context")
	}
	if second.Err() != nil {
		t.Fatalf("second run context must be live")
	}
	if reg.consumeCancelled("m1") {
		t.Fatalf("a superseded run is not a user cancellation")
	}
}

func TestMissionRunBaseContextWithoutMissionIsDetached(t *testing.T) {
	s := &Server{}
	ctx, release := missionRunBaseContext(s, "")
	defer release()
	if ctx != context.Background() {
		t.Fatalf("requests without X-Mission-ID must keep the detached background context")
	}
	ctx, release = missionRunBaseContext(s, "m1")
	defer release()
	if ctx == context.Background() || ctx.Err() != nil {
		t.Fatalf("mission requests must receive a live registry-derived context")
	}
	if !s.missionRunTracker().cancel("m1") {
		t.Fatalf("mission context must be registered for cancellation")
	}
}
