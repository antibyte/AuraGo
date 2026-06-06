package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAllowEmbedsForYouTubeAndDesktopStoreApps(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	for _, marker := range []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com https://unpkg.com",
		"connect-src 'self' blob: ws: wss: https://api.open-meteo.com https://geocoding-api.open-meteo.com https://de1.api.radio-browser.info",
		"img-src 'self' data: blob: https:",
		"media-src 'self' data: blob: http: https:",
		"worker-src 'self' blob:",
		"object-src 'none'",
		"form-action 'self'",
		"frame-src 'self' http: https:",
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

func TestSecurityHeadersMainCSPDoesNotAllowUnsafeEval(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "'unsafe-eval'") {
		t.Fatalf("Content-Security-Policy must not allow unsafe-eval: %s", csp)
	}
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'") {
		t.Fatalf("Content-Security-Policy lost required script-src baseline: %s", csp)
	}
}

func TestSecurityHeadersAllowFirstPartyDesktopWidgetConnectOrigins(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/desktop", nil)
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	for _, marker := range []string{
		"https://api.open-meteo.com",
		"https://geocoding-api.open-meteo.com",
		"https://de1.api.radio-browser.info",
	} {
		if !strings.Contains(csp, marker) {
			t.Fatalf("Content-Security-Policy missing first-party desktop connect origin %q: %s", marker, csp)
		}
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
		w.Header().Set("Content-Security-Policy", desktopWidgetWorkspaceCSP)
		w.WriteHeader(http.StatusNoContent)
	}), true, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.test/files/desktop/Apps/notes/widget.html", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want empty for desktop iframe files", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != desktopWidgetWorkspaceCSP {
		t.Fatalf("Content-Security-Policy = %q, want desktop widget CSP", got)
	}
}

func TestDesktopWorkspaceCSPKeepsGeneratedAppsOriginIsolated(t *testing.T) {
	if strings.Contains(desktopAppWorkspaceCSP, "allow-same-origin") {
		t.Fatalf("generated app CSP must keep an opaque sandbox origin: %s", desktopAppWorkspaceCSP)
	}
	if strings.Contains(desktopAppWorkspaceCSP, "connect-src 'self'") {
		t.Fatalf("generated app CSP must not allow direct fetches to AuraGo APIs: %s", desktopAppWorkspaceCSP)
	}
	if strings.Contains(desktopWidgetWorkspaceCSP, "allow-same-origin") {
		t.Fatalf("widget CSP must keep stronger origin isolation: %s", desktopWidgetWorkspaceCSP)
	}
}

func TestDesktopWorkspaceCSPAllowsGeneratedAppCDNs(t *testing.T) {
	for _, marker := range []string{
		"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'",
		"https://cdn.jsdelivr.net",
		"https://cdnjs.cloudflare.com",
		"https://unpkg.com",
		"https://esm.sh",
		"https://cdn.skypack.dev",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' data: https://fonts.gstatic.com",
		"img-src 'self' data: blob: https:",
	} {
		if !strings.Contains(desktopAppWorkspaceCSP, marker) {
			t.Fatalf("generated app CSP missing %q: %s", marker, desktopAppWorkspaceCSP)
		}
	}
	for _, forbidden := range []string{"'unsafe-eval'", "new Function"} {
		if strings.Contains(desktopAppWorkspaceCSP, forbidden) {
			t.Fatalf("generated app CSP must not allow %q: %s", forbidden, desktopAppWorkspaceCSP)
		}
	}
	for _, marker := range []string{"form-action 'self'", "frame-ancestors 'self'"} {
		if !strings.Contains(desktopAppWorkspaceCSP, marker) {
			t.Fatalf("generated app CSP missing %q: %s", marker, desktopAppWorkspaceCSP)
		}
	}
	if strings.Contains(desktopWidgetWorkspaceCSP, "https://cdn.jsdelivr.net") {
		t.Fatalf("widget CSP should remain narrower than generated app CSP: %s", desktopWidgetWorkspaceCSP)
	}
}

func TestDesktopWorkspaceCSPAllowsSameOriginMedia(t *testing.T) {
	const mediaSrc = "media-src 'self' data: blob:"
	if !strings.Contains(desktopAppWorkspaceCSP, mediaSrc) {
		t.Fatalf("generated app CSP must allow same-origin media streams: %s", desktopAppWorkspaceCSP)
	}
	if !strings.Contains(desktopWidgetWorkspaceCSP, mediaSrc) {
		t.Fatalf("widget CSP must allow same-origin media streams: %s", desktopWidgetWorkspaceCSP)
	}
	if strings.Contains(desktopWidgetWorkspaceCSP, "media-src 'self' data: blob: http:") || strings.Contains(desktopWidgetWorkspaceCSP, "media-src 'self' data: blob: https:") {
		t.Fatalf("widget CSP must not allow arbitrary remote media: %s", desktopWidgetWorkspaceCSP)
	}
}

func TestSecurityHeadersDoNotCacheVersionlessDesktopSDK(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/js/desktop/aura-desktop-sdk.js", nil)
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-store for versionless desktop SDK", got)
	}
}

func TestDesktopWorkspaceCSPAllowsWeatherWidgets(t *testing.T) {
	if !strings.Contains(desktopWidgetWorkspaceCSP, "connect-src 'self' https://api.open-meteo.com https://geocoding-api.open-meteo.com") {
		t.Fatalf("desktop workspace CSP does not allow Open-Meteo weather widgets: %s", desktopWidgetWorkspaceCSP)
	}
	for _, field := range strings.Fields(strings.ReplaceAll(desktopWidgetWorkspaceCSP, ";", " ")) {
		if field == "https:" {
			t.Fatalf("desktop workspace CSP must not allow arbitrary HTTPS fetches: %s", desktopWidgetWorkspaceCSP)
		}
	}
}
