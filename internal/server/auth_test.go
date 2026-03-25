package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClearSessionCookieIncludesSecureOnHTTPS(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com/auth/logout", nil)
	rec := httptest.NewRecorder()

	ClearSessionCookie(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != sessionCookieName {
		t.Fatalf("expected cookie %q, got %q", sessionCookieName, c.Name)
	}
	if !c.Secure {
		t.Fatalf("expected secure cookie on HTTPS logout")
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

func TestClearSessionCookieIncludesProxySecureAttribute(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth/logout", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	ClearSessionCookie(rec, req)

	header := rec.Header().Get("Set-Cookie")
	if !strings.Contains(header, "Secure") {
		t.Fatalf("expected secure flag in Set-Cookie header, got %q", header)
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
