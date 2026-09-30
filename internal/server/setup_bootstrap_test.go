package server

import (
	"aurago/internal/config"
	"aurago/internal/localllm"
	"aurago/internal/security"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bootstrapTestRemotePeer = "203.0.113.7:40000"

func setSetupBootstrapTokenForTest(s *Server, token string) {
	s.setupBootstrapMu.Lock()
	defer s.setupBootstrapMu.Unlock()
	s.setupBootstrapValue = token
}

// withSetupBootstrapToken lets tests of later setup checks pass the bootstrap
// gate the way the owner would.
func withSetupBootstrapToken(s *Server, req *http.Request) {
	setSetupBootstrapTokenForTest(s, "test-bootstrap-token")
	req.Header.Set(setupBootstrapHeader, "test-bootstrap-token")
}

// newBootstrapTestServer loads the shipped config template the way a fresh
// install does. host mirrors AURAGO_SERVER_HOST (the Docker image uses 0.0.0.0).
func newBootstrapTestServer(t *testing.T, host string) *Server {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	input, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template: %v", err)
	}
	if err := os.WriteFile(configPath, input, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("AURAGO_SERVER_HOST", host)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.ConfigPath = configPath
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	return &Server{Cfg: cfg, Logger: slog.Default(), Vault: vault}
}

func setupClaimPatch(t *testing.T) string {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"auth": map[string]interface{}{"enabled": true, "admin_password": "owner-password"},
		"providers": []interface{}{map[string]interface{}{
			"id": "main", "type": "openai", "name": "Main", "base_url": "https://api.openai.com/v1",
			"api_key": "sk-owner", "model": "gpt-test",
		}},
		"llm": map[string]interface{}{"provider": "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func postSetup(s *Server, remoteAddr, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handleSetupSave(s).ServeHTTP(rec, req)
	return rec
}

func responseCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	code, _ := payload["code"].(string)
	return code
}

func TestSetupSaveRejectsRemoteRequestWithoutBootstrapToken(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	addSetupCSRFTokenForTest(s, "csrf-a")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	for _, token := range []string{"", "wrong-token"} {
		rec := postSetup(s, bootstrapTestRemotePeer, setupClaimPatch(t), map[string]string{
			"X-CSRF-Token":       "csrf-a",
			setupBootstrapHeader: token,
		})
		if rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_bootstrap_token_required" {
			t.Fatalf("token %q: status = %d body=%s, want 403 setup_bootstrap_token_required", token, rec.Code, rec.Body.String())
		}
	}
	if s.Cfg.Auth.PasswordHash != "" {
		t.Fatal("rejected setup request still set an admin password")
	}
	if !validateSetupCSRFToken(s, "csrf-a", false) {
		t.Fatal("rejected setup request consumed the CSRF token")
	}

	rec := postSetup(s, bootstrapTestRemotePeer, setupClaimPatch(t), map[string]string{
		"X-CSRF-Token":       "csrf-a",
		setupBootstrapHeader: "bootstrap-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("setup with bootstrap token: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if s.Cfg.Auth.PasswordHash == "" {
		t.Fatal("setup with bootstrap token did not set the admin password")
	}
	if s.validSetupBootstrapToken("bootstrap-secret") {
		t.Fatal("bootstrap token stayed valid after setup completed")
	}
}

func TestSetupSaveAllowsLoopbackPeerOnLocalOnlyListener(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1")
	addSetupCSRFTokenForTest(s, "csrf-local")

	rec := postSetup(s, "127.0.0.1:50000", setupClaimPatch(t), map[string]string{"X-CSRF-Token": "csrf-local"})
	if rec.Code != http.StatusOK {
		t.Fatalf("local setup without bootstrap token: status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSetupSaveRequiresBootstrapTokenFromLoopbackWhenTunnelExposesUI(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1")
	s.Cfg.CloudflareTunnel.Enabled = true
	s.Cfg.CloudflareTunnel.ExposeWebUI = true
	addSetupCSRFTokenForTest(s, "csrf-tunnel")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	rec := postSetup(s, "127.0.0.1:50001", setupClaimPatch(t), map[string]string{"X-CSRF-Token": "csrf-tunnel"})
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_bootstrap_token_required" {
		t.Fatalf("tunnelled loopback setup: status = %d body=%s, want 403 setup_bootstrap_token_required", rec.Code, rec.Body.String())
	}
}

func TestSetupSaveRefusesToDisableAuthOnRemoteListener(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	addSetupCSRFTokenForTest(s, "csrf-b")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	body := `{"auth":{"enabled":false},"agent":{"allow_shell":true}}`
	rec := postSetup(s, bootstrapTestRemotePeer, body, map[string]string{
		"X-CSRF-Token":       "csrf-b",
		setupBootstrapHeader: "bootstrap-secret",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("disabling auth on a remote listener: status = %d body=%s, want 400", rec.Code, rec.Body.String())
	}
	if !s.Cfg.Auth.Enabled {
		t.Fatal("runtime auth was disabled despite the rejected setup save")
	}
	saved, err := config.Load(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if !saved.Auth.Enabled || saved.Agent.AllowShell {
		t.Fatalf("rejected setup save reached disk: auth.enabled=%v allow_shell=%v", saved.Auth.Enabled, saved.Agent.AllowShell)
	}
}

func TestRemoteAuthExposureUsesServerHostEnvOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1" // config.yaml value; the Docker image overrides it
	t.Setenv("AURAGO_SERVER_HOST", "0.0.0.0")
	if err := validateRemoteAuthExposure(cfg); err == nil {
		t.Fatal("auth-disabled config accepted although AURAGO_SERVER_HOST binds all interfaces")
	}
}

func TestSetupStatusReportsBootstrapRequirement(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	status := func(remoteAddr, token string) map[string]interface{} {
		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		req.RemoteAddr = remoteAddr
		if token != "" {
			req.Header.Set(setupBootstrapHeader, token)
		}
		rec := httptest.NewRecorder()
		handleSetupStatus(s).ServeHTTP(rec, req)
		var payload map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode status: %v body=%s", err, rec.Body.String())
		}
		return payload
	}

	remote := status(bootstrapTestRemotePeer, "")
	if remote["bootstrap_token_required"] != true || remote["bootstrap_token_valid"] == true {
		t.Fatalf("remote status without token = %v", remote)
	}
	if valid := status(bootstrapTestRemotePeer, "bootstrap-secret"); valid["bootstrap_token_valid"] != true {
		t.Fatalf("remote status with token = %v", valid)
	}

	local := newBootstrapTestServer(t, "127.0.0.1")
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	req.RemoteAddr = "127.0.0.1:50002"
	rec := httptest.NewRecorder()
	handleSetupStatus(local).ServeHTTP(rec, req)
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["bootstrap_token_required"] != false {
		t.Fatalf("local status = %v, want bootstrap_token_required=false", payload)
	}
}

func TestSetupStatusGeneratesBootstrapTokenForRemoteRequests(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	req.RemoteAddr = bootstrapTestRemotePeer
	handleSetupStatus(s).ServeHTTP(httptest.NewRecorder(), req)

	s.setupBootstrapMu.Lock()
	token := s.setupBootstrapValue
	s.setupBootstrapMu.Unlock()
	if len(token) < 32 {
		t.Fatalf("remote setup status did not provision a bootstrap token (got %q)", token)
	}
	if !s.validSetupBootstrapToken(token) {
		t.Fatal("provisioned bootstrap token is not accepted")
	}
}

func activeSetupBootstrapToken(s *Server) string {
	s.setupBootstrapMu.Lock()
	defer s.setupBootstrapMu.Unlock()
	return s.setupBootstrapValue
}

func TestAnnounceSetupBootstrapOnlyForReachableOpenSetup(t *testing.T) {
	remote := newBootstrapTestServer(t, "0.0.0.0")
	remote.announceSetupBootstrap()
	if activeSetupBootstrapToken(remote) == "" {
		t.Fatal("remote unconfigured instance started without a bootstrap token in the log")
	}

	local := newBootstrapTestServer(t, "127.0.0.1")
	local.announceSetupBootstrap()
	if activeSetupBootstrapToken(local) != "" {
		t.Fatal("local-only instance logged a bootstrap token nobody needs")
	}

	locked := newVaultLockedTestServer(t, "0.0.0.0")
	locked.announceSetupBootstrap()
	if activeSetupBootstrapToken(locked) != "" {
		t.Fatal("instance with an undecryptable vault offered a bootstrap token")
	}
}

func TestSetupTestConnectionRequiresBootstrapToken(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	addSetupCSRFTokenForTest(s, "csrf-test")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/setup/test", strings.NewReader(`{"provider_type":"openrouter","base_url":"https://openrouter.ai/api/v1","api_key":"sk-test","model":"test-model"}`))
	req.RemoteAddr = bootstrapTestRemotePeer
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf-test")
	rec := httptest.NewRecorder()
	handleSetupTestConnection(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_bootstrap_token_required" {
		t.Fatalf("status = %d body=%s, want 403 setup_bootstrap_token_required", rec.Code, rec.Body.String())
	}
}

func TestSetupLocalLLMProbeRequiresBootstrapToken(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	s.LocalLLM = &localllm.Manager{}
	addSetupCSRFTokenForTest(s, "csrf-probe")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/setup/local-llm/probe", strings.NewReader(`{"backend":"cpu","model_family":"qwen"}`))
	req.RemoteAddr = bootstrapTestRemotePeer
	req.Header.Set("X-CSRF-Token", "csrf-probe")
	rec := httptest.NewRecorder()
	handleSetupLocalLLMProbe(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_bootstrap_token_required" {
		t.Fatalf("status = %d body=%s, want 403 setup_bootstrap_token_required", rec.Code, rec.Body.String())
	}
}

func postFirstPassword(s *Server, remoteAddr, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"new_password":"claimed-password"}`))
	req.RemoteAddr = remoteAddr
	req.Host = "aurago.example"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://aurago.example")
	if token != "" {
		req.Header.Set(setupBootstrapHeader, token)
	}
	rec := httptest.NewRecorder()
	handleAuthSetPassword(s).ServeHTTP(rec, req)
	return rec
}

func TestAuthSetPasswordLockdownRequiresBootstrapToken(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	rec := postFirstPassword(s, bootstrapTestRemotePeer, "")
	if rec.Code != http.StatusForbidden || responseCode(t, rec) != "setup_bootstrap_token_required" {
		t.Fatalf("lockdown password without token: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := s.Vault.ReadSecret("auth_password_hash"); err == nil {
		t.Fatal("rejected first password request stored a password hash")
	}

	rec = postFirstPassword(s, bootstrapTestRemotePeer, "bootstrap-secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("lockdown password with token: status = %d body=%s", rec.Code, rec.Body.String())
	}
	if s.validSetupBootstrapToken("bootstrap-secret") {
		t.Fatal("bootstrap token stayed valid after the first password was set")
	}
}

func TestAuthSetPasswordWithAuthDisabledKeepsSameOriginFirstSetup(t *testing.T) {
	s := newBootstrapTestServer(t, "0.0.0.0")
	s.Cfg.Auth.Enabled = false

	rec := postFirstPassword(s, bootstrapTestRemotePeer, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("auth-disabled first password: status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// newVaultLockedTestServer reproduces a configured instance restarted with a
// wrong AURAGO_MASTER_KEY: the vault exists but cannot be decrypted.
func newVaultLockedTestServer(t *testing.T, host string) *Server {
	t.Helper()
	s := newBootstrapTestServer(t, host)
	vaultPath := filepath.Join(filepath.Dir(s.Cfg.ConfigPath), "vault.bin")
	if err := s.Vault.WriteSecret("auth_password_hash", "$2a$12$existing-owner-hash"); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
	wrongKey, err := security.NewVault(strings.Repeat("d", 64), vaultPath)
	if err != nil {
		t.Fatalf("open vault with wrong key: %v", err)
	}
	s.Vault = wrongKey
	return s
}

func TestSetupStaysClosedWhenVaultCannotBeDecrypted(t *testing.T) {
	s := newVaultLockedTestServer(t, "0.0.0.0")
	setSetupBootstrapTokenForTest(s, "bootstrap-secret")

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	req.RemoteAddr = bootstrapTestRemotePeer
	rec := httptest.NewRecorder()
	handleSetupStatus(s).ServeHTTP(rec, req)
	var status map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &status)
	if status["vault_locked"] != true || status["csrf_token"] != nil {
		t.Fatalf("status with undecryptable vault = %v, want vault_locked and no csrf_token", status)
	}

	addSetupCSRFTokenForTest(s, "csrf-v")
	rec = postSetup(s, bootstrapTestRemotePeer, `{"auth":{"enabled":false}}`, map[string]string{
		"X-CSRF-Token":       "csrf-v",
		setupBootstrapHeader: "bootstrap-secret",
	})
	if rec.Code != http.StatusServiceUnavailable || responseCode(t, rec) != "setup_vault_locked" {
		t.Fatalf("setup save with undecryptable vault: status = %d body=%s, want 503 setup_vault_locked", rec.Code, rec.Body.String())
	}
	if !s.Cfg.Auth.Enabled {
		t.Fatal("setup save disabled auth on an instance with an undecryptable vault")
	}

	rec = postFirstPassword(s, bootstrapTestRemotePeer, "bootstrap-secret")
	if rec.Code != http.StatusServiceUnavailable || responseCode(t, rec) != "setup_vault_locked" {
		t.Fatalf("first password with undecryptable vault: status = %d body=%s, want 503 setup_vault_locked", rec.Code, rec.Body.String())
	}
}

func TestSetupVaultLockAlsoAppliesToLocalLoopbackRequests(t *testing.T) {
	s := newVaultLockedTestServer(t, "127.0.0.1")
	addSetupCSRFTokenForTest(s, "csrf-local-v")

	rec := postSetup(s, "127.0.0.1:50003", `{"auth":{"enabled":false}}`, map[string]string{"X-CSRF-Token": "csrf-local-v"})
	if rec.Code != http.StatusServiceUnavailable || responseCode(t, rec) != "setup_vault_locked" {
		t.Fatalf("local setup save with undecryptable vault: status = %d body=%s, want 503 setup_vault_locked", rec.Code, rec.Body.String())
	}
}
