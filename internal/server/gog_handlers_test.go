package server

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gog"
)

func gogTestServer(t *testing.T) *Server {
	s, _ := newOAuthHandlerTestServer(t, "")
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.Gog.Enabled = true
	s.Cfg.Agent.AllowNetworkRequests = true
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Gog.Accounts = []config.GogAccount{{ID: "fixture", Name: "Fixture", ClientID: "fixture-client", Enabled: true, DesktopAccess: true, Services: map[string]config.GogGrants{"docs": {Read: true}}}}
	return s
}
func TestGogStatusIsLocalAndSecretsStayPrivate(t *testing.T) {
	s := gogTestServer(t)
	secret := "fixture-secret-private"
	_ = s.Vault.WriteSecret(gog.ClientKey("fixture"), secret)
	for _, route := range []string{"status", "accounts"} {
		w := httptest.NewRecorder()
		handleGog(s)(w, httptest.NewRequest("GET", "/api/gog/"+route, nil))
		if w.Code != 200 || strings.Contains(w.Body.String(), secret) {
			t.Fatalf("unsafe local status %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	handleGog(s)(w, httptest.NewRequest("GET", "/api/gog/test", nil))
	if w.Code != 405 {
		t.Fatal("GET test accepted")
	}
	w = httptest.NewRecorder()
	handleGog(s)(w, httptest.NewRequest("POST", "/api/gog/test", strings.NewReader(`{"account_id":"fixture","service":"docs"}`)))
	if !strings.Contains(w.Body.String(), "test_object_required") {
		t.Fatal(w.Body.String())
	}
}
func TestGogOAuthUsesPKCEAndRejectsWrongCallback(t *testing.T) {
	s := gogTestServer(t)
	_ = s.Vault.WriteSecret(gog.ClientKey("fixture"), "fixture-private")
	w := httptest.NewRecorder()
	handleGog(s)(w, httptest.NewRequest("POST", "http://localhost:8088/api/gog/oauth/start", strings.NewReader(`{"account_id":"fixture"}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	auth, _ := url.Parse(result["auth_url"])
	q := auth.Query()
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" || q.Get("state") == "" || q.Get("access_type") != "offline" {
		t.Fatal("missing OAuth protections")
	}
	state := q.Get("state")
	raw, _ := s.Vault.ReadSecret("oauth_state_" + state)
	var saved gogOAuthSession
	_ = json.Unmarshal([]byte(raw), &saved)
	if saved.CodeVerifier == "" || saved.State != state || saved.ExpiresAt.IsZero() {
		t.Fatal("missing state")
	}
	if saved.Previous == "" {
		t.Fatal("first authorization did not pin an atomic token marker")
	}
	var marker gog.Token
	if json.Unmarshal([]byte(saved.Previous), &marker) != nil || marker.PendingLogin != state || marker.AccessToken != "" {
		t.Fatalf("first-login marker is invalid: %#v", marker)
	}
	if current, err := s.Vault.ReadSecret(gog.TokenKey("fixture")); err != nil || current != saved.Previous {
		t.Fatal("first-login marker was not stored before redirect")
	}
	callback := "https://evil.invalid/api/gog/oauth/callback?state=" + state + "&code=fixture-code"
	body, _ := json.Marshal(map[string]string{"url": callback})
	w = httptest.NewRecorder()
	handleGog(s)(w, httptest.NewRequest("POST", "/api/gog/oauth/manual", strings.NewReader(string(body))))
	if w.Code != 400 {
		t.Fatal("foreign callback accepted")
	}
	after, _ := s.Vault.ReadSecret("oauth_state_" + state)
	if after != raw {
		t.Fatal("bad callback consumed state")
	}
}
func TestGogDesktopReadOnlyAndAccountFiltering(t *testing.T) {
	s := gogTestServer(t)
	s.Cfg.Gog.ReadOnly = true
	s.Cfg.VirtualDesktop.ReadOnly = true
	w := httptest.NewRecorder()
	handleDesktopGog(s)(w, httptest.NewRequest("GET", "/api/desktop/integrations/gog/accounts", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "fixture-client") {
		t.Fatal("unsafe desktop account listing")
	}
	w = httptest.NewRecorder()
	handleDesktopGog(s)(w, httptest.NewRequest("POST", "/api/desktop/integrations/gog/call", strings.NewReader(`{"account_id":"fixture","service":"docs","operation":"create","args":{"title":"Fixture"}}`)))
	if w.Code != 403 {
		t.Fatalf("readonly write accepted: %d", w.Code)
	}
	w = httptest.NewRecorder()
	handleDesktopGog(s)(w, httptest.NewRequest("POST", "/api/desktop/integrations/gog/call", strings.NewReader(`{"account_id":"fixture","service":"raw","operation":"exec"}`)))
	if w.Code != 400 {
		t.Fatal("raw operation accepted")
	}
}

func TestGogClientSecretDisconnectsOldLogin(t *testing.T) {
	s := gogTestServer(t)
	for _, previous := range []string{"", `{"access_token":"fixture-old-token","client_id":"fixture-client"}`} {
		if previous != "" {
			if e := s.Vault.WriteSecret(gog.TokenKey("fixture"), previous); e != nil {
				t.Fatal(e)
			}
		}
		w := httptest.NewRecorder()
		handleGog(s)(w, httptest.NewRequest("POST", "/api/gog/client-secret", strings.NewReader(`{"account_id":"fixture","secret":"fixture-new-secret"}`)))
		if w.Code != 200 || strings.Contains(w.Body.String(), "fixture-new-secret") {
			t.Fatalf("client secret update failed or exposed the secret: %d", w.Code)
		}
		if raw, _ := s.Vault.ReadSecret(gog.TokenKey("fixture")); raw != "" {
			t.Fatal("old login survived client secret change")
		}
		if secret, _ := s.Vault.ReadSecret(gog.ClientKey("fixture")); secret != "fixture-new-secret" {
			t.Fatal("client secret was not saved")
		}
	}
}
