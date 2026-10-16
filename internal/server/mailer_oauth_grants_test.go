package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/mailer"
	"aurago/internal/security"
)

func stubMailerOAuthResponse(t *testing.T, endpoint string, response tokenExchangeResponse) {
	t.Helper()
	previous := newMailerOAuthHTTPClient
	t.Cleanup(func() { newMailerOAuthHTTPClient = previous })
	newMailerOAuthHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost || r.URL.String() != endpoint {
				t.Fatalf("unexpected OAuth request: %s %s", r.Method, r.URL)
			}
			if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code_verifier") == "" {
				t.Fatal("OAuth exchange lost its grant type or PKCE verifier")
			}
			body, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header), Request: r}, nil
		})}
	}
}

func TestMailerMicrosoftOAuthAcceptsResourceGrantsAndRequiresRefreshToken(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, scopes, refresh string
		wantError                       bool
	}{
		{"IMAP", "imap", "https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send", "fixture-refresh", false},
		{"POP3", "pop3", "https://outlook.office.com/POP.AccessAsUser.All https://outlook.office.com/SMTP.Send", "fixture-refresh", false},
		{"missing send", "imap", "https://outlook.office.com/IMAP.AccessAsUser.All", "fixture-refresh", true},
		{"wrong receive protocol", "pop3", "https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send", "fixture-refresh", true},
		{"missing refresh", "imap", "https://outlook.office.com/IMAP.AccessAsUser.All https://outlook.office.com/SMTP.Send", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailerOAuthTestServer(t)
			s.Cfg.Auth.Enabled = false
			s.Mailer = &mailer.Service{}
			account := config.EmailAccount{ID: "fixture-ms", ReceiveProtocol: tc.protocol, OAuthSource: "mailer", OAuthProvider: "microsoft", OAuthClientID: "fixture-client"}
			s.Cfg.EmailAccounts = []config.EmailAccount{account}
			if err := s.Vault.WriteSecret("email_fixture-ms_oauth_client_secret", "fixture-client-secret"); err != nil {
				t.Fatal(err)
			}
			key, previous := "email_fixture-ms_oauth_token", `{"pending_login":"fixture"}`
			if err := s.Vault.WriteSecret(key, previous); err != nil {
				t.Fatal(err)
			}
			stubMailerOAuthResponse(t, mailerMicrosoftTokenURL, tokenExchangeResponse{AccessToken: "fixture-access", RefreshToken: tc.refresh, TokenType: "Bearer", ExpiresIn: 3600, Scope: tc.scopes})
			pending := mailerOAuthPending{AccountID: "email:fixture-ms", Provider: "microsoft", IdentityKind: "open", Previous: previous, Verifier: "fixture-verifier", Scopes: microsoftMailerScopes(account)}
			err := s.completeMailerMicrosoftOAuth(context.Background(), pending, "fixture-code")
			assertMailerOAuthGrantPublication(t, s, key, previous, tc.scopes, err, tc.wantError)
		})
	}
}

func TestMailerWorkspaceOAuthAcceptsEffectiveRequestedCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		readOnly, send, modify bool
		granted                string
		wantError              bool
	}{
		{name: "requested read"},
		{name: "requested modify", modify: true},
		{name: "requested send and modify", send: true, modify: true},
		{name: "readonly with stored write flags", readOnly: true, send: true, modify: true},
		{name: "modify includes send and read", send: true, modify: true, granted: "https://www.googleapis.com/auth/gmail.modify"},
		{name: "full mail includes requested capabilities", send: true, modify: true, granted: "https://mail.google.com/"},
		{name: "missing modify", modify: true, granted: "https://www.googleapis.com/auth/gmail.readonly", wantError: true},
		{name: "missing send", send: true, granted: "https://www.googleapis.com/auth/gmail.readonly", wantError: true},
		{name: "missing read", send: true, granted: "https://www.googleapis.com/auth/gmail.send", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailerOAuthTestServer(t)
			s.Cfg.Auth.Enabled = false
			s.Mailer = &mailer.Service{}
			gw := &s.Cfg.GoogleWorkspace
			gw.Enabled, gw.Gmail, gw.ClientID = true, true, "fixture-client"
			gw.ReadOnly, gw.GmailSend, gw.GmailModifyLabels = tc.readOnly, tc.send, tc.modify
			if err := s.Vault.WriteSecret("google_workspace_client_secret", "fixture-client-secret"); err != nil {
				t.Fatal(err)
			}
			key, previous := "oauth_google_workspace", `{"pending_login":"fixture"}`
			if err := s.Vault.WriteSecret(key, previous); err != nil {
				t.Fatal(err)
			}
			requested := s.Cfg.FindProvider("google_workspace").OAuthScopes
			granted := tc.granted
			if granted == "" {
				granted = requested
			}
			stubMailerOAuthResponse(t, mailerGoogleTokenURL, tokenExchangeResponse{AccessToken: "fixture-access", RefreshToken: "fixture-refresh", TokenType: "Bearer", ExpiresIn: 3600, Scope: granted})
			pending := mailerOAuthPending{AccountID: "google_workspace:default", Provider: "google_workspace", IdentityKind: "open", Previous: previous, Verifier: "fixture-verifier", Scopes: strings.Fields(requested)}
			err := s.completeMailerWorkspaceOAuth(context.Background(), pending, "fixture-code")
			assertMailerOAuthGrantPublication(t, s, key, previous, granted, err, tc.wantError)
		})
	}
}

func assertMailerOAuthGrantPublication(t *testing.T, s *Server, key, previous, granted string, completionErr error, wantError bool) {
	t.Helper()
	if (completionErr != nil) != wantError {
		t.Fatalf("completion error=%v, want error=%v", completionErr, wantError)
	}
	raw, err := s.Vault.ReadSecret(key)
	if err != nil {
		t.Fatal(err)
	}
	if wantError {
		if raw != previous {
			t.Fatal("rejected grant changed the existing Vault value")
		}
		return
	}
	var stored config.OAuthToken
	if json.Unmarshal([]byte(raw), &stored) != nil || stored.AccessToken != "fixture-access" || stored.RefreshToken != "fixture-refresh" || !sameScopeSet(stored.Scopes, strings.Fields(granted)) {
		t.Fatal("successful grant did not store the actual scopes and tokens")
	}
}

func TestMailerOAuthIdentityUsesDesktopAdministratorScopeAndRevocation(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	tm, err := security.NewTokenManager(s.Vault, filepath.Join(t.TempDir(), "tokens.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s.TokenManager = tm
	for _, scope := range []string{desktopScopeAdmin, desktopScopeRead, desktopScopeWrite, "admin"} {
		t.Run(scope, func(t *testing.T) {
			raw, meta, err := tm.Create("fixture", []string{scope}, nil)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPost, "/api/desktop/mailer/oauth/start", nil)
			r.Header.Set("Authorization", "Bearer "+raw)
			allowed := scope == desktopScopeAdmin
			if authenticateDesktopPermission(s, httptest.NewRecorder(), r, desktopScopeAdmin) != allowed {
				t.Fatal("fixture does not match route authorization")
			}
			kind, identity, ok := s.mailerOAuthIdentity(r, s.Cfg)
			if ok != allowed {
				t.Fatalf("OAuth identity accepted=%v, route accepted=%v", ok, allowed)
			}
			pending := mailerOAuthPending{IdentityKind: kind, Identity: identity}
			if s.mailerOAuthIdentityStillValid(pending) != allowed {
				t.Fatal("callback identity validation differs from start validation")
			}
			if err := tm.Delete(meta.ID); err != nil {
				t.Fatal(err)
			}
			if s.mailerOAuthIdentityStillValid(pending) {
				t.Fatal("revoked administrator token remains valid for an OAuth callback")
			}
		})
	}
}
