package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/llm/catalog"
)

func TestAgodeskProviderCapabilitiesRequireWebConfigVaultAndFilterReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	vault := newOAuthSessionTestVault(t)
	server := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}

	all := agodeskServerCapabilities(server)
	for _, want := range []string{
		agodesk.CapabilityConfigProvidersRead,
		agodesk.CapabilityConfigProvidersWrite,
		agodesk.CapabilityConfigProvidersOAuth,
	} {
		if !agodeskTestContainsString(all, want) {
			t.Fatalf("agodeskServerCapabilities missing %s: %v", want, all)
		}
	}

	readOnly := agodeskServerCapabilitiesForDevice(server, true)
	if !agodeskTestContainsString(readOnly, agodesk.CapabilityConfigProvidersRead) {
		t.Fatalf("read-only capabilities missing provider read: %v", readOnly)
	}
	for _, denied := range []string{agodesk.CapabilityConfigProvidersWrite, agodesk.CapabilityConfigProvidersOAuth} {
		if agodeskTestContainsString(readOnly, denied) {
			t.Fatalf("read-only capabilities advertised %s: %v", denied, readOnly)
		}
	}

	cfg.WebConfig.Enabled = false
	disabled := agodeskServerCapabilities(server)
	if agodeskTestContainsString(disabled, agodesk.CapabilityConfigProvidersRead) {
		t.Fatalf("provider capabilities advertised while web_config disabled: %v", disabled)
	}
}

func TestAgodeskProviderPayloadDoesNotExposeSecretsAndIncludesReferences(t *testing.T) {
	server, vault := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
  - id: oauth
    name: OAuth Provider
    type: google
    base_url: https://generativelanguage.googleapis.com/v1beta/openai
    model: gemini-2.5-flash
    auth_type: oauth2
    oauth_auth_url: https://accounts.example/authorize
    oauth_token_url: https://accounts.example/token
    oauth_client_id: client-id
llm:
  provider: main
  helper_provider: oauth
image_generation:
  provider: main
`)
	if err := vault.WriteSecret("provider_main_api_key", "static-api-secret"); err != nil {
		t.Fatalf("WriteSecret(api key) error = %v", err)
	}
	if err := vault.WriteSecret("provider_oauth_oauth_client_secret", "oauth-client-secret"); err != nil {
		t.Fatalf("WriteSecret(client secret) error = %v", err)
	}
	if err := vault.WriteSecret("oauth_oauth", `{"access_token":"oauth-access-token","refresh_token":"oauth-refresh-token","expiry":"2099-01-02T03:04:05Z"}`); err != nil {
		t.Fatalf("WriteSecret(oauth token) error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()
	server.Cfg.ApplyOAuthTokens(vault)

	payload := agodeskProvidersPayload(server, "agodesk:dev-1")
	if len(payload.Providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(payload.Providers))
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, forbidden := range []string{"static-api-secret", "oauth-client-secret", "oauth-access-token", "oauth-refresh-token"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("provider payload leaked %q: %s", forbidden, raw)
		}
	}

	main := payload.Providers[0]
	if main.ID != "main" || !main.Secrets.APIKey.Present {
		t.Fatalf("main provider secret state = %+v", main)
	}
	if !agodeskTestProviderReferenceContains(main.References, "llm.provider") || !agodeskTestProviderReferenceContains(main.References, "image_generation.provider") {
		t.Fatalf("main references = %+v", main.References)
	}
	oauth := payload.Providers[1]
	if !oauth.Secrets.OAuthClientSecret.Present || !oauth.OAuth.Authorized || !oauth.OAuth.HasRefreshToken {
		t.Fatalf("oauth safe state = %+v", oauth)
	}
}

func TestAgodeskProviderUpsertHonorsSecretOperations(t *testing.T) {
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
	if err := vault.WriteSecret("provider_main_api_key", "old-secret"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()

	_, err := upsertAgodeskProvider(server, agodesk.ConfigProviderUpsertPayload{
		SessionID: "agodesk:dev-1",
		Mode:      "update",
		Provider: agodesk.ConfigProviderEntryPayload{
			ID:       "main",
			Name:     "Main",
			Type:     "openai",
			BaseURL:  "https://api.openai.com/v1",
			Model:    "gpt-4o-mini",
			AuthType: "api_key",
		},
		Secrets: agodesk.ConfigProviderSecretOpsPayload{
			APIKey: agodesk.SecretOperationPayload{Op: "keep"},
		},
	})
	if err != nil {
		t.Fatalf("upsert keep: %v", err)
	}
	if got, err := vault.ReadSecret("provider_main_api_key"); err != nil || got != "old-secret" {
		t.Fatalf("kept secret = %q err=%v, want old-secret", got, err)
	}

	_, err = upsertAgodeskProvider(server, agodesk.ConfigProviderUpsertPayload{
		SessionID: "agodesk:dev-1",
		Mode:      "update",
		Provider: agodesk.ConfigProviderEntryPayload{
			ID:       "main",
			Name:     "Main",
			Type:     "openai",
			BaseURL:  "https://api.openai.com/v1",
			Model:    "gpt-4o-mini",
			AuthType: "api_key",
		},
		Secrets: agodesk.ConfigProviderSecretOpsPayload{
			APIKey: agodesk.SecretOperationPayload{Op: "set", Value: "new-secret"},
		},
	})
	if err != nil {
		t.Fatalf("upsert set: %v", err)
	}
	if got, err := vault.ReadSecret("provider_main_api_key"); err != nil || got != "new-secret" {
		t.Fatalf("set secret = %q err=%v, want new-secret", got, err)
	}

	_, err = upsertAgodeskProvider(server, agodesk.ConfigProviderUpsertPayload{
		SessionID: "agodesk:dev-1",
		Mode:      "update",
		Provider: agodesk.ConfigProviderEntryPayload{
			ID:       "main",
			Name:     "Main",
			Type:     "openai",
			BaseURL:  "https://api.openai.com/v1",
			Model:    "gpt-4o-mini",
			AuthType: "api_key",
		},
		Secrets: agodesk.ConfigProviderSecretOpsPayload{
			APIKey: agodesk.SecretOperationPayload{Op: "clear"},
		},
	})
	if err != nil {
		t.Fatalf("upsert clear: %v", err)
	}
	if got, err := vault.ReadSecret("provider_main_api_key"); err == nil {
		t.Fatalf("cleared secret still readable: %q", got)
	}
}

func TestAgodeskProviderCatalogDetailIncludesGoogleOAuthSetup(t *testing.T) {
	server := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	server.Cfg.ModelCatalog.Enabled = true
	payload, err := agodeskProviderCatalogPayload(server, "agodesk:dev-1", agodesk.ConfigProviderCatalogDetailPayload{
		SessionID:  "agodesk:dev-1",
		ProviderID: "google",
	})
	if err != nil {
		t.Fatalf("agodeskProviderCatalogPayload() error = %v", err)
	}
	if len(payload.Providers) != 1 || payload.Providers[0].ID != "google" {
		t.Fatalf("catalog providers = %+v", payload.Providers)
	}
	if payload.Providers[0].OAuthSetup == nil {
		t.Fatalf("google catalog detail missing oauth_setup: %+v", payload.Providers[0])
	}
	if payload.Providers[0].OAuthSetup.AuthURL == "" || payload.Providers[0].OAuthSetup.TokenURL == "" {
		t.Fatalf("google oauth setup missing endpoints: %+v", payload.Providers[0].OAuthSetup)
	}
	if payload.Metadata.PackageName == "" {
		t.Fatalf("catalog metadata not included: %+v", payload.Metadata)
	}
}

func TestAgodeskProviderOAuthDesktopStartAndComplete(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var form url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		form = r.Form
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "desktop-access",
			"refresh_token": "desktop-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(tokenServer.Close)

	server, vault := newOAuthHandlerTestServer(t, tokenServer.URL)
	started, err := startAgodeskProviderOAuth(server, agodesk.ConfigProviderOAuthStartPayload{
		SessionID:   "agodesk:dev-1",
		ProviderID:  "main",
		RedirectURI: "http://127.0.0.1:49152/oauth/callback",
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("startAgodeskProviderOAuth() error = %v", err)
	}
	if started.Mode != oauthFlowModeAgodeskLoopback || started.OAuthState == "" || started.AuthURL == "" {
		t.Fatalf("started payload = %+v", started)
	}
	authURL, err := url.Parse(started.AuthURL)
	if err != nil {
		t.Fatalf("parse auth_url: %v", err)
	}
	if authURL.Query().Get("redirect_uri") != started.RedirectURI {
		t.Fatalf("auth_url redirect_uri = %q, want %q", authURL.Query().Get("redirect_uri"), started.RedirectURI)
	}

	status, err := completeAgodeskProviderOAuth(server, agodesk.ConfigProviderOAuthCompletePayload{
		SessionID:   "agodesk:dev-1",
		ProviderID:  "main",
		RedirectURI: started.RedirectURI,
		Code:        "desktop-code",
		State:       started.OAuthState,
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("completeAgodeskProviderOAuth() error = %v", err)
	}
	if !status.Authorized || status.ProviderID != "main" {
		t.Fatalf("oauth status = %+v", status)
	}
	if form.Get("redirect_uri") != started.RedirectURI {
		t.Fatalf("token redirect_uri = %q, want %q", form.Get("redirect_uri"), started.RedirectURI)
	}
	if form.Get("code_verifier") == "" {
		t.Fatalf("token request missing code_verifier: %v", form)
	}
	raw, err := vault.ReadSecret("oauth_main")
	if err != nil {
		t.Fatalf("oauth_main token missing: %v", err)
	}
	if !strings.Contains(raw, "desktop-access") || !strings.Contains(raw, "desktop-refresh") {
		t.Fatalf("stored oauth token = %s", raw)
	}
}

func TestAgodeskProviderOAuthCompleteRejectsRedirectMismatch(t *testing.T) {
	server, vault := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	session, err := newOAuthSession("main", oauthFlowModeAgodeskLoopback, "http://127.0.0.1:49152/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}
	_, err = completeAgodeskProviderOAuth(server, agodesk.ConfigProviderOAuthCompletePayload{
		SessionID:   "agodesk:dev-1",
		ProviderID:  "main",
		RedirectURI: "http://127.0.0.1:49153/oauth/callback",
		Code:        "code",
		State:       session.State,
	}, time.Now().UTC())
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "redirect") {
		t.Fatalf("completeAgodeskProviderOAuth redirect mismatch error = %v", err)
	}
}

func TestAgodeskProviderOAuthStatusAndRevoke(t *testing.T) {
	server, vault := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	if err := vault.WriteSecret("oauth_main", `{"access_token":"status-access","refresh_token":"status-refresh","expiry":"2099-01-02T03:04:05Z"}`); err != nil {
		t.Fatalf("WriteSecret(oauth_main) error = %v", err)
	}
	status := agodeskProviderOAuthStatus(server, "main", "http://127.0.0.1:49152/oauth/callback")
	if !status.Authorized || status.Expired || !status.HasRefreshToken {
		t.Fatalf("oauth status before revoke = %+v", status)
	}
	if err := revokeAgodeskProviderOAuth(server, "main"); err != nil {
		t.Fatalf("revokeAgodeskProviderOAuth() error = %v", err)
	}
	status = agodeskProviderOAuthStatus(server, "main", "http://127.0.0.1:49152/oauth/callback")
	if status.Authorized || status.HasRefreshToken {
		t.Fatalf("oauth status after revoke = %+v", status)
	}
}

func TestAgodeskProviderDeleteCleansVaultAndReturnsList(t *testing.T) {
	server, vault := newProviderTestServer(t, `
providers:
  - id: main
    name: Main
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
  - id: spare
    name: Spare
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
llm:
  provider: main
`)
	if err := vault.WriteSecret("provider_spare_api_key", "spare-secret"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	if err := vault.WriteSecret("oauth_spare", `{"access_token":"token"}`); err != nil {
		t.Fatalf("WriteSecret(oauth_spare) error = %v", err)
	}

	payload, err := deleteAgodeskProvider(server, agodesk.ConfigProviderDeletePayload{
		SessionID:  "agodesk:dev-1",
		ProviderID: "spare",
	})
	if err != nil {
		t.Fatalf("deleteAgodeskProvider() error = %v", err)
	}
	if len(payload.Providers) != 1 || payload.Providers[0].ID != "main" {
		t.Fatalf("providers after delete = %+v", payload.Providers)
	}
	for _, key := range []string{"provider_spare_api_key", "provider_spare_oauth_client_secret", "oauth_spare"} {
		if value, err := vault.ReadSecret(key); err == nil {
			t.Fatalf("%s still in vault: %q", key, value)
		}
	}
}

func TestAgodeskProviderWSRejectsProviderListWithoutCapability(t *testing.T) {
	server := &Server{
		Cfg:    &config.Config{},
		Vault:  newOAuthSessionTestVault(t),
		Logger: slog.Default(),
	}
	conn, cleanup := dialAgodeskTestWebSocket(t, server, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	req, err := agodesk.NewEnvelope(agodesk.TypeConfigProvidersList, agodesk.ConfigProvidersListPayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope providers.list: %v", err)
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write providers.list: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatError {
		t.Fatalf("response type = %q, want chat.error", resp.Type)
	}
	var errPayload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &errPayload)
	if errPayload.Code != agodesk.ErrorUnsupportedCapability {
		t.Fatalf("error code = %q, want unsupported capability", errPayload.Code)
	}
}

func TestAgodeskProviderWSListInDevModeDoesNotLeakSecrets(t *testing.T) {
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
	server.Cfg.WebConfig.Enabled = true
	if err := vault.WriteSecret("provider_main_api_key", "ws-secret"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()

	conn, cleanup := dialAgodeskTestWebSocket(t, server, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	req, err := agodesk.NewEnvelope(agodesk.TypeConfigProvidersList, agodesk.ConfigProvidersListPayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope providers.list: %v", err)
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write providers.list: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeConfigProviders {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeConfigProviders)
	}
	var payload agodesk.ConfigProvidersPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	raw, _ := json.Marshal(payload)
	if strings.Contains(string(raw), "ws-secret") {
		t.Fatalf("ws providers leaked secret: %s", raw)
	}
	if len(payload.Providers) != 1 || !payload.Providers[0].Secrets.APIKey.Present {
		t.Fatalf("providers payload = %+v", payload)
	}
}

func agodeskTestProviderReferenceContains(refs []agodesk.ProviderReferencePayload, path string) bool {
	for _, ref := range refs {
		if ref.Path == path {
			return true
		}
	}
	return false
}

func TestAgodeskProviderCatalogPayloadMatchesModelCatalogAvailability(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{}
	cfg.ConfigPath = filepath.Join(tmpDir, "config.yaml")
	cfg.ModelCatalog.Enabled = true
	cfg.Providers = []config.ProviderEntry{{
		ID:     "main",
		Name:   "Main",
		Type:   "google",
		APIKey: "configured",
	}}
	server := &Server{Cfg: cfg, Logger: slog.Default()}
	payload, err := agodeskProviderCatalogPayload(server, "agodesk:dev-1", agodesk.ConfigProviderCatalogDetailPayload{
		SessionID:  "agodesk:dev-1",
		ProviderID: "google",
	})
	if err != nil {
		t.Fatalf("agodeskProviderCatalogPayload() error = %v", err)
	}
	snapshot, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load() error = %v", err)
	}
	provider, ok := snapshot.FindProvider("google")
	if !ok {
		t.Fatal("catalog missing google provider")
	}
	available, availability := catalogProviderAvailability(cfg, provider, disabledCatalogProviders(cfg))
	if payload.Providers[0].Available != available || payload.Providers[0].Availability != availability {
		t.Fatalf("agodesk availability = %v/%q, catalog = %v/%q", payload.Providers[0].Available, payload.Providers[0].Availability, available, availability)
	}
}
