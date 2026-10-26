package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestWebAuthnEnrollmentSessionRenewal(t *testing.T) {
	for _, scenario := range []string{"renewal", "legacy renewal", "other login", "logout renewed cookie", "logout original cookie", "password change"} {
		t.Run(scenario, func(t *testing.T) {
			s, original := newWebAuthnTestServer(t, "passkey")
			original.Value = createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Minute))
			if scenario == "legacy renewal" {
				payload := base64.URLEncoding.EncodeToString(fmt.Appendf(nil, "user|%d", time.Now().Add(time.Minute).Unix()))
				mac := hmac.New(sha256.New, []byte(s.Cfg.Auth.SessionSecret))
				mac.Write([]byte(payload))
				original.Value = payload + "." + hex.EncodeToString(mac.Sum(nil))
			}
			id, challenge, browser := webAuthnTestBegin(t, s, true, original, "")
			renewal := webAuthnTestRequest(t, handleAuthActivity(s), http.MethodPost, "/api/auth/activity", nil, original)
			var current *http.Cookie
			for _, cookie := range renewal.Result().Cookies() {
				if cookie.Name == sessionCookieName {
					current = cookie
				}
			}
			if renewal.Code != http.StatusOK || current == nil {
				t.Fatal("activity did not renew the owner session")
			}
			want := http.StatusUnauthorized
			switch scenario {
			case "renewal", "legacy renewal":
				want = http.StatusOK
			case "other login":
				current.Value = createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))
			case "logout renewed cookie", "logout original cookie":
				logoutCookie := current
				if scenario == "logout original cookie" {
					logoutCookie = original
				}
				logout := webAuthnTestRequest(t, handleAuthLogoutAPI(s), http.MethodPost, "/api/auth/logout", nil, logoutCookie)
				if logout.Code != http.StatusOK {
					t.Fatalf("logout status = %d", logout.Code)
				}
				for _, cookie := range []*http.Cookie{original, current} {
					if validateSessionValue(s.Cfg.Auth.SessionSecret, cookie.Value) {
						t.Fatal("logout left a version of the renewed session usable")
					}
				}
				revokedSessions.Lock()
				until := revokedSessions.entries[sessionRevocationKey(s.Cfg.Auth.SessionSecret, original.Value)]
				revokedSessions.Unlock()
				if until.Before(sessionExpiry(s.Cfg.Auth.SessionSecret, current.Value)) {
					t.Fatal("revocation expires before the renewed cookie")
				}
				retry := webAuthnTestRequest(t, handleAuthActivity(s), http.MethodPost, "/api/auth/activity", nil, current)
				if retry.Code != http.StatusUnauthorized || len(retry.Result().Cookies()) != 0 {
					t.Fatal("logout session was renewed again")
				}
			case "password change":
				s.Cfg.Auth.SessionSecret = "replacement-session-secret"
			}
			key, credential := webAuthnTestKey(t)
			response := webAuthnTestCredential(t, key, credential, challenge, nil, true, true)
			finish := webAuthnTestRequest(t, handleWebAuthnFinish(s, true), http.MethodPost, "/api/auth/webauthn/register/finish", map[string]any{"transaction": id, "credential": response}, current, browser)
			if finish.Code != want {
				t.Fatalf("finish status = %d, want %d: %s", finish.Code, want, finish.Body.String())
			}
		})
	}
}

func TestAuthSessionHonorsLegacyRevocationFile(t *testing.T) {
	s, owner := newWebAuthnTestServer(t, "passkey")
	key := sha256.Sum256([]byte(s.Cfg.Auth.SessionSecret + "\x00" + owner.Value))
	body, err := json.Marshal(map[string]time.Time{hex.EncodeToString(key[:]): time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Cfg.Directories.DataDir, "auth_session_revocations.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadSessionRevocations(s); err != nil {
		t.Fatal(err)
	}
	if validateSessionValue(s.Cfg.Auth.SessionSecret, owner.Value) {
		t.Fatal("a persisted legacy token revocation was ignored")
	}
}

func TestAuthSessionConcurrentRenewalAndLogout(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "synthetic-concurrent-renewal-secret"
	s.Cfg.Directories.DataDir = t.TempDir()
	owner := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Minute))}
	start := make(chan struct{})
	renewed := make(chan *http.Cookie, 20)
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() {
			<-start
			result := webAuthnTestRequest(t, handleAuthActivity(s), http.MethodPost, "/api/auth/activity", nil, owner)
			if result.Code != http.StatusOK && result.Code != http.StatusUnauthorized {
				t.Errorf("activity status = %d", result.Code)
			}
			for _, cookie := range result.Result().Cookies() {
				if cookie.Name == sessionCookieName {
					renewed <- cookie
				}
			}
		})
	}
	workers.Go(func() {
		<-start
		if result := webAuthnTestRequest(t, handleAuthLogoutAPI(s), http.MethodPost, "/api/auth/logout", nil, owner); result.Code != http.StatusOK {
			t.Errorf("logout status = %d", result.Code)
		}
	})
	close(start)
	workers.Wait()
	close(renewed)
	if validateSessionValue(s.Cfg.Auth.SessionSecret, owner.Value) {
		t.Fatal("logout left the original cookie usable")
	}
	for cookie := range renewed {
		if validateSessionValue(s.Cfg.Auth.SessionSecret, cookie.Value) {
			t.Fatal("a concurrent renewal survived logout")
		}
	}
}

func TestWebAuthnBeginRefreshesBrowserBinding(t *testing.T) {
	s, _ := newWebAuthnTestServer(t, "passkey")
	const existing = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	r := httptest.NewRequest(http.MethodPost, "https://localhost/auth/webauthn/login/begin", nil)
	r.AddCookie(&http.Cookie{Name: webAuthnCookie, Value: existing})
	w := httptest.NewRecorder()
	if _, err := s.putWebAuthnTransaction(w, r, webAuthnTransaction{}); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == webAuthnCookie {
			if cookie.Value != existing || cookie.MaxAge < int(webAuthnTTL.Seconds()) || cookie.Path != "/" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatal("begin did not preserve and refresh the protected browser binding")
			}
			return
		}
	}
	t.Fatal("begin did not refresh the existing browser-binding cookie")
}
