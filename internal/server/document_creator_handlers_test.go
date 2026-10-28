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
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/testutil"
)

func TestDocumentCreatorConfigUIUsesNormalizedSavedURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("tools:\n  document_creator:\n    gotenberg:\n      url: http://gotenberg:3000\n      timeout: 42\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	response := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	dc := result["tools"].(map[string]any)["document_creator"].(map[string]any)
	gotenberg := dc["gotenberg"].(map[string]any)
	if dc["allow_remote_resources"] != false || gotenberg["url"] != cfg.Tools.DocumentCreator.Gotenberg.URL || gotenberg["timeout"] != float64(42) {
		t.Fatal(dc)
	}
}

func TestGotenbergConnectionTestUsesSavedSettingsAndReportsSecurity(t *testing.T) {
	var verified atomic.Bool
	verified.Store(true)
	saved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("connection test mutated sidecar")
		}
		if r.URL.Path == "/health" {
			_, _ = io.WriteString(w, `{"status":"up"}`)
			return
		}
		if verified.Load() {
			testutil.ServeGotenbergDebug(w, r)
			return
		}
		http.Error(w, "private-diagnostic-marker", 404)
	}))
	defer saved.Close()
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("used a request URL instead of saved settings") }))
	defer foreign.Close()
	cfg := &config.Config{}
	cfg.Tools.DocumentCreator.Backend = "gotenberg"
	cfg.Tools.DocumentCreator.Gotenberg = config.GotenbergConfig{URL: saved.URL, Timeout: 2}
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	for _, safe := range []bool{true, false} {
		verified.Store(safe)
		req := httptest.NewRequest(http.MethodPost, "/api/document-creator/test", strings.NewReader(`{"url":"`+foreign.URL+`"}`))
		response := httptest.NewRecorder()
		handleGotenbergTest(s).ServeHTTP(response, req)
		var result map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result["reachable"] != true || result["security_verified"] != safe || result["active_backend"] != "gotenberg" {
			t.Fatal(result)
		}
		if strings.Contains(response.Body.String(), "private-diagnostic-marker") {
			t.Fatal("leaked raw diagnostic body")
		}
	}
}

func TestGotenbergConnectionTestRejectsNonAdminBearer(t *testing.T) {
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := tokens.Create("read-only fixture", []string{"read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "configured"
	cfg.Auth.SessionSecret = "fixture-session-secret"
	s := &Server{Cfg: cfg, TokenManager: tokens, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/document-creator/test", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("non-admin reached the handler") })).ServeHTTP(response, req)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=403", response.Code)
	}
}
