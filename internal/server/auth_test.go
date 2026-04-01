package server

import (
	"aurago/internal/config"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClearSessionCookieIncludesSecureOnHTTPS(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/auth/logout", nil)
	rec := httptest.NewRecorder()

	ClearSessionCookie(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected two cookies for HTTPS logout, got %d", len(cookies))
	}
	var foundSecure bool
	var foundInsecure bool
	for _, c := range cookies {
		if c.Name != sessionCookieName {
			t.Fatalf("expected cookie %q, got %q", sessionCookieName, c.Name)
		}
		if c.Secure {
			foundSecure = true
		} else {
			foundInsecure = true
		}
		if c.MaxAge != -1 {
			t.Fatalf("expected MaxAge -1, got %d", c.MaxAge)
		}
		if c.Path != "/" {
			t.Fatalf("expected path '/', got %q", c.Path)
		}
		if c.Expires.IsZero() {
			t.Fatalf("expected explicit expiry in the past")
		}
	}
	if !foundSecure || !foundInsecure {
		t.Fatalf("expected both secure and insecure logout cookies, got secure=%v insecure=%v", foundSecure, foundInsecure)
	}
}

func TestClearSessionCookieIncludesProxySecureAttribute(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth/logout", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	ClearSessionCookie(rec, req)

	headers := rec.Header().Values("Set-Cookie")
	if len(headers) != 2 {
		t.Fatalf("expected two Set-Cookie headers, got %d", len(headers))
	}
	foundSecure := false
	for _, header := range headers {
		if strings.Contains(header, "Secure") {
			foundSecure = true
			break
		}
	}
	if !foundSecure {
		t.Fatalf("expected at least one secure Set-Cookie header, got %v", headers)
	}
}

func TestHandleAuthLogoutReturnsJSONForAjaxRequests(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handleAuthLogout(&Server{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), sessionCookieName+"=") {
		t.Fatalf("expected logout response to clear session cookie")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"ok\":true") || !strings.Contains(body, "\"redirect\":\"/auth/login\"") {
		t.Fatalf("unexpected JSON body: %s", body)
	}
}

func TestHandleAuthLogoutAPIReturnsJSON(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	handleAuthLogoutAPI(&Server{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"ok\":true") || !strings.Contains(body, "\"redirect\":\"/auth/login\"") {
		t.Fatalf("unexpected JSON body: %s", body)
	}
	if rec.Header().Get("Location") != "" {
		t.Fatalf("did not expect redirect location for API logout")
	}
}

func TestLoginBackoffDelayUsesHighestFailureCount(t *testing.T) {

	loginRecords = make(map[string]*loginRecord)
	ipKey := loginScopeKey("ip", "127.0.0.1")
	accountKey := loginScopeKey("account", "admin")
	RecordFailedLogin(ipKey, 10, 1)
	RecordFailedLogin(ipKey, 10, 1)
	RecordFailedLogin(accountKey, 10, 1)

	delay := LoginBackoffDelay(ipKey, accountKey)
	if delay != 500*time.Millisecond {
		t.Fatalf("delay = %v, want %v", delay, 500*time.Millisecond)
	}
}

func TestHandleAuthLoginLocksOutAcrossAccountScope(t *testing.T) {

	loginRecords = make(map[string]*loginRecord)
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = hash
	cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	cfg.Auth.MaxLoginAttempts = 2
	cfg.Auth.LockoutMinutes = 10
	cfg.Auth.SessionTimeoutHours = 1
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	makeRequest := func(ip string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"password": "wrong"})
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		handleAuthLogin(s).ServeHTTP(rec, req)
		return rec
	}

	if rec := makeRequest("127.0.0.1"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("first login status = %d, want 401", rec.Code)
	}
	if rec := makeRequest("127.0.0.2"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second login on different IP status = %d, want 429", rec.Code)
	}
}

func TestAuthMiddlewareLoopbackFollowUpBypassRequiresLoopbackRemoteAddr(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(s, next)

	externalReq := httptest.NewRequest(http.MethodPost, "/api/invasion/nests/n1/hatch", nil)
	externalReq.Header.Set("X-Internal-FollowUp", "true")
	externalReq.RemoteAddr = "10.0.0.42:1234"
	externalRec := httptest.NewRecorder()
	handler.ServeHTTP(externalRec, externalReq)
	if externalRec.Code != http.StatusUnauthorized {
		t.Fatalf("external follow-up status = %d, want 401", externalRec.Code)
	}

	loopbackReq := httptest.NewRequest(http.MethodPost, "/api/invasion/nests/n1/hatch", nil)
	loopbackReq.Header.Set("X-Internal-FollowUp", "true")
	loopbackReq.RemoteAddr = "127.0.0.1:1234"
	loopbackRec := httptest.NewRecorder()
	handler.ServeHTTP(loopbackRec, loopbackReq)
	if loopbackRec.Code != http.StatusNoContent {
		t.Fatalf("loopback follow-up status = %d, want 204", loopbackRec.Code)
	}
}
