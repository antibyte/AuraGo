package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func TestHandleYepAPITestUsesDraftProvider(t *testing.T) {
	var auth string
	upstream := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization") + r.Header.Get("X-API-Key")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{}
	cfg.YepAPI.Enabled = true
	cfg.YepAPI.BaseURL = upstream.URL
	cfg.Providers = []config.ProviderEntry{
		{ID: "saved", Type: "yepapi", APIKey: "saved-key"},
		{ID: "draft", Type: "yepapi", APIKey: "draft-key"},
	}
	cfg.YepAPI.Provider = "saved"

	s := &Server{Cfg: cfg, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/yepapi/test", strings.NewReader(`{"draft":{"yepapi":{"provider":"draft"}}}`))
	rec := httptest.NewRecorder()
	handleYepAPITest(s).ServeHTTP(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v body=%s", err, rec.Body.String())
	}
	if payload["status"] != "ok" {
		t.Fatalf("payload = %v", payload)
	}
	if !strings.Contains(auth, "draft-key") || strings.Contains(auth, "saved-key") {
		t.Fatalf("probe did not use the drafted provider key: %q", auth)
	}
	if cfg.YepAPI.Provider != "saved" {
		t.Fatalf("saved config was modified: %q", cfg.YepAPI.Provider)
	}
}
