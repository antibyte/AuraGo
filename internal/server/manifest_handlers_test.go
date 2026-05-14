package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestManifestStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/manifest/status", nil)
	rec := httptest.NewRecorder()

	handleManifestStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestManifestStatusReportsMissingSecrets(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Manifest.Enabled = true
	s.Cfg.Manifest.Mode = "managed"
	s.Cfg.Manifest.URL = "http://127.0.0.1:2099"

	req := httptest.NewRequest(http.MethodGet, "/api/manifest/status", nil)
	rec := httptest.NewRecorder()

	handleManifestStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "setup_required") || !strings.Contains(rec.Body.String(), "postgres password") {
		t.Fatalf("body = %s, want setup_required missing secret message", rec.Body.String())
	}
}

func TestManifestStartRequiresPost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/manifest/start", nil)
	rec := httptest.NewRecorder()

	handleManifestStart(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want 405", rec.Code)
	}
}

func TestManifestStartExternalModeDoesNotReportSidecarStart(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Manifest.Enabled = true
	s.Cfg.Manifest.Mode = "external"
	s.Cfg.Manifest.ExternalBaseURL = "https://manifest.example.test/v1"

	req := httptest.NewRequest(http.MethodPost, "/api/manifest/start", nil)
	rec := httptest.NewRecorder()

	handleManifestStart(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "starting") || strings.Contains(rec.Body.String(), "sidecars are starting") {
		t.Fatalf("body = %s, want external mode no-op status", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "external mode") {
		t.Fatalf("body = %s, want external mode message", rec.Body.String())
	}
}

func TestEnsureManifestSecretsCreatesManagedSidecarSecrets(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	cfg := &config.Config{}
	cfg.Manifest.Enabled = true
	cfg.Manifest.Mode = "managed"
	s := &Server{Cfg: cfg, Vault: vault}

	if err := s.ensureManifestSecrets(cfg); err != nil {
		t.Fatalf("ensureManifestSecrets() error = %v", err)
	}

	if strings.TrimSpace(cfg.Manifest.PostgresPassword) == "" {
		t.Fatal("PostgresPassword was not generated")
	}
	if strings.TrimSpace(cfg.Manifest.BetterAuthSecret) == "" {
		t.Fatal("BetterAuthSecret was not generated")
	}
	for key, want := range map[string]string{
		"manifest_postgres_password":  cfg.Manifest.PostgresPassword,
		"manifest_better_auth_secret": cfg.Manifest.BetterAuthSecret,
	} {
		got, err := vault.ReadSecret(key)
		if err != nil {
			t.Fatalf("vault.ReadSecret(%q) error = %v", key, err)
		}
		if got != want {
			t.Fatalf("vault secret %q = %q, want generated value", key, got)
		}
	}
	if _, err := tools.ResolveManifestSidecarConfig(cfg, false); err != nil {
		t.Fatalf("ResolveManifestSidecarConfig() after ensureManifestSecrets error = %v", err)
	}
}

func TestEnsureManifestSecretsDoesNotDeadlockWhenRuntimeConfigLocked(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Enabled = true
	cfg.Manifest.Mode = "managed"
	s := &Server{Cfg: cfg}

	s.CfgMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- s.ensureManifestSecrets(cfg)
	}()

	var err error
	timedOut := false
	select {
	case err = <-done:
	case <-time.After(200 * time.Millisecond):
		timedOut = true
	}
	s.CfgMu.Unlock()

	if timedOut {
		<-done
		t.Fatal("ensureManifestSecrets deadlocked while runtime config lock was already held")
	}
	if err != nil {
		t.Fatalf("ensureManifestSecrets() error = %v", err)
	}
}

func TestManifestTestAcceptsPatchAndReportsSetupRequired(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	body := strings.NewReader(`{"manifest":{"enabled":true,"mode":"managed","url":"http://127.0.0.1:2099"}}`)

	req := httptest.NewRequest(http.MethodPost, "/api/manifest/test", body)
	rec := httptest.NewRecorder()

	handleManifestTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "setup_required") {
		t.Fatalf("body = %s, want setup_required", rec.Body.String())
	}
}

func TestManifestTestRejectsInvalidJSONWithoutStatusBody(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/manifest/test", strings.NewReader(`{"manifest":`))
	rec := httptest.NewRecorder()

	handleManifestTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "setup_required") || strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("body = %s, want only invalid payload error", rec.Body.String())
	}
}

func TestManifestStatusHTMLURLUsesRequestHostForLoopbackBrowserURL(t *testing.T) {
	payload := map[string]interface{}{"url": "http://127.0.0.1:2099"}
	req := httptest.NewRequest(http.MethodGet, "http://dockge.local:8088/api/manifest/status", nil)

	manifestRewriteBrowserURL(req, payload)

	if payload["url"] != "http://dockge.local:2099" {
		t.Fatalf("url = %v, want request host with Manifest port", payload["url"])
	}
}

func TestManifestBrowserBaseURLForRequestUsesRequestHost(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Enabled = true
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = "http://127.0.0.1:2099"
	cfg.Manifest.Host = "127.0.0.1"
	cfg.Manifest.Port = 2099
	cfg.Manifest.HostPort = 2099
	cfg.Manifest.PostgresPassword = "pg-secret"
	cfg.Manifest.BetterAuthSecret = "better-auth-secret"

	req := httptest.NewRequest(http.MethodPost, "http://192.168.6.43:8088/api/manifest/start", nil)

	got := manifestBrowserBaseURLForRequest(&Server{Cfg: cfg}, cfg, req)
	if got != "http://192.168.6.43:2099" {
		t.Fatalf("manifestBrowserBaseURLForRequest() = %q, want request host Manifest URL", got)
	}
}
