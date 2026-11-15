package server

import (
	"aurago/internal/config"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthStatusSessionScopeTracksValidatedSessionIdentity(t *testing.T) {
	const (
		secret = "notification-scope-test-secret"
		nonce  = "00112233445566778899aabbccddeeff"
	)
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = secret
	wantHash := sha256.Sum256([]byte(nonce))
	wantScope := hex.EncodeToString(wantHash[:])

	for _, expiry := range []time.Time{time.Now().Add(time.Hour), time.Now().Add(2 * time.Hour)} {
		value := signSessionValue(secret, expiry, nonce)
		req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})
		rec := httptest.NewRecorder()
		handleAuthStatus(s).ServeHTTP(rec, req)

		var status map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status["session_scope"] != wantScope || status["authenticated"] != true {
			t.Fatalf("status = %#v, want stable validated session scope", status)
		}
		if strings.Contains(rec.Body.String(), value) || strings.Contains(rec.Body.String(), nonce) {
			t.Fatal("auth status exposed the session cookie or nonce")
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("auth status with a session scope must not be cached")
		}
	}
}

func TestAuthStatusOmitsSessionScopeWhenDisabledOrInvalid(t *testing.T) {
	const secret = "notification-scope-disabled-test-secret"
	for _, tc := range []struct {
		name    string
		enabled bool
		cookie  string
	}{
		{name: "disabled", enabled: false},
		{name: "missing cookie", enabled: true},
		{name: "invalid cookie", enabled: true, cookie: "not-a-session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{Cfg: &config.Config{}}
			s.Cfg.Auth.Enabled = tc.enabled
			s.Cfg.Auth.SessionSecret = secret
			req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: tc.cookie})
			}
			rec := httptest.NewRecorder()
			handleAuthStatus(s).ServeHTTP(rec, req)

			var status map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if _, exists := status["session_scope"]; exists {
				t.Fatalf("session_scope unexpectedly present: %#v", status["session_scope"])
			}
			if status["authenticated"] != false {
				t.Fatalf("authenticated = %v, want false", status["authenticated"])
			}
		})
	}
}
