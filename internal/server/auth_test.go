package server

import (
	"aurago/internal/config"
	"bytes"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
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

func TestAuthMiddlewareRedirectsToSetupWhenPasswordMissing(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.LLM.APIKey = "configured"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(s, next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/setup" {
		t.Fatalf("redirect location = %q, want /setup", loc)
	}
}

func TestHandleAuthLoginPageRedirectsToSetupWhenPasswordMissing(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.LLM.APIKey = "configured"

	uiFS := fstest.MapFS{
		"login.html": &fstest.MapFile{Data: []byte("ignored")},
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rec := httptest.NewRecorder()
	handleAuthLoginPage(s, fs.FS(uiFS)).ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/setup" {
		t.Fatalf("redirect location = %q, want /setup", loc)
	}
}

func TestHandleAuthLoginReturnsSetupRedirectWhenPasswordMissing(t *testing.T) {
	t.Parallel()

	loginRecords = make(map[string]*loginRecord)
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.LLM.APIKey = "configured"
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	body, _ := json.Marshal(map[string]string{"password": "irrelevant"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleAuthLogin(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if payload["redirect"] != "/setup" {
		t.Fatalf("redirect = %v, want /setup", payload["redirect"])
	}
	if payload["setup_required"] != true {
		t.Fatalf("setup_required = %v, want true", payload["setup_required"])
	}
}

func TestSetupWizardRedirectTargetDefaultsToSetupWhenPasswordBootstrapState(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.LLM.APIKey = "configured"

	target := setupWizardRedirectTarget(&Server{Cfg: cfg})
	if target != "/setup" {
		t.Fatalf("target = %q, want /setup", target)
	}
}

func TestAuthMiddlewareAllowsLoginAssetsWithoutSession(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(s, next)

	paths := []string{
		"/fonts/fonts.css",
		"/css/tokens.css",
		"/css/enhancements.css",
		"/js/vendor/three.min.js",
		"/js/login/main.js",
		"/site.webmanifest",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("path %q status = %d, want 204", path, rec.Code)
		}
	}
}

func TestAuthMiddlewareAllowsSetupAssetsWhenPasswordMissing(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.LLM.APIKey = "configured"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(s, next)

	paths := []string{
		"/setup",
		"/fonts/fonts.css",
		"/css/tailwind-compat.css",
		"/css/tokens.css",
		"/css/setup.css",
		"/css/enhancements.css",
		"/shared.js?v=7",
		"/js/setup/main.js",
		"/site.webmanifest",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("path %q status = %d, want 204", path, rec.Code)
		}
	}
}

func TestAuthMiddlewareLoopbackFollowUpBypassRequiresLoopbackRemoteAddr(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	s.internalToken = "test-secret-token"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(s, next)

	externalReq := httptest.NewRequest(http.MethodPost, "/api/invasion/nests/n1/hatch", nil)
	externalReq.Header.Set("X-Internal-FollowUp", "true")
	externalReq.Header.Set("X-Internal-Token", "test-secret-token")
	externalReq.RemoteAddr = "10.0.0.42:1234"
	externalRec := httptest.NewRecorder()
	handler.ServeHTTP(externalRec, externalReq)
	if externalRec.Code != http.StatusUnauthorized {
		t.Fatalf("external follow-up status = %d, want 401", externalRec.Code)
	}

	loopbackReq := httptest.NewRequest(http.MethodPost, "/api/invasion/nests/n1/hatch", nil)
	loopbackReq.Header.Set("X-Internal-FollowUp", "true")
	loopbackReq.Header.Set("X-Internal-Token", "test-secret-token")
	loopbackReq.RemoteAddr = "127.0.0.1:1234"
	loopbackRec := httptest.NewRecorder()
	handler.ServeHTTP(loopbackRec, loopbackReq)
	if loopbackRec.Code != http.StatusNoContent {
		t.Fatalf("loopback follow-up with valid token status = %d, want 204", loopbackRec.Code)
	}

	wrongTokenReq := httptest.NewRequest(http.MethodPost, "/api/invasion/nests/n1/hatch", nil)
	wrongTokenReq.Header.Set("X-Internal-FollowUp", "true")
	wrongTokenReq.Header.Set("X-Internal-Token", "wrong-token")
	wrongTokenReq.RemoteAddr = "127.0.0.1:1234"
	wrongTokenRec := httptest.NewRecorder()
	handler.ServeHTTP(wrongTokenRec, wrongTokenReq)
	if wrongTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("loopback follow-up with wrong token status = %d, want 401", wrongTokenRec.Code)
	}
}

func TestAuthMiddlewareToolBridgeBypassRequiresLoopbackInternalToken(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	s.internalToken = "test-secret-token"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware(s, next)

	externalReq := httptest.NewRequest(http.MethodPost, "/api/internal/tool-bridge/proxmox", nil)
	externalReq.Header.Set("X-Internal-Token", "test-secret-token")
	externalReq.RemoteAddr = "10.0.0.42:1234"
	externalRec := httptest.NewRecorder()
	handler.ServeHTTP(externalRec, externalReq)
	if externalRec.Code != http.StatusUnauthorized {
		t.Fatalf("external tool-bridge status = %d, want 401", externalRec.Code)
	}

	loopbackReq := httptest.NewRequest(http.MethodPost, "/api/internal/tool-bridge/proxmox", nil)
	loopbackReq.Header.Set("X-Internal-Token", "test-secret-token")
	loopbackReq.RemoteAddr = "127.0.0.1:1234"
	loopbackRec := httptest.NewRecorder()
	handler.ServeHTTP(loopbackRec, loopbackReq)
	if loopbackRec.Code != http.StatusNoContent {
		t.Fatalf("loopback tool-bridge with valid token status = %d, want 204", loopbackRec.Code)
	}

	wrongTokenReq := httptest.NewRequest(http.MethodPost, "/api/internal/tool-bridge/proxmox", nil)
	wrongTokenReq.Header.Set("X-Internal-Token", "wrong-token")
	wrongTokenReq.RemoteAddr = "127.0.0.1:1234"
	wrongTokenRec := httptest.NewRecorder()
	handler.ServeHTTP(wrongTokenRec, wrongTokenReq)
	if wrongTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("loopback tool-bridge with wrong token status = %d, want 401", wrongTokenRec.Code)
	}
}

// TestCheckCSRFOriginValidOrigin verifies that a matching Origin header is accepted.
func TestCheckCSRFOriginValidOrigin(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Host = "example.com"
	if !checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return true for matching Origin")
	}
}

// TestCheckCSRFOriginMismatchOrigin verifies that a mismatched Origin header is rejected.
func TestCheckCSRFOriginMismatchOrigin(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Host = "example.com"
	if checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return false for mismatched Origin")
	}
}

// TestCheckCSRFOriginNoHeaders verifies that requests without Origin or Referer are rejected.
func TestCheckCSRFOriginNoHeaders(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Host = "example.com"
	if checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return false when both Origin and Referer are missing")
	}
}

// TestCheckCSRFOriginValidReferer verifies that a valid matching Referer is accepted.
func TestCheckCSRFOriginValidReferer(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Referer", "https://example.com/page")
	req.Host = "example.com"
	if !checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return true for matching Referer")
	}
}

// TestCheckCSRFOriginMismatchReferer verifies that a mismatched Referer host is rejected.
func TestCheckCSRFOriginMismatchReferer(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Referer", "https://evil.com/page")
	req.Host = "example.com"
	if checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return false for mismatched Referer host")
	}
}

// TestCheckCSRFOriginHTTPSDowngrade verifies that HTTPS requests with HTTP Referer are rejected.
func TestCheckCSRFOriginHTTPSDowngrade(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Referer", "http://example.com/page") // HTTP Referer on HTTPS request
	req.Host = "example.com"
	// Simulate TLS being present (r.TLS != nil) by constructing the URL with https scheme.
	req.URL, _ = url.Parse("https://example.com/api/data")
	if checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return false for HTTPS request with HTTP Referer")
	}
}

// TestCheckCSRFOriginMalformedReferer verifies that a malformed Referer is rejected.
func TestCheckCSRFOriginMalformedReferer(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Referer", "not-a-valid-url")
	req.Host = "example.com"
	if checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return false for malformed Referer")
	}
}

// TestCheckCSRFOriginWithXForwardedHost verifies that X-Forwarded-Host is respected.
func TestCheckCSRFOriginWithXForwardedHost(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/data", nil)
	req.Header.Set("Referer", "https://example.com/page")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Host = "localhost" // actual host differs, but X-Forwarded-Host takes precedence
	if !checkCSRFOrigin(req) {
		t.Error("expected checkCSRFOrigin to return true when X-Forwarded-Host matches Referer host")
	}
}
