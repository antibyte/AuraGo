package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func newProviderTestServer(t *testing.T, configYAML string) (*Server, *security.Vault) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := config.WriteFileAtomic(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("security.NewVault() error = %v", err)
	}

	cfg.ApplyVaultSecrets(vault)
	cfg.ResolveProviders()
	cfg.ApplyOAuthTokens(vault)

	return &Server{
		Cfg:    cfg,
		Vault:  vault,
		Logger: slog.Default(),
	}, vault
}

func TestHandleGetProvidersDoesNotExposeOAuthAccessTokenAsApiKey(t *testing.T) {
	t.Parallel()

	server, vault := newProviderTestServer(t, `
providers:
  - id: oauth
    name: OAuth Provider
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    auth_type: oauth2
    oauth_auth_url: https://accounts.example/authorize
    oauth_token_url: https://accounts.example/token
    oauth_client_id: client-id
llm:
  provider: oauth
`)
	if err := vault.WriteSecret("provider_oauth_oauth_client_secret", "client-secret"); err != nil {
		t.Fatalf("WriteSecret(client secret) error = %v", err)
	}
	if err := vault.WriteSecret("oauth_oauth", `{"access_token":"oauth-access-token"}`); err != nil {
		t.Fatalf("WriteSecret(oauth token) error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()
	server.Cfg.ApplyOAuthTokens(vault)

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var providers []providerJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("provider count = %d, want 1", len(providers))
	}
	if providers[0].APIKey != "" {
		t.Fatalf("api_key = %q, want empty for oauth2 providers", providers[0].APIKey)
	}
	if providers[0].OAuthClientSecret != maskedKey {
		t.Fatalf("oauth_client_secret = %q, want masked", providers[0].OAuthClientSecret)
	}
}

func TestHandlePutProvidersDeletesClearedStaticApiKey(t *testing.T) {
	t.Parallel()

	server, vault := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: main
`)
	if err := vault.WriteSecret("provider_main_api_key", "static-secret"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()

	body := `[{"id":"main","name":"Main","type":"openai","base_url":"https://api.openai.com/v1","api_key":"","model":"gpt-4o-mini","auth_type":"api_key"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if _, err := vault.ReadSecret("provider_main_api_key"); err == nil {
		t.Fatal("expected provider_main_api_key to be deleted from vault")
	}
}

func TestHandlePutProvidersDoesNotPersistOAuthAccessTokenAsStaticKey(t *testing.T) {
	t.Parallel()

	server, vault := newProviderTestServer(t, `
providers:
  - id: oauth
    name: OAuth Provider
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    auth_type: oauth2
    oauth_auth_url: https://accounts.example/authorize
    oauth_token_url: https://accounts.example/token
    oauth_client_id: client-id
llm:
  provider: oauth
`)
	if err := vault.WriteSecret("provider_oauth_oauth_client_secret", "client-secret"); err != nil {
		t.Fatalf("WriteSecret(client secret) error = %v", err)
	}
	if err := vault.WriteSecret("oauth_oauth", `{"access_token":"oauth-access-token"}`); err != nil {
		t.Fatalf("WriteSecret(oauth token) error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()
	server.Cfg.ApplyOAuthTokens(vault)

	body := `[{"id":"oauth","name":"OAuth Provider","type":"openai","base_url":"https://api.openai.com/v1","api_key":"","model":"gpt-4o-mini","auth_type":"oauth2","oauth_auth_url":"https://accounts.example/authorize","oauth_token_url":"https://accounts.example/token","oauth_client_id":"client-id","oauth_client_secret":"••••••••"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if _, err := vault.ReadSecret("provider_oauth_api_key"); err == nil {
		t.Fatal("expected no static api key to be persisted for oauth2 provider")
	}
	secret, err := vault.ReadSecret("provider_oauth_oauth_client_secret")
	if err != nil {
		t.Fatalf("ReadSecret(client secret) error = %v", err)
	}
	if secret != "client-secret" {
		t.Fatalf("oauth client secret = %q, want client-secret", secret)
	}
	token, err := vault.ReadSecret("oauth_oauth")
	if err != nil {
		t.Fatalf("ReadSecret(oauth token) error = %v", err)
	}
	if !strings.Contains(token, "oauth-access-token") {
		t.Fatalf("oauth token vault entry = %q, want access token to remain intact", token)
	}
}
