package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
)

func newOAuthHandlerTestServer(t *testing.T, tokenURL string) (*Server, *security.Vault) {
	t.Helper()
	vault := newOAuthSessionTestVault(t)
	cfg := &config.Config{}
	cfg.Server.Port = 8088
	cfg.Providers = []config.ProviderEntry{{
		ID:            "main",
		Name:          "Main OAuth",
		Type:          "openai",
		BaseURL:       "https://api.example/v1",
		Model:         "model",
		AuthType:      "oauth2",
		OAuthAuthURL:  "https://accounts.example/authorize",
		OAuthTokenURL: tokenURL,
		OAuthClientID: "client-id",
		OAuthScopes:   "openid profile",
	}}
	return &Server{
		Cfg:    cfg,
		Vault:  vault,
		Logger: slog.Default(),
	}, vault
}

func TestOAuthStartReturnsAutomatedSessionMetadataAndPKCE(t *testing.T) {
	t.Parallel()

	server, vault := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/start?provider=main", nil)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthStart(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		AuthURL       string   `json:"auth_url"`
		Mode          string   `json:"mode"`
		SessionID     string   `json:"session_id"`
		ExpiresAt     string   `json:"expires_at"`
		FallbackModes []string `json:"fallback_modes"`
		RedirectURI   string   `json:"redirect_uri"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.AuthURL == "" || body.SessionID == "" || body.ExpiresAt == "" {
		t.Fatalf("start response missing rich metadata: %+v", body)
	}
	if body.Mode != oauthFlowModeBrowserCallback {
		t.Fatalf("mode = %q, want %q", body.Mode, oauthFlowModeBrowserCallback)
	}
	if len(body.FallbackModes) != 1 || body.FallbackModes[0] != oauthFlowModeManualPaste {
		t.Fatalf("fallback_modes = %#v, want manual paste fallback", body.FallbackModes)
	}
	if body.RedirectURI != "http://aurago.example/api/oauth/callback" {
		t.Fatalf("redirect_uri = %q", body.RedirectURI)
	}
	authURL, err := url.Parse(body.AuthURL)
	if err != nil {
		t.Fatalf("parse auth_url: %v", err)
	}
	q := authURL.Query()
	if q.Get("state") != body.SessionID {
		t.Fatalf("auth_url state = %q, want session_id %q", q.Get("state"), body.SessionID)
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("auth_url missing PKCE: %s", body.AuthURL)
	}
	if q.Get("redirect_uri") != "http://aurago.example/api/oauth/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	raw, err := vault.ReadSecret("oauth_state_" + body.SessionID)
	if err != nil {
		t.Fatalf("oauth session not stored in vault: %v", err)
	}
	if !strings.Contains(raw, `"code_verifier"`) {
		t.Fatalf("stored session does not contain server-side code_verifier: %s", raw)
	}
}

func TestOAuthStartLaunchRedirectsToAuthorizationURL(t *testing.T) {
	t.Parallel()

	server, vault := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/start?provider=main&launch=1", nil)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthStart(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusFound, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("launch response missing Location header")
	}
	authURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	q := authURL.Query()
	state := q.Get("state")
	if state == "" {
		t.Fatalf("launch Location missing state: %s", location)
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("launch Location missing PKCE: %s", location)
	}
	if raw, err := vault.ReadSecret("oauth_state_" + state); err != nil || !strings.Contains(raw, `"code_verifier"`) {
		t.Fatalf("launch session not stored with verifier: raw=%q err=%v", raw, err)
	}
}

func TestOAuthStatusReturnsConfigurationMetadata(t *testing.T) {
	t.Parallel()

	server, _ := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	server.Cfg.Providers = append(server.Cfg.Providers, config.ProviderEntry{
		ID:            "incomplete",
		Name:          "Incomplete OAuth",
		Type:          "openai",
		BaseURL:       "https://api.example/v1",
		Model:         "model",
		AuthType:      "oauth2",
		OAuthAuthURL:  "https://accounts.example/authorize",
		OAuthTokenURL: "",
		OAuthClientID: "",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/status?provider=main", nil)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()
	handleOAuthStatus(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var configured struct {
		Provider      string   `json:"provider"`
		Authorized    bool     `json:"authorized"`
		Configured    bool     `json:"configured"`
		MissingFields []string `json:"missing_fields"`
		RedirectURI   string   `json:"redirect_uri"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &configured); err != nil {
		t.Fatalf("json.Unmarshal(configured) error = %v", err)
	}
	if configured.Provider != "main" || configured.Authorized {
		t.Fatalf("unexpected status identity: %+v", configured)
	}
	if !configured.Configured {
		t.Fatalf("configured = false, want true; body=%s", rec.Body.String())
	}
	if len(configured.MissingFields) != 0 {
		t.Fatalf("missing_fields = %#v, want empty", configured.MissingFields)
	}
	if configured.RedirectURI != "http://aurago.example/api/oauth/callback" {
		t.Fatalf("redirect_uri = %q", configured.RedirectURI)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/oauth/status?provider=incomplete", nil)
	req.Host = "aurago.example"
	rec = httptest.NewRecorder()
	handleOAuthStatus(server).ServeHTTP(rec, req)

	var incomplete struct {
		Configured    bool     `json:"configured"`
		MissingFields []string `json:"missing_fields"`
		RedirectURI   string   `json:"redirect_uri"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &incomplete); err != nil {
		t.Fatalf("json.Unmarshal(incomplete) error = %v", err)
	}
	if incomplete.Configured {
		t.Fatalf("configured = true for incomplete provider; body=%s", rec.Body.String())
	}
	wantMissing := map[string]bool{"oauth_token_url": true, "oauth_client_id": true}
	for _, field := range incomplete.MissingFields {
		delete(wantMissing, field)
	}
	if len(wantMissing) != 0 {
		t.Fatalf("missing_fields = %#v, still missing expected %#v", incomplete.MissingFields, wantMissing)
	}
	if incomplete.RedirectURI != "http://aurago.example/api/oauth/callback" {
		t.Fatalf("redirect_uri for incomplete provider = %q", incomplete.RedirectURI)
	}
}

func TestExchangeCodeForTokenSendsPKCEVerifier(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var form url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		form = r.Form
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access",
			"refresh_token": "refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(tokenServer.Close)

	prov := config.ProviderEntry{
		OAuthTokenURL: tokenServer.URL,
		OAuthClientID: "client-id",
	}
	if _, err := exchangeCodeForToken(prov, "auth-code", "http://aurago.example/api/oauth/callback", "pkce-verifier"); err != nil {
		t.Fatalf("exchangeCodeForToken() error = %v", err)
	}
	if got := form.Get("code_verifier"); got != "pkce-verifier" {
		t.Fatalf("code_verifier = %q, want pkce-verifier; form=%v", got, form)
	}
}

func TestOAuthManualCompletesStoredPKCESession(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var form url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		form = r.Form
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "manual-access",
			"refresh_token": "manual-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(tokenServer.Close)

	server, vault := newOAuthHandlerTestServer(t, tokenServer.URL)
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "http://aurago.example/api/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	body := strings.NewReader(`{"url":"http://localhost:8088/api/oauth/callback?code=manual-code&state=` + session.State + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/manual", body)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthManual(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := form.Get("code_verifier"); got != session.CodeVerifier {
		t.Fatalf("code_verifier = %q, want session verifier", got)
	}
	raw, err := vault.ReadSecret("oauth_main")
	if err != nil {
		t.Fatalf("oauth_main token was not stored: %v", err)
	}
	if !strings.Contains(raw, "manual-access") || !strings.Contains(raw, "manual-refresh") {
		t.Fatalf("stored token = %s", raw)
	}
}

func TestOAuthCallbackResultBroadcastsProviderID(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "callback-access",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	t.Cleanup(tokenServer.Close)

	server, vault := newOAuthHandlerTestServer(t, tokenServer.URL)
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "http://aurago.example/api/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=callback-code&state="+session.State, nil)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthCallback(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `new BroadcastChannel("aurago-oauth")`) {
		t.Fatalf("callback result did not initialize OAuth BroadcastChannel: %s", body)
	}
	if !strings.Contains(body, `"provider_id":"main"`) {
		t.Fatalf("callback result did not include provider_id in broadcast payload: %s", body)
	}
}

func TestOAuthCallbackErrorBroadcastsProviderID(t *testing.T) {
	server, vault := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "http://aurago.example/api/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?error=access_denied&error_description=user+cancelled&state="+session.State, nil)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthCallback(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Authorization denied") {
		t.Fatalf("callback error page should mention denied authorization: %s", body)
	}
	if !strings.Contains(body, `"provider_id":"main"`) {
		t.Fatalf("callback error result did not include provider_id in broadcast payload: %s", body)
	}
}

func TestOAuthCallbackShowsHelpfulTokenExchangeError(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "redirect_uri mismatch",
		})
	}))
	t.Cleanup(tokenServer.Close)

	server, vault := newOAuthHandlerTestServer(t, tokenServer.URL)
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "http://aurago.example/api/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/callback?code=bad-code&state="+session.State, nil)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthCallback(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Redirect URI") {
		t.Fatalf("callback error should mention Redirect URI; body=%s", body)
	}
	if strings.Contains(body, "Token exchange failed</p>") {
		t.Fatalf("callback error should not show only a generic token exchange failure: %s", body)
	}
}

func TestOAuthManualShowsHelpfulTokenExchangeError(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_client",
			"error_description": "client secret is invalid",
		})
	}))
	t.Cleanup(tokenServer.Close)

	server, vault := newOAuthHandlerTestServer(t, tokenServer.URL)
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "http://aurago.example/api/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	body := strings.NewReader(`{"url":"http://localhost:8088/api/oauth/callback?code=bad-code&state=` + session.State + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/manual", body)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthManual(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if result.Success {
		t.Fatalf("success = true, want false; body=%s", rec.Body.String())
	}
	if !strings.Contains(result.Message, "Client ID or client secret") {
		t.Fatalf("manual error should mention client credentials; message=%q", result.Message)
	}
	if result.Message == "Token exchange failed" {
		t.Fatalf("manual error should not be generic")
	}
}

func TestOAuthManualAppliesTokenToRuntimeClients(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "runtime-access",
			"refresh_token": "runtime-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(tokenServer.Close)

	server, vault := newOAuthHandlerTestServer(t, tokenServer.URL)
	server.Cfg.LLM.Provider = "main"
	server.Cfg.LLMGuardian.Enabled = true
	server.Cfg.LLMGuardian.Provider = "main"
	server.Cfg.ResolveProviders()
	server.LLMClient = llm.NewFailoverManager(server.Cfg, server.Logger)
	if fm, ok := server.LLMClient.(*llm.FailoverManager); ok {
		t.Cleanup(fm.Stop)
	}
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "http://aurago.example/api/oauth/callback", time.Now().UTC())
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	body := strings.NewReader(`{"url":"http://localhost:8088/api/oauth/callback?code=runtime-code&state=` + session.State + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/manual", body)
	req.Host = "aurago.example"
	rec := httptest.NewRecorder()

	handleOAuthManual(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if server.Cfg.LLM.APIKey != "runtime-access" {
		t.Fatalf("runtime cfg LLM APIKey = %q, want OAuth access token", server.Cfg.LLM.APIKey)
	}
	if got := failoverPrimaryAPIKey(t, server.LLMClient); got != "runtime-access" {
		t.Fatalf("failover primary API key = %q, want OAuth access token", got)
	}
	if server.LLMGuardian == nil {
		t.Fatal("LLMGuardian was not recreated after OAuth token application")
	}
}

func failoverPrimaryAPIKey(t *testing.T, client llm.ChatClient) string {
	t.Helper()
	v := reflect.ValueOf(client)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		t.Fatalf("client is %T, want *llm.FailoverManager", client)
	}
	elem := v.Elem()
	field := elem.FieldByName("primaryAPIKey")
	if !field.IsValid() {
		t.Fatalf("client %T has no primaryAPIKey field", client)
	}
	return field.String()
}
