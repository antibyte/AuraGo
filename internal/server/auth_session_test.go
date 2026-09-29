package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestAuthSessionActivity(t *testing.T) {
	t.Parallel()
	const secret = "session-renewal-test-secret"
	for _, tt := range []struct {
		name, method, origin, referer, token string
		remaining                            time.Duration
		disabled, strict, emptySecret        bool
		status                               int
		renew                                bool
	}{
		{name: "active near expiry", remaining: time.Minute, status: 200, renew: true},
		{name: "preserve initial duration", remaining: time.Hour, status: 200},
		{name: "no accumulated extensions", remaining: 9 * time.Minute, status: 200, renew: true},
		{name: "expired cannot revive", remaining: -time.Minute, status: 401},
		{name: "missing cookie", token: "missing", status: 401},
		{name: "malformed cookie", token: "malformed", status: 401},
		{name: "tampered cookie", token: "tampered", remaining: time.Minute, status: 401},
		{name: "rotated secret", token: "rotated", remaining: time.Minute, status: 401},
		{name: "empty secret", emptySecret: true, remaining: time.Minute, status: 401},
		{name: "foreign origin", origin: "https://foreign.test", remaining: time.Minute, status: 403},
		{name: "missing origin", origin: "none", remaining: time.Minute, status: 403},
		{name: "same origin referer", origin: "none", referer: "https://aurago.test/desktop", remaining: time.Minute, status: 200, renew: true},
		{name: "strict origin", origin: "none", referer: "https://aurago.test/desktop", strict: true, remaining: time.Minute, status: 403},
		{name: "get cannot renew", method: "GET", remaining: time.Minute, status: 405},
		{name: "disabled cannot issue cookie", disabled: true, token: "missing", status: 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
			s.Cfg.Auth.Enabled = !tt.disabled
			s.Cfg.Auth.PasswordHash = "configured"
			s.Cfg.Auth.SessionSecret = secret
			s.Cfg.Auth.RequireOriginHeader = tt.strict
			if tt.emptySecret {
				s.Cfg.Auth.SessionSecret = ""
			}
			method := tt.method
			if method == "" {
				method = http.MethodPost
			}
			r := httptest.NewRequest(method, "https://aurago.test/api/auth/activity", nil)
			origin := tt.origin
			if origin == "" {
				origin = "https://aurago.test"
			}
			if origin != "none" {
				r.Header.Set("Origin", origin)
			}
			r.Header.Set("Referer", tt.referer)
			expiry := time.Now().Add(tt.remaining).Truncate(time.Second)
			value := createSessionValue(s.Cfg.Auth.SessionSecret, expiry)
			switch tt.token {
			case "malformed":
				value = "not-a-session"
			case "tampered":
				value += "a"
			case "rotated":
				value = createSessionValue("previous-test-secret", expiry)
			}
			if tt.token != "missing" {
				r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})
			}
			rec := httptest.NewRecorder()
			authMiddleware(s, handleAuthActivity(s)).ServeHTTP(rec, r)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			cookies := rec.Result().Cookies()
			if !tt.renew {
				if len(cookies) != 0 {
					t.Fatal("request unexpectedly changed the session cookie")
				}
			} else {
				if len(cookies) != 1 {
					t.Fatalf("cookies = %d, want 1", len(cookies))
				}
				c := cookies[0]
				if c.Name != sessionCookieName || c.Path != "/" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
					t.Fatal("renewal lost session cookie security attributes")
				}
				remaining := time.Until(sessionExpiry(secret, c.Value))
				if remaining < 599*time.Second || remaining > 10*time.Minute {
					t.Fatalf("renewed lifetime = %v, want ten minutes", remaining)
				}
				if !sessionExpiry(secret, c.Value).Equal(c.Expires) {
					t.Fatal("browser and signed session expiry differ")
				}
			}
			if rec.Code == http.StatusOK && !tt.disabled {
				var status struct {
					Authenticated bool    `json:"authenticated"`
					Remaining     float64 `json:"expires_in_seconds"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil || !status.Authenticated || status.Remaining < 599 {
					t.Fatalf("invalid activity status: %v", err)
				}
				if rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), value) {
					t.Fatal("activity response is cacheable or exposes the session credential")
				}
			}
		})
	}
}

func TestAuthSessionPollingDoesNotRenew(t *testing.T) {
	t.Parallel()
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "session-status-test-secret"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/status", handleAuthStatus(s))
	mux.HandleFunc("/api/poll", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	handler := authMiddleware(s, mux)
	for _, remaining := range []time.Duration{time.Minute, -time.Second} {
		for _, path := range []string{"/api/auth/status", "/api/poll"} {
			r := httptest.NewRequest(http.MethodGet, "https://aurago.test"+path, nil)
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(remaining))})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, r)
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("background request renewed the session")
			}
			if path == "/api/auth/status" {
				var status struct {
					Authenticated bool    `json:"authenticated"`
					Remaining     float64 `json:"expires_in_seconds"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil || status.Authenticated != (remaining > 0) {
					t.Fatalf("status does not reflect expiry: %v", err)
				}
				if status.Remaining < 0 || status.Remaining > 60 || (remaining < 0 && status.Remaining != 0) {
					t.Fatalf("invalid remaining lifetime: %v", status.Remaining)
				}
				if rec.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("auth status must never be cached")
				}
			} else if remaining < 0 && rec.Code != http.StatusUnauthorized {
				t.Fatal("idle expired session still grants API access")
			}
		}
	}
}
