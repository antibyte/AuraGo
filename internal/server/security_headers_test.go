package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAllowYouTubeEmbeds(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	for _, marker := range []string{
		"default-src 'self'",
		"connect-src 'self' ws: wss: https://de1.api.radio-browser.info",
		"img-src 'self' data: blob: https:",
		"media-src 'self' data: blob: http: https:",
		"object-src 'none'",
		"form-action 'self'",
		"frame-src 'self' https://www.youtube-nocookie.com https://www.youtube.com",
		"frame-ancestors 'none'",
		"base-uri 'self'",
	} {
		if !strings.Contains(csp, marker) {
			t.Fatalf("Content-Security-Policy missing %q: %s", marker, csp)
		}
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
}

func TestSecurityHeadersSetStrictTransportSecurityForHTTPS(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), true, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=31536000") {
		t.Fatalf("Strict-Transport-Security = %q, want max-age=31536000", got)
	}
}

func TestSecurityHeadersAllowDesktopWorkspaceFilesToBeFramed(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", desktopWorkspaceCSP)
		w.WriteHeader(http.StatusNoContent)
	}), true, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.test/files/desktop/Apps/notes/widget.html", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want empty for desktop iframe files", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != desktopWorkspaceCSP {
		t.Fatalf("Content-Security-Policy = %q, want desktop CSP", got)
	}
}

func TestDesktopWorkspaceCSPAllowsWeatherWidgets(t *testing.T) {
	if !strings.Contains(desktopWorkspaceCSP, "connect-src 'self' https://api.open-meteo.com") {
		t.Fatalf("desktop workspace CSP does not allow Open-Meteo weather widgets: %s", desktopWorkspaceCSP)
	}
	for _, field := range strings.Fields(strings.ReplaceAll(desktopWorkspaceCSP, ";", " ")) {
		if field == "https:" {
			t.Fatalf("desktop workspace CSP must not allow arbitrary HTTPS fetches: %s", desktopWorkspaceCSP)
		}
	}
}
