package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestHandleGetProvidersMasksSecretsAndIncludesReferences(t *testing.T) {
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
image_generation:
  provider: main
`)
	if err := vault.WriteSecret("provider_main_api_key", "static-secret"); err != nil {
		t.Fatalf("WriteSecret(api key) error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var providers []struct {
		providerJSON
		References []struct {
			Path string `json:"path"`
			Role string `json:"role"`
		} `json:"references"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("provider count = %d, want 1", len(providers))
	}
	if providers[0].APIKey != maskedKey {
		t.Fatalf("api_key = %q, want masked", providers[0].APIKey)
	}
	if !providerJSONReferenceContains(providers[0].References, "llm.provider") ||
		!providerJSONReferenceContains(providers[0].References, "image_generation.provider") {
		t.Fatalf("references = %+v, want llm and image generation references", providers[0].References)
	}
}

func TestHandleProvidersRoundTripsCapabilities(t *testing.T) {
	server, _ := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o
llm:
  provider: main
`)

	body := `[{
		"id":"main",
		"name":"Main",
		"type":"openai",
		"base_url":"https://api.openai.com/v1",
		"model":"gpt-4o",
		"auth_type":"api_key",
		"capabilities":{
			"auto":false,
			"tool_calling":true,
			"structured_outputs":true,
			"multimodal":true,
			"detected_model":"gpt-4o",
			"source":"manual"
		}
	}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handleProviders(server).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	rec = httptest.NewRecorder()
	handleProviders(server).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var providers []providerJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &providers); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("provider count = %d, want 1", len(providers))
	}
	if providers[0].Capabilities == nil {
		t.Fatal("expected capabilities in provider response")
	}
	if providers[0].Capabilities.Auto {
		t.Fatal("expected manual capabilities to round-trip")
	}
	if !providers[0].Capabilities.ToolCalling || !providers[0].Capabilities.StructuredOutputs || !providers[0].Capabilities.Multimodal {
		t.Fatalf("capabilities did not round-trip: %+v", *providers[0].Capabilities)
	}
	if !providers[0].EffectiveCapabilities.ToolCalling || !providers[0].EffectiveCapabilities.StructuredOutputs || !providers[0].EffectiveCapabilities.Multimodal {
		t.Fatalf("effective capabilities not returned: %+v", providers[0].EffectiveCapabilities)
	}
}

func TestHandlePutProvidersRejectsInvalidProviderIDs(t *testing.T) {
	t.Parallel()

	server, _ := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: main
`)
	// Spaces and uppercase remain invalid; dots in IDs are allowed (e.g. llama-3.1-8b).
	body := `[{"id":"main","name":"Main","type":"openai","base_url":"https://api.openai.com/v1","model":"gpt-4o-mini","auth_type":"api_key"},{"id":"Bad ID","name":"Bad","type":"openai","base_url":"https://api.openai.com/v1","model":"gpt-4o-mini","auth_type":"api_key"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if got := server.Cfg.FindProvider("main"); got == nil {
		t.Fatal("existing provider disappeared after invalid request")
	}
}

func TestHandlePutProvidersAllowsDotsInNewProviderIDs(t *testing.T) {
	t.Parallel()

	server, _ := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: main
`)
	body := `[{"id":"main","name":"Main","type":"openai","base_url":"https://api.openai.com/v1","model":"gpt-4o-mini","auth_type":"api_key"},{"id":"llama-3.1-8b","name":"Llama 3.1","type":"openrouter","base_url":"https://openrouter.ai/api/v1","model":"meta-llama/llama-3.1-8b-instruct","auth_type":"api_key"},{"id":"crof.ai","name":"CROF","type":"openai","base_url":"https://crof.ai/v2","model":"deepseek-v4-pro","auth_type":"api_key"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := server.Cfg.FindProvider("llama-3.1-8b"); got == nil {
		t.Fatal("provider id with dots not saved")
	}
	if got := server.Cfg.FindProvider("crof.ai"); got == nil {
		t.Fatal("provider id with domain-style dots not saved")
	}
}

func TestHandlePutProvidersAllowsLegacyInvalidIDsWhenReSaving(t *testing.T) {
	t.Parallel()

	server, _ := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
  - id: llama-3.1-8b
    name: Llama 3.1
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    model: meta-llama/llama-3.1-8b-instruct
llm:
  provider: main
`)
	// Re-save the legacy invalid ID and add a valid new provider — must succeed.
	body := `[{"id":"main","name":"Main","type":"openai","base_url":"https://api.openai.com/v1","model":"gpt-4o-mini","auth_type":"api_key"},{"id":"llama-3.1-8b","name":"Llama 3.1","type":"openrouter","base_url":"https://openrouter.ai/api/v1","model":"meta-llama/llama-3.1-8b-instruct","auth_type":"api_key"},{"id":"agnesai","name":"agnes","type":"openai","base_url":"https://apihub.agnes-ai.com/v1","model":"agnes-2.0-flash","auth_type":"api_key"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := server.Cfg.FindProvider("llama-3.1-8b"); got == nil {
		t.Fatal("legacy provider missing after re-save")
	}
	if got := server.Cfg.FindProvider("agnesai"); got == nil {
		t.Fatal("new provider not saved")
	}
}

func TestHandlePutProvidersCopyFromUsesVaultAndConfigSecrets(t *testing.T) {
	t.Parallel()

	t.Run("vault secret", func(t *testing.T) {
		server, vault := newProviderTestServer(t, `
providers:
  - id: source
    name: Source
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: source
`)
		if err := vault.WriteSecret("provider_source_api_key", "vault-secret"); err != nil {
			t.Fatalf("WriteSecret() error = %v", err)
		}
		server.Cfg.ApplyVaultSecrets(vault)
		server.Cfg.ResolveProviders()

		body := `[{"id":"source","name":"Source","type":"openai","base_url":"https://api.openai.com/v1","api_key":"••••••••","model":"gpt-4o-mini","auth_type":"api_key"},{"id":"target","name":"Target","type":"openai","base_url":"https://api.openai.com/v1","api_key":"__copy_from__source","model":"gpt-4o-mini","auth_type":"api_key"}]`
		req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		handleProviders(server).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		got, err := vault.ReadSecret("provider_target_api_key")
		if err != nil || got != "vault-secret" {
			t.Fatalf("copied vault secret = %q err=%v, want vault-secret", got, err)
		}
	})

	t.Run("loaded config secret", func(t *testing.T) {
		server, vault := newProviderTestServer(t, `
providers:
  - id: source
    name: Source
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: source
`)
		server.Cfg.Providers[0].APIKey = "config-secret"
		body := `[{"id":"source","name":"Source","type":"openai","base_url":"https://api.openai.com/v1","api_key":"••••••••","model":"gpt-4o-mini","auth_type":"api_key"},{"id":"target","name":"Target","type":"openai","base_url":"https://api.openai.com/v1","api_key":"__copy_from__source","model":"gpt-4o-mini","auth_type":"api_key"}]`
		req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		handleProviders(server).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		got, err := vault.ReadSecret("provider_target_api_key")
		if err != nil || got != "config-secret" {
			t.Fatalf("copied config secret = %q err=%v, want config-secret", got, err)
		}
	})
}

func TestHandlePutProvidersCopyFromMissingSourceReturnsBadRequest(t *testing.T) {
	t.Parallel()

	server, vault := newProviderTestServer(t, `
providers:
  - id: source
    name: Source
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: source
`)
	body := `[{"id":"source","name":"Source","type":"openai","base_url":"https://api.openai.com/v1","api_key":"","model":"gpt-4o-mini","auth_type":"api_key"},{"id":"target","name":"Target","type":"openai","base_url":"https://api.openai.com/v1","api_key":"__copy_from__source","model":"gpt-4o-mini","auth_type":"api_key"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if _, err := vault.ReadSecret("provider_target_api_key"); err == nil {
		t.Fatal("target secret was written despite copy-from failure")
	}
	if provider := server.Cfg.FindProvider("target"); provider != nil {
		t.Fatalf("target provider was saved despite copy-from failure: %+v", provider)
	}
}

func TestHandlePutProvidersPreservesTopLevelYAMLCommentsAndOrder(t *testing.T) {
	t.Parallel()

	server, _ := newProviderTestServer(t, `# top comment
server:
  # host comment
  host: 127.0.0.1
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
# llm comment
llm:
  provider: main
`)
	body := `[{"id":"main","name":"Renamed","type":"openai","base_url":"https://api.openai.com/v1","api_key":"","model":"gpt-4o-mini","auth_type":"api_key"}]`
	req := httptest.NewRequest(http.MethodPut, "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handleProviders(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	data, err := os.ReadFile(server.Cfg.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	out := string(data)
	for _, want := range []string{"# top comment", "# host comment", "# llm comment", "name: Renamed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("saved config missing %q:\n%s", want, out)
		}
	}
	if !(strings.Index(out, "server:") < strings.Index(out, "providers:") &&
		strings.Index(out, "providers:") < strings.Index(out, "llm:")) {
		t.Fatalf("top-level key order changed unexpectedly:\n%s", out)
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

func providerJSONReferenceContains(refs []struct {
	Path string `json:"path"`
	Role string `json:"role"`
}, path string) bool {
	for _, ref := range refs {
		if ref.Path == path {
			return true
		}
	}
	return false
}
