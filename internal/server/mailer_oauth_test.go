package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/i18n"
	"aurago/internal/mailer"
	"aurago/internal/security"
)

func newMailerOAuthTestServer(t *testing.T) *Server {
	t.Helper()
	s := newDesktopOfficeTestServer(t)
	s.Cfg.Server.Host = "127.0.0.1"
	s.Cfg.Server.Port = 8443
	s.Cfg.Server.HTTPS.Enabled = true
	s.Cfg.Server.OAuthRedirectBaseURL = "https://mailer.example.test"
	s.Cfg.Agent.AllowNetworkRequests = true
	s.Cfg.VirtualDesktop.Enabled = true
	var err error
	s.Vault, err = security.NewVault(strings.Repeat("cd", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMailerOAuthRedirectUsesConfiguredOrigin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8443
	cfg.Server.HTTPS.Enabled = true
	r := httptest.NewRequest("GET", "http://attacker.example/api/start", nil)
	r.Header.Set("X-Forwarded-Proto", "http")
	if got := mailerOAuthRedirect(r, cfg); got != "https://localhost:8443"+mailerOAuthCallbackPath {
		t.Fatalf("untrusted request origin affected OAuth redirect: %q", got)
	}
	cfg.Server.OAuthRedirectBaseURL = "https://mailer.example.test"
	if got := mailerOAuthRedirect(r, cfg); got != "https://mailer.example.test"+mailerOAuthCallbackPath {
		t.Fatalf("configured OAuth redirect was not retained: %q", got)
	}
	cfg.Server.OAuthRedirectBaseURL = "https://user:pass@mailer.example.test"
	if got := mailerOAuthRedirect(r, cfg); got != "" {
		t.Fatalf("credential-bearing OAuth redirect accepted: %q", got)
	}
}

func TestMailerOAuthResultEscapesTranslatedMessage(t *testing.T) {
	html := mailerOAuthResultHTML("ok", `<img src=x onerror="alert(1)">`)
	if strings.Contains(html, `<img src=x`) || !strings.Contains(html, `&lt;img src=x onerror=&#34;alert(1)&#34;&gt;`) {
		t.Fatalf("OAuth result did not escape message text: %s", html)
	}
	if !strings.Contains(html, `<title>Mailer</title>`) || !strings.Contains(html, `status:"ok"`) {
		t.Fatalf("OAuth result lost its fixed title or trusted status: %s", html)
	}
}

func TestMailerGoogleAPIScopesRequestFullMailOnlyForGrantedPermanentDelete(t *testing.T) {
	base := config.GogAccount{Services: map[string]config.GogGrants{"gmail": {Read: true, Send: true}}}
	baseScopes := strings.Join(mailerGoogleAPIScopes(base, false), " ")
	if strings.Contains(baseScopes, "mail.google.com") {
		t.Fatalf("Mailer requested permanent-delete scope without a GoG delete grant: %q", baseScopes)
	}
	base.Services["gmail"] = config.GogGrants{Read: true, Delete: true}
	mailerScopes := strings.Join(mailerGoogleAPIScopes(base, false), " ")
	if !strings.Contains(mailerScopes, "https://mail.google.com/") || strings.Contains(mailerScopes, "gmail.") {
		t.Fatalf("Mailer did not request full mail for explicit existing delete grant: %q", mailerScopes)
	}
	nativeScopes := strings.Join(gog.Scopes(base, false), " ")
	if strings.Contains(nativeScopes, "mail.google.com") || !strings.Contains(nativeScopes, "gmail.modify") {
		t.Fatalf("native GoG scope request broadened its delete grant: %q", nativeScopes)
	}
	readonlyScopes := strings.Join(mailerGoogleAPIScopes(base, true), " ")
	if strings.Contains(readonlyScopes, "mail.google.com") {
		t.Fatalf("read-only account requested permanent-delete scope: %q", readonlyScopes)
	}
}

func TestGoogleWorkspaceMailerTokenUsesActualScopesAndConfiguredGrants(t *testing.T) {
	tests := []struct {
		name, grant string
		scopes      []string
		grantSend   bool
		grantModify bool
		wantErr     string
	}{
		{name: "send scope", grant: "send", scopes: []string{"https://www.googleapis.com/auth/gmail.send"}, grantSend: true},
		{name: "compose scope sends", grant: "send", scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}, grantSend: true},
		{name: "modify scope sends", grant: "send", scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}, grantSend: true},
		{name: "labels scope creates label", grant: "create", scopes: []string{"https://www.googleapis.com/auth/gmail.labels"}, grantModify: true},
		{name: "compose cannot create label", grant: "create", scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}, grantModify: true, wantErr: "missing_scope"},
		{name: "provider scope cannot bypass local grant", grant: "create", scopes: []string{"https://www.googleapis.com/auth/gmail.labels"}, wantErr: "permission_denied"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailerOAuthTestServer(t)
			s.Cfg.GoogleWorkspace.Enabled = true
			s.Cfg.GoogleWorkspace.Gmail = true
			s.Cfg.GoogleWorkspace.GmailSend = tc.grantSend
			s.Cfg.GoogleWorkspace.GmailModifyLabels = tc.grantModify
			s.Cfg.GoogleWorkspace.ClientID = "workspace-client"
			s.Cfg.GoogleWorkspace.ClientSecret = "workspace-secret"
			if err := s.Vault.WriteSecret("google_workspace_client_secret", "workspace-secret"); err != nil {
				t.Fatal(err)
			}
			token := config.OAuthToken{AccessToken: "fixture-workspace-access", RefreshToken: "fixture-workspace-refresh",
				Expiry: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Scopes: tc.scopes}
			if err := s.Vault.WriteSecret("oauth_google_workspace", marshalOAuthToken(token)); err != nil {
				t.Fatal(err)
			}
			got, err := s.googleWorkspaceMailerToken(context.Background(), tc.grant, s.ConfigSnapshot())
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr || got != "" {
					t.Fatalf("token=%q err=%v, want %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != token.AccessToken {
				t.Fatalf("token=%q err=%v", got, err)
			}
		})
	}
}

func TestGoogleWorkspaceMailerScopeCapabilitiesFailClosedAndStayPassive(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	s.Cfg.GoogleWorkspace.Enabled = true
	s.Cfg.GoogleWorkspace.Gmail = true
	if got := s.googleWorkspaceMailerScopeCapabilities(); got != (gog.MailerGmailScopes{}) {
		t.Fatalf("missing OAuth token produced capabilities: %+v", got)
	}
	token := config.OAuthToken{AccessToken: "fixture-workspace-access", Scopes: []string{"https://www.googleapis.com/auth/gmail.labels"}}
	if err := s.Vault.WriteSecret("oauth_google_workspace", marshalOAuthToken(token)); err != nil {
		t.Fatal(err)
	}
	got := s.googleWorkspaceMailerScopeCapabilities()
	if got.Read || got.Send || got.Modify || !got.CreateLabel || got.PermanentDelete || got.Protocol {
		t.Fatalf("labels-only token produced incorrect scope capabilities: %+v", got)
	}
	token.Scopes = []string{"https://www.googleapis.com/auth/gmail.modify"}
	if err := s.Vault.WriteSecret("oauth_google_workspace", marshalOAuthToken(token)); err != nil {
		t.Fatal(err)
	}
	got = s.googleWorkspaceMailerScopeCapabilities()
	if !got.Read || !got.Send || !got.Modify || !got.CreateLabel || got.PermanentDelete || got.Protocol {
		t.Fatalf("gmail.modify token produced incorrect scope capabilities: %+v", got)
	}
}

func TestMailerOAuthStartCreatesBoundPKCEFlows(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	google := config.GogAccount{ID: "google", Enabled: true, DesktopAccess: true, ClientID: "gog-client"}
	s.Cfg.Gog = config.GogConfig{Enabled: true, Accounts: []config.GogAccount{google}}
	if err := s.Vault.WriteSecret(gog.ClientKey(google.ID), "gog-client-secret-fixture"); err != nil {
		t.Fatal(err)
	}
	s.Cfg.EmailAccounts = []config.EmailAccount{
		{ID: "protocol", ReceiveProtocol: "imap", OAuthSource: "gog", OAuthProvider: "google", OAuthAccountID: google.ID},
		{ID: "microsoft", ReceiveProtocol: "pop3", OAuthSource: "mailer", OAuthProvider: "microsoft", OAuthClientID: "microsoft-client"},
	}

	cases := []struct {
		name, accountID, provider string
		scopes                    []string
		gog                       *config.GogAccount
	}{
		{name: "Google API", accountID: "gog:google", provider: "gog", scopes: []string{"https://www.googleapis.com/auth/gmail.readonly"}, gog: &google},
		{name: "Google IMAP", accountID: "email:protocol", provider: "gog_protocol", scopes: []string{"https://mail.google.com/"}, gog: &google},
		{name: "Microsoft POP3 and SMTP", accountID: "email:microsoft", provider: "microsoft", scopes: microsoftMailerScopes(s.Cfg.EmailAccounts[1])},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", "http://attacker.example/api/start", nil)
			request.Header.Set("X-Forwarded-Host", "attacker.example")
			response := httptest.NewRecorder()
			result, err := s.beginMailerOAuth(response, request, tc.accountID, tc.provider, tc.scopes, tc.gog, "")
			if err != nil {
				t.Fatal(err)
			}
			authURL, err := url.Parse(result["auth_url"])
			if err != nil || result["auth_url"] == "" || result["redirect_uri"] != "https://mailer.example.test"+mailerOAuthCallbackPath {
				t.Fatalf("invalid start response: %#v err=%v", result, err)
			}
			query := authURL.Query()
			if query.Get("state") == "" || query.Get("client_id") == "" || query.Get("redirect_uri") != result["redirect_uri"] || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" {
				t.Fatalf("OAuth URL omitted binding/PKCE fields: %s", authURL)
			}
			for _, scope := range tc.scopes {
				if !strings.Contains(query.Get("scope"), scope) {
					t.Fatalf("requested scope %q missing from %q", scope, query.Get("scope"))
				}
			}
			secret, err := s.Vault.ReadSecret(mailerOAuthStatePrefix + query.Get("state"))
			if err != nil {
				t.Fatal(err)
			}
			var pending mailerOAuthPending
			if err := json.Unmarshal([]byte(secret), &pending); err != nil || pending.State != query.Get("state") || pending.AccountID != tc.accountID || pending.Provider != tc.provider || pending.Verifier == "" || pending.IdentityKind != "open" {
				t.Fatalf("OAuth state did not bind expected account/identity: %#v err=%v", pending, err)
			}
			key, err := mailerOAuthTokenKey(pending)
			if err != nil || pending.Previous == "" {
				t.Fatalf("first-login flow did not pin a token marker: key=%q previous=%q err=%v", key, pending.Previous, err)
			}
			stored, err := s.Vault.ReadSecret(key)
			if err != nil || stored != pending.Previous {
				t.Fatal("OAuth start did not atomically reserve the first-login token slot")
			}
			cookies := response.Result().Cookies()
			if len(cookies) != 1 || cookies[0].Name != mailerOAuthCookieName || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || !cookies[0].Secure || cookies[0].Path != mailerOAuthCallbackPath {
				t.Fatalf("unexpected nonce cookie: %#v", cookies)
			}
		})
	}
}

func TestMailerOAuthFirstLoginPublicationCannotResurrectOrOverwrite(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	google := config.GogAccount{ID: "google", Enabled: true, DesktopAccess: true, ClientID: "gog-client"}
	start := func() mailerOAuthPending {
		w := httptest.NewRecorder()
		result, err := s.beginMailerOAuth(w, httptest.NewRequest("POST", "https://mailer.example.test/start", nil), "gog:google", "gog", []string{"https://www.googleapis.com/auth/gmail.readonly"}, &google, "")
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(result["auth_url"])
		raw, err := s.Vault.ReadSecret(mailerOAuthStatePrefix + u.Query().Get("state"))
		if err != nil {
			t.Fatal(err)
		}
		var pending mailerOAuthPending
		if err := json.Unmarshal([]byte(raw), &pending); err != nil {
			t.Fatal(err)
		}
		return pending
	}
	first, second := start(), start()
	key, err := mailerOAuthTokenKey(first)
	if err != nil || key != gog.TokenKey(google.ID) || first.Previous == "" || second.Previous != first.Previous {
		t.Fatalf("parallel first-login starts did not share exact CAS predecessor: first=%q second=%q key=%q err=%v", first.Previous, second.Previous, key, err)
	}
	if err := s.Vault.DeleteSecret(key); err != nil {
		t.Fatal(err)
	}
	if err := s.publishMailerOAuthToken(key, first.Previous, `{"access_token":"fixture-revived"}`); err == nil {
		t.Fatal("revoked first-login callback recreated the token")
	}
	third := start()
	if err := s.publishMailerOAuthToken(key, third.Previous, `{"access_token":"fixture-first-winner"}`); err != nil {
		t.Fatalf("first completion failed: %v", err)
	}
	if err := s.publishMailerOAuthToken(key, third.Previous, `{"access_token":"fixture-late-overwrite"}`); err == nil {
		t.Fatal("parallel completion overwrote the first published token")
	}
	stored, err := s.Vault.ReadSecret(key)
	if err != nil || stored != `{"access_token":"fixture-first-winner"}` {
		t.Fatalf("stored token changed after losing completion: %q err=%v", stored, err)
	}
}

func TestMailerOAuthCallbackConsumesStateOnceAndRejectsReplay(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	google := config.GogAccount{ID: "google", Enabled: true, DesktopAccess: true, ClientID: "gog-client"}
	response := httptest.NewRecorder()
	result, err := s.beginMailerOAuth(response, httptest.NewRequest("POST", "http://attacker.example/start", nil), "gog:google", "gog", []string{"https://www.googleapis.com/auth/gmail.readonly"}, &google, "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(result["auth_url"])
	state := parsed.Query().Get("state")
	cookie := response.Result().Cookies()[0]
	s.Mailer = &mailer.Service{}
	for attempt := 0; attempt < 2; attempt++ {
		callback := httptest.NewRequest("GET", "https://attacker.example"+mailerOAuthCallbackPath+"?state="+url.QueryEscape(state)+"&error=access_denied", nil)
		callback.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		s.handleMailerOAuthCallback(recorder, callback)
		if recorder.Code != 400 || !strings.Contains(recorder.Body.String(), "aurago-mailer-oauth") {
			t.Fatalf("callback attempt %d returned status %d: %s", attempt, recorder.Code, recorder.Body.String())
		}
		if attempt == 0 && !strings.Contains(recorder.Body.String(), i18n.T(normalizeLang(s.Cfg.Agent.SystemLanguage), "mailer.oauth_failed")) {
			t.Fatalf("first callback did not consume valid denied state: %s", recorder.Body.String())
		}
	}
}
