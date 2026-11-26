package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestMailerHTTPOversizeBodyIs413(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	got := mailerHTTPRequest(s, "POST", "drafts", `{"account_id":"fixture","subject":"`+strings.Repeat("x", 2<<20)+`"}`)
	if got.Code != 413 || !strings.Contains(got.Body.String(), "too_large") {
		t.Fatalf("oversize draft = %d %s, want 413 too_large", got.Code, got.Body)
	}
}

func TestMailerHTTPAccountFoldersGET(t *testing.T) {
	s, _ := newMailerHTTPTestServer(t)
	got := mailerHTTPRequest(s, "GET", "accounts/fixture/folders", "")
	if got.Code != 200 {
		t.Fatalf("GET folders = %d %s", got.Code, got.Body)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(got.Body.Bytes(), &payload); err != nil || string(payload["folders"]) == "" || string(payload["folders"]) == "null" {
		t.Fatalf("folders payload = %s err=%v, want {\"folders\":[...]}", got.Body, err)
	}
}

func TestMicrosoftMailerScopesHonourReadOnly(t *testing.T) {
	const send = "https://outlook.office.com/SMTP.Send"
	for _, protocol := range []string{"imap", "pop3"} {
		rw := config.EmailAccount{ReceiveProtocol: protocol}
		ro := config.EmailAccount{ReceiveProtocol: protocol, ReadOnly: true}
		if !strings.Contains(strings.Join(microsoftMailerScopes(rw), " "), send) || !strings.Contains(strings.Join(microsoftMailerRequiredScopes(rw), " "), send) {
			t.Fatalf("%s writable account must request and require SMTP.Send", protocol)
		}
		if strings.Contains(strings.Join(microsoftMailerScopes(ro), " "), send) || strings.Contains(strings.Join(microsoftMailerRequiredScopes(ro), " "), send) {
			t.Fatalf("%s read-only account must not request or require SMTP.Send", protocol)
		}
		if len(microsoftMailerRequiredScopes(ro)) != 1 {
			t.Fatalf("%s read-only required scopes = %v", protocol, microsoftMailerRequiredScopes(ro))
		}
	}
}

func TestMailerOAuthStateIsDeletedOnConsumeAndExpiredStatesAreReaped(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	google := config.GogAccount{ID: "google", Enabled: true, DesktopAccess: true, ClientID: "gog-client"}
	begin := func() (string, string) {
		w := httptest.NewRecorder()
		result, err := s.beginMailerOAuth(w, httptest.NewRequest("POST", "https://mailer.example.test/start", nil), "gog:google", "gog", []string{"https://www.googleapis.com/auth/gmail.readonly"}, &google, "")
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(result["auth_url"])
		return u.Query().Get("state"), w.Result().Cookies()[0].Value
	}
	state, nonce := begin()
	// An expired leftover from an abandoned flow.
	stale := mailerOAuthStatePrefix + strings.Repeat("a", 64)
	data, _ := json.Marshal(mailerOAuthPending{Version: 1, State: strings.Repeat("a", 64), ExpiresAt: time.Now().Add(-time.Hour)})
	if err := s.Vault.WriteSecret(stale, string(data)); err != nil {
		t.Fatal(err)
	}
	begin() // a new start reaps the stale entry
	if raw, _ := s.Vault.ReadSecret(stale); raw != "" {
		t.Fatalf("expired pending state survived the next OAuth start")
	}
	if raw, _ := s.Vault.ReadSecret(mailerOAuthStatePrefix + state); raw == "" {
		t.Fatal("a live pending state was reaped")
	}
	req := httptest.NewRequest("GET", "https://mailer.example.test"+mailerOAuthCallbackPath+"?state="+state, nil)
	req.AddCookie(&http.Cookie{Name: mailerOAuthCookieName, Value: nonce})
	if _, err := s.consumeMailerOAuthPending(req, state); err != nil {
		t.Fatalf("consume: %v", err)
	}
	keys, _ := s.Vault.ListKeys()
	for _, k := range keys {
		if k == mailerOAuthStatePrefix+state {
			t.Fatal("consumed OAuth state key still exists in the Vault")
		}
	}
}

func TestMailerOAuthCookieIgnoresUntrustedForwardedProto(t *testing.T) {
	r := httptest.NewRequest("GET", "http://mailer.example.test/x", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	mailerOAuthSetCookie(w, r, &config.Config{}, "nonce")
	if c := w.Result().Cookies(); len(c) != 1 || c[0].Secure {
		t.Fatalf("an untrusted X-Forwarded-Proto set Secure: %#v", c)
	}
}
