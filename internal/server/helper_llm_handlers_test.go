package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
)

func TestHandleHelperLLMStatsReturnsSnapshot(t *testing.T) {
	agent.ResetHelperLLMRuntimeStats()
	t.Cleanup(agent.ResetHelperLLMRuntimeStats)

	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openrouter"
	cfg.LLM.HelperResolvedModel = "google/gemini-2.0-flash-lite"

	agent.MergeHelperLLMRuntimeStats("content_summaries", agent.HelperLLMOperationStats{
		Requests:     1,
		LLMCalls:     1,
		BatchedItems: 2,
		SavedCalls:   1,
		LastDetail:   "llm_call",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/debug/helper-llm/stats", nil)
	rec := httptest.NewRecorder()

	handleHelperLLMStats(&Server{Cfg: cfg}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		Enabled    bool                                     `json:"enabled"`
		UpdatedAt  string                                   `json:"updated_at"`
		Operations map[string]agent.HelperLLMOperationStats `json:"operations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !payload.Enabled {
		t.Fatal("expected enabled=true")
	}
	stats, ok := payload.Operations["content_summaries"]
	if !ok {
		t.Fatalf("missing content_summaries stats: %#v", payload.Operations)
	}
	if stats.Requests != 1 || stats.LLMCalls != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if stats.BatchedItems != 2 || stats.SavedCalls != 1 {
		t.Fatalf("unexpected efficiency stats: %#v", stats)
	}
	if payload.UpdatedAt == "" {
		t.Fatal("expected updated_at to be present")
	}
}

func TestHandleHelperLLMStatsMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/debug/helper-llm/stats", nil)
	rec := httptest.NewRecorder()

	handleHelperLLMStats(&Server{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
	}
}
