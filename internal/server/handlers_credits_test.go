package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestHandleOpenRouterCreditsIgnoresInactiveProviderEntries(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.HelperProviderType = "minimax"
	cfg.Providers = []config.ProviderEntry{
		{
			ID:      "or-unused",
			Type:    "openrouter",
			BaseURL: "https://openrouter.ai/api/v1",
			APIKey:  "sk-or-should-not-be-used",
		},
	}

	s := &Server{Cfg: cfg}
	req := httptest.NewRequest(http.MethodGet, "/api/credits", nil)
	rec := httptest.NewRecorder()

	handleOpenRouterCredits(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if available, _ := body["available"].(bool); available {
		t.Fatalf("expected credits endpoint to report unavailable, got %#v", body)
	}
	if got, _ := body["reason"].(string); got != "provider is not openrouter" {
		t.Fatalf("reason = %q, want %q", got, "provider is not openrouter")
	}
}
