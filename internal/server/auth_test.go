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
