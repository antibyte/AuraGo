package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func sessionCookies(rec *httptest.ResponseRecorder) []*http.Cookie {
	var out []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			out = append(out, c)
		}
	}
	return out
}

func signedInFlag(t *testing.T, rec *httptest.ResponseRecorder) bool {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v body=%s", err, rec.Body.String())
	}
	v, present := payload["signed_in"].(bool)
	if !present {
		t.Fatalf("response lacks signed_in: %s", rec.Body.String())
	}
	return v
}

func TestSetupSaveSignsInTheBrowserThatSetThePassword(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1", false)
	addSetupCSRFTokenForTest(s, "csrf-session")

	rec := postSetup(s, "127.0.0.1:50000", setupClaimPatch(t), map[string]string{
		"X-CSRF-Token":   "csrf-session",
		"Origin":         "http://example.com",
		"Sec-Fetch-Site": "same-origin",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !signedInFlag(t, rec) {
		t.Fatal("signed_in = false, want true")
	}
	cookies := sessionCookies(rec)
	if len(cookies) != 1 {
		t.Fatalf("got %d %s cookies, want exactly 1", len(cookies), sessionCookieName)
	}
	c := cookies[0]
	if c.Name != "aurago_session" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Secure {
		t.Fatalf("unexpected cookie attributes: %+v", c)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	if !IsAuthenticated(req, s.Cfg.Auth.SessionSecret) {
		t.Fatal("issued cookie does not authenticate against the new session secret")
	}
}

func TestSetupSaveSkipsSessionForForeignOrigin(t *testing.T) {
	s := newBootstrapTestServer(t, "127.0.0.1", false)
	addSetupCSRFTokenForTest(s, "csrf-foreign")

	rec := postSetup(s, "127.0.0.1:50000", setupClaimPatch(t), map[string]string{
		"X-CSRF-Token": "csrf-foreign",
		"Origin":       "http://evil.example",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if s.Cfg.Auth.PasswordHash == "" {
		t.Fatal("setup was not saved")
	}
	if signedInFlag(t, rec) {
		t.Fatal("signed_in = true for a foreign origin")
	}
	if got := sessionCookies(rec); len(got) != 0 {
		t.Fatalf("foreign origin received cookies: %v", got)
	}
}

func TestSetupSaveSkipsSessionWhenPasswordAlreadySet(t *testing.T) {
	defer withDebugForceSetup(true)()
	s := newBootstrapTestServer(t, "127.0.0.1", false)
	hash, err := HashPassword("existing-password")
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg.Auth.PasswordHash = hash
	s.Cfg.Auth.SessionSecret = "existing-secret"
	addSetupCSRFTokenForTest(s, "csrf-existing")

	rec := postSetup(s, "127.0.0.1:50000", `{"auth":{"enabled":true},"server":{"ui_language":"en"}}`, map[string]string{
		"X-CSRF-Token": "csrf-existing",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if signedInFlag(t, rec) {
		t.Fatal("signed_in = true although no password was initialized")
	}
	if got := sessionCookies(rec); len(got) != 0 {
		t.Fatalf("received cookies without a new password: %v", got)
	}
}
