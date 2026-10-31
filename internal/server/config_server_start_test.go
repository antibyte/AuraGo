package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// The Config UI keeps the server start time with its restart marker, so both the
// config read and a save that needs a restart report it (Unix milliseconds).
func TestConfigResponsesReportServerStartTime(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("flows:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	started := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	s := &Server{Cfg: loaded, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Vault: vault, StartedAt: started}

	rec := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["_server_started_at"] != float64(started.UnixMilli()) {
		t.Fatalf("GET _server_started_at = %v, want %d", got["_server_started_at"], started.UnixMilli())
	}

	rec = httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"flows":{"max_parallel_runs":4}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var saved map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["needs_restart"] != true || saved["server_started_at"] != float64(started.UnixMilli()) {
		t.Fatalf("restart answer = %v, want needs_restart and server_started_at", saved)
	}

	// The diagnostics field is not a config key: echoing a read back into a save is harmless.
	rec = httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"_server_started_at":1,"flows":{"max_parallel_runs":5}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("echoed diagnostics: status = %d; body=%s", rec.Code, rec.Body.String())
	}

	// Without a known start time the field is simply absent.
	s.StartedAt = time.Time{}
	rec = httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if strings.Contains(rec.Body.String(), "_server_started_at") {
		t.Fatal("an unknown start time must not be reported")
	}
}

// Linking or revoking a Microsoft account is an admin action: a non-admin bearer token is refused.
func TestOneDriveAuthChangeRoutesRequireAdmin(t *testing.T) {
	t.Parallel()
	s, adminToken, _, readToken := newBearerSchemeTestServer(t)
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	s.registerConfigAPIRoutes(mux, nil)
	for _, route := range []string{"start", "poll", "revoke"} {
		req := httptest.NewRequest(http.MethodPost, "/api/onedrive/auth/"+route, nil)
		req.Header.Set("Authorization", "Bearer "+readToken)
		rec := httptest.NewRecorder()
		authMiddleware(s, mux).ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s with a read token: status = %d, want 403; body=%s", route, rec.Code, rec.Body.String())
		}
		req = httptest.NewRequest(http.MethodPost, "/api/onedrive/auth/"+route, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec = httptest.NewRecorder()
		authMiddleware(s, mux).ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
			t.Errorf("%s with an admin token: status = %d, want the handler to run", route, rec.Code)
		}
	}
}
