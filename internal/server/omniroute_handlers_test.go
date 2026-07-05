package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestOmniRouteStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/omniroute/status", nil)
	rec := httptest.NewRecorder()

	handleOmniRouteStatus(s).ServeHTTP(rec, req)

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

func TestOmniRouteStatusReportsMissingInitialPassword(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.OmniRoute.Enabled = true
	s.Cfg.OmniRoute.Mode = "managed"
	s.Cfg.OmniRoute.URL = "http://127.0.0.1:20128"

	req := httptest.NewRequest(http.MethodGet, "/api/omniroute/status", nil)
	rec := httptest.NewRecorder()

	handleOmniRouteStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "setup_required") || !strings.Contains(rec.Body.String(), "initial password") {
		t.Fatalf("body = %s, want setup_required missing initial password message", rec.Body.String())
	}
}

func TestOmniRouteStartExternalModeDoesNotReportSidecarStart(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.OmniRoute.Enabled = true
	s.Cfg.OmniRoute.Mode = "external"
	s.Cfg.OmniRoute.ExternalBaseURL = "https://omniroute.example.test/v1"

	req := httptest.NewRequest(http.MethodPost, "/api/omniroute/start", nil)
	rec := httptest.NewRecorder()

	handleOmniRouteStart(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "starting") || strings.Contains(rec.Body.String(), "sidecar is starting") {
		t.Fatalf("body = %s, want external mode no-op status", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "external mode") {
		t.Fatalf("body = %s, want external mode message", rec.Body.String())
	}
}

func TestEnsureOmniRouteSecretsGeneratesBackendSecretsButRequiresInitialPassword(t *testing.T) {
	const masterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	vault, err := security.NewVault(masterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	cfg := &config.Config{}
	cfg.OmniRoute.Enabled = true
	cfg.OmniRoute.Mode = "managed"
	s := &Server{Cfg: cfg, Vault: vault}

	err = s.ensureOmniRouteSecrets(cfg)
	if err == nil || !strings.Contains(err.Error(), "initial password") {
		t.Fatalf("ensureOmniRouteSecrets() error = %v, want missing initial password", err)
	}

	for key, value := range map[string]string{
		"omniroute_jwt_secret":       cfg.OmniRoute.JWTSecret,
		"omniroute_api_key_secret":   cfg.OmniRoute.APIKeySecret,
		"omniroute_ws_bridge_secret": cfg.OmniRoute.WSBridgeSecret,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("%s was not generated on config", key)
		}
		got, err := vault.ReadSecret(key)
		if err != nil {
			t.Fatalf("vault.ReadSecret(%q) error = %v", key, err)
		}
		if got != value {
			t.Fatalf("vault secret %q = %q, want generated value", key, got)
		}
	}

	cfg.OmniRoute.InitialPassword = "initial-admin-password"
	if err := s.ensureOmniRouteSecrets(cfg); err != nil {
		t.Fatalf("ensureOmniRouteSecrets() after initial password error = %v", err)
	}
	if _, err := tools.ResolveOmniRouteSidecarConfig(cfg, false); err != nil {
		t.Fatalf("ResolveOmniRouteSidecarConfig() after ensureOmniRouteSecrets error = %v", err)
	}
}

func TestOmniRouteTestAcceptsPatchAndReportsSetupRequired(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	body := strings.NewReader(`{"omniroute":{"enabled":true,"mode":"managed","url":"http://127.0.0.1:20128"}}`)

	req := httptest.NewRequest(http.MethodPost, "/api/omniroute/test", body)
	rec := httptest.NewRecorder()

	handleOmniRouteTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "setup_required") {
		t.Fatalf("body = %s, want setup_required", rec.Body.String())
	}
}

func TestOmniRoutePatchAppliesFieldsAndNormalizesMode(t *testing.T) {
	cfg := &config.Config{}

	body := strings.NewReader(`{"omniroute":{"enabled":true,"mode":"external","external_base_url":"https://omniroute.example.test/v1","port":20129,"host_port":20130,"data_volume":"omni_data","memory_mb":768}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/omniroute/test", body)
	rec := httptest.NewRecorder()

	if !applyOmniRoutePatch(rec, req, cfg) {
		t.Fatalf("applyOmniRoutePatch returned false; status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !cfg.OmniRoute.Enabled {
		t.Fatal("omniroute.enabled = false, want true")
	}
	if cfg.OmniRoute.Mode != "external" {
		t.Fatalf("mode = %q, want external", cfg.OmniRoute.Mode)
	}
	if cfg.OmniRoute.ExternalBaseURL != "https://omniroute.example.test/v1" {
		t.Fatalf("external_base_url = %q, want patched URL", cfg.OmniRoute.ExternalBaseURL)
	}
	if cfg.OmniRoute.Port != 20129 || cfg.OmniRoute.HostPort != 20130 {
		t.Fatalf("port/host_port = %d/%d, want 20129/20130", cfg.OmniRoute.Port, cfg.OmniRoute.HostPort)
	}
	if cfg.OmniRoute.DataVolume != "omni_data" {
		t.Fatalf("data_volume = %q, want omni_data", cfg.OmniRoute.DataVolume)
	}
	if cfg.OmniRoute.MemoryMB != 768 {
		t.Fatalf("memory_mb = %d, want 768", cfg.OmniRoute.MemoryMB)
	}
}

func TestOmniRouteStatusHTMLURLUsesRequestHostForLoopbackBrowserURL(t *testing.T) {
	payload := map[string]interface{}{"url": "http://127.0.0.1:20128"}
	req := httptest.NewRequest(http.MethodGet, "http://dockge.local:8088/api/omniroute/status", nil)

	omniRouteRewriteBrowserURL(req, payload)

	if payload["url"] != "http://dockge.local:20128" {
		t.Fatalf("url = %v, want request host with OmniRoute port", payload["url"])
	}
}
