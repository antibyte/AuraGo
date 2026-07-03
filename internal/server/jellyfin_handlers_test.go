package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestJellyfinStatusReportsClientInitializationDetails(t *testing.T) {
	s := &Server{
		Cfg: &config.Config{
			Jellyfin: config.JellyfinConfig{
				Enabled: true,
				Host:    "jellyfin.local",
				Port:    8096,
			},
		},
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/jellyfin/status", nil)
	rec := httptest.NewRecorder()
	handleJellyfinStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "error" {
		t.Fatalf("payload status = %#v, want error", payload["status"])
	}
	message, _ := payload["error"].(string)
	if !strings.Contains(message, "API key is required") {
		t.Fatalf("error = %q, want actionable API key detail", message)
	}
	if strings.Contains(message, "test-api-key") {
		t.Fatalf("error leaked API key value: %q", message)
	}
}
