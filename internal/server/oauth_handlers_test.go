package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
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
