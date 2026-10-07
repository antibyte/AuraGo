package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/evomap"
	"aurago/internal/security"
)

type fakeEvomapServerClient struct {
	status   evomap.StatusResult
	register evomap.RegisterResponse
}

func (f fakeEvomapServerClient) Status(context.Context) (evomap.StatusResult, error) {
	if len(f.status.Raw) == 0 {
		return evomap.StatusResult{Status: "ok", Raw: json.RawMessage(`{"status":"ok"}`)}, nil
	}
	return f.status, nil
}

func (f fakeEvomapServerClient) RegisterNode(context.Context, evomap.RegisterRequest) (evomap.RegisterResponse, error) {
	return f.register, nil
}

func withFakeEvomapServerClient(t *testing.T, client evomapServerClient) {
	t.Helper()
	old := newEvomapServerClient
	newEvomapServerClient = func(config.EvomapConfig) (evomapServerClient, error) {
		return client, nil
	}
	t.Cleanup(func() { newEvomapServerClient = old })
}

func TestHandleEvomapStatusMasksSecrets(t *testing.T) {
	s := &Server{
		Cfg: &config.Config{Evomap: config.EvomapConfig{
			Enabled:    true,
			ReadOnly:   true,
			BaseURL:    "https://evomap.ai",
			NodeID:     "node-1",
			NodeSecret: "secret-node",
			APIKey:     "secret-api",
		}},
		Logger: slog.Default(),
	}
	req := httptest.NewRequest(http.MethodGet, "/api/evomap/status", nil)
	rec := httptest.NewRecorder()

	handleEvomapStatus(s).ServeHTTP(rec, req)

	raw := rec.Body.String()
	if strings.Contains(raw, "secret-node") || strings.Contains(raw, "secret-api") {
		t.Fatalf("response leaked secret: %s", raw)
	}
	var body evomapStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Status != "ready" || !body.NodeSecretConfigured || !body.APIKeyConfigured {
		t.Fatalf("unexpected status response: %#v", body)
	}
}

func TestHandleEvomapTestUsesClientWithoutSecrets(t *testing.T) {
	withFakeEvomapServerClient(t, fakeEvomapServerClient{status: evomap.StatusResult{Status: "ok", Raw: json.RawMessage(`{"status":"ok","capsules":2}`)}})
	s := &Server{Cfg: &config.Config{Evomap: config.EvomapConfig{BaseURL: "https://evomap.ai", RequestTimeoutSeconds: 1}}, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/evomap/test", nil)
	rec := httptest.NewRecorder()

	handleEvomapTest(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %#v, want ok; body=%s", body["status"], rec.Body.String())
	}
}

func TestHandleEvomapRegisterStoresSecretAndOmitsItFromResponse(t *testing.T) {
	withFakeEvomapServerClient(t, fakeEvomapServerClient{register: evomap.RegisterResponse{
		NodeID:     "node-registered",
		NodeSecret: "node-secret-from-server",
		ClaimURL:   "https://evomap.ai/claim/node-registered",
	}})
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	if err := config.WriteFileAtomic(cfgPath, []byte("evomap:\n  enabled: true\n  readonly: false\n  base_url: https://evomap.ai\ncustom_fixture: preserved\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	cfg.Evomap.Enabled = true
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tmp, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/evomap/register", nil)
	rec := httptest.NewRecorder()

	handleEvomapRegister(s).ServeHTTP(rec, req)

	raw := rec.Body.String()
	if strings.Contains(raw, "node-secret-from-server") {
		t.Fatalf("register response leaked node secret: %s", raw)
	}
	stored, err := vault.ReadSecret("evomap_node_secret")
	if err != nil {
		t.Fatalf("ReadSecret(evomap_node_secret) error = %v", err)
	}
	if stored != "node-secret-from-server" {
		t.Fatalf("stored secret = %q", stored)
	}
	if cfg.Evomap.NodeID != "" || cfg.Evomap.NodeSecret != "" {
		t.Fatal("published config snapshot mutated")
	}
	if s.ConfigSnapshot().Evomap.NodeID != "node-registered" {
		t.Fatal("node ID not published")
	}
	if s.ConfigSnapshot().Evomap.NodeSecret != "node-secret-from-server" {
		t.Fatalf("in-memory node secret was not updated")
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil || !strings.Contains(string(data), "custom_fixture: preserved") || strings.Contains(string(data), "node-secret-from-server") {
		t.Fatal("saved YAML lost unrelated values or contains a secret")
	}
}

func TestEvomapRegistrationRechecksAfterConfigSaveLock(t *testing.T) {
	cfg := &config.Config{Evomap: config.EvomapConfig{Enabled: true}}
	s := &Server{Cfg: cfg}
	s.initConfigSnapshot()
	s.CfgSaveMu.Lock()
	done := make(chan error, 1)
	go func() { _, _, _, err := cfg.RegisterEvomapNode(context.Background()); done <- err }()
	select {
	case <-done:
		t.Fatal("registration ignored config save lock")
	case <-time.After(20 * time.Millisecond):
	}
	next := *cfg
	next.Evomap.ReadOnly = true
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(&next)
	s.CfgMu.Unlock()
	s.CfgSaveMu.Unlock()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "writable") {
			t.Fatalf("revocation not honored: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("registration deadlocked")
	}
}
