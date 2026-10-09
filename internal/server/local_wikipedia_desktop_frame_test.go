package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// The middleware never exempts Local Wikipedia paths: it decides on the
// decoded path while the mux routes on the escaped one, so the framing
// exception lives in the content handler. Every path keeps DENY and the
// unframable global policy until that handler answers, and none of them gets
// the public static-asset cache, whatever its suffix.
func TestSecurityHeadersKeepLocalWikipediaPathsUnframable(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}), true, false)
	for _, path := range []string{
		"/api/desktop/local-wikipedia/content/Berlin",
		"/api/desktop/local-wikipedia/content/_assets_/logo.png",
		"/api/desktop/local-wikipedia/content/style.css",
		"/api/desktop/local-wikipedia/content/icon.svg",
		"/api/desktop/local-wikipedia/content/app.js",
		"/api/desktop/local-wikipedia%2Fcontent/Berlin",
		"/api/desktop/local-wikipedia%2Fcontent/logo.png",
		"/api/desktop/local-wikipedia/status",
		"/api/desktop/local-wikipedia/search.png",
		"/api/desktop/local-wikipedia/content",
		"/api/desktop/local-wikipediax/content/Berlin",
		"/api/local-wikipedia/content/Berlin",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://example.test"+path, nil))
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("%s X-Frame-Options = %q, want DENY", path, got)
		}
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("%s Content-Security-Policy = %q, want frame-ancestors 'none'", path, csp)
		}
		if strings.HasPrefix(path, "/api/desktop/local-wikipedia") && !strings.HasPrefix(path, "/api/desktop/local-wikipediax") {
			if cache := rec.Header().Get("Cache-Control"); strings.Contains(cache, "public") || !strings.Contains(cache, "no-store") {
				t.Fatalf("%s Cache-Control = %q, want no-store", path, cache)
			}
		}
	}
}

// The other framing exemptions, the DENY default and the static-asset cache of
// other routes are unchanged.
func TestSecurityHeadersLocalWikipediaExceptionLeavesOtherFramingRoutesAlone(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)
	for path, want := range map[string]string{
		"/":                                   "DENY",
		"/desktop":                            "DENY",
		"/api/desktop/files":                  "DENY",
		"/api/desktop/local-wikipedia/random": "DENY",
		"/api/desktop/local-wikipedia/content/Berlin": "DENY",
		"/files/desktop/app/index.html":               "",
		"/api/go2rtc/viewer/cam1":                     "",
		"/api/game-maker/preview/p1/":                 "",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Frame-Options"); got != want {
			t.Fatalf("%s X-Frame-Options = %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{"/img/logo.png", "/api/desktop/store/logos/app.png", "/api/go2rtc/viewer/cam1/video-rtc.js"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
			t.Fatalf("%s Cache-Control = %q, want the static asset cache", path, got)
		}
	}
}

// Through the real authMiddleware: an authenticated session cookie reaches the
// content route (same-site iframe GET), the content headers replace the
// framing and cache headers set before the handler, and the middleware's
// hardware policies stay next to the content policy.
func TestLocalWikipediaContentFramingSurvivesTheMiddlewareChain(t *testing.T) {
	const secret = "local-wiki-frame-test-secret"
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = secret

	mux := http.NewServeMux()
	mux.HandleFunc("/api/desktop/local-wikipedia/content/", func(w http.ResponseWriter, r *http.Request) {
		setLocalWikiContentHeaders(w.Header())
		w.Header().Set("Cache-Control", "private, max-age=86400")
		w.Header().Del("Pragma")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/desktop/local-wikipedia/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := securityHeadersMiddleware(authMiddleware(s, mux), false, false)
	request := func(path string, withSession bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if withSession {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(secret, time.Now().Add(time.Hour))})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	for _, path := range []string{"/api/desktop/local-wikipedia/content/Berlin", "/api/desktop/local-wikipedia/content/_assets_/logo.png"} {
		w := request(path, true)
		if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP ||
			w.Header().Get("Cache-Control") != "private, max-age=86400" || w.Header().Get("Pragma") != "" {
			t.Fatalf("%s = %d xfo=%q csp=%q cache=%q pragma=%q", path, w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"), w.Header().Get("Cache-Control"), w.Header().Get("Pragma"))
		}
		policy := strings.Join(w.Header().Values("Permissions-Policy"), ", ")
		for _, want := range []string{"serial=()", "midi=()", "attribution-reporting=()", "browsing-topics=()"} {
			if !strings.Contains(policy, want) {
				t.Fatalf("%s Permissions-Policy = %q, want %s", path, policy, want)
			}
		}
	}
	w := request("/api/desktop/local-wikipedia/status", true)
	if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("status = %d xfo=%q cache=%q", w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Cache-Control"))
	}
	// Unauthenticated requests are refused before the handler runs and keep
	// the unframable global policy, without a public cache for asset suffixes.
	for _, path := range []string{"/api/desktop/local-wikipedia/content/Berlin", "/api/desktop/local-wikipedia/content/_assets_/logo.png"} {
		w = request(path, false)
		if w.Code != http.StatusUnauthorized || w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") ||
			strings.Contains(w.Header().Get("Cache-Control"), "public") {
			t.Fatalf("anonymous %s = %d xfo=%q csp=%q cache=%q", path, w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"), w.Header().Get("Cache-Control"))
		}
	}
}

// The real Desktop handler behind the real middleware chain and mux: article
// and asset responses are framable with the content CSP and their own cache
// policy, framable error pages too, while the JSON routes, requests the mux
// hands to another handler (an escaped "%2F") and the mux's own clean-path
// redirects stay DENY.
func TestLocalWikiContentSurvivesTheMiddlewareChain(t *testing.T) {
	s, _ := newLocalWikiDesktopTestServer(t)
	s.Logger = slog.Default()
	backend := newFakeLocalWikiBackend()
	mux := http.NewServeMux()
	mux.HandleFunc(localWikiDesktopPrefix, func(w http.ResponseWriter, r *http.Request) {
		s.serveLocalWikipediaDesktop(w, r, backend)
	})
	mux.HandleFunc("/api/desktop/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Handler", "desktop")
		w.WriteHeader(http.StatusNotFound)
	})
	handler := securityHeadersMiddleware(authMiddleware(s, mux), false, false)
	request := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/desktop/local-wikipedia/content/Berlin", "/api/desktop/local-wikipedia/content/_assets_/logo.png"} {
		w := request(path)
		if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP ||
			w.Header().Get("Cache-Control") != "private, max-age=86400" || w.Header().Get("Pragma") != "" {
			t.Fatalf("%s = %d xfo=%q csp=%q cache=%q pragma=%q", path, w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"), w.Header().Get("Cache-Control"), w.Header().Get("Pragma"))
		}
	}
	w := request("/api/desktop/local-wikipedia/content/Gibt_es_nicht")
	if w.Code != http.StatusNotFound || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP {
		t.Fatalf("missing article = %d xfo=%q csp=%q", w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"))
	}
	w = request("/api/desktop/local-wikipedia/status")
	if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "DENY" || w.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'") {
		t.Fatalf("status = %d xfo=%q cache=%q csp=%q", w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Cache-Control"), w.Header().Get("Content-Security-Policy"))
	}
	// The mux routes on the escaped path: "local-wikipedia%2Fcontent" is one
	// segment, so another handler answers, and it must not be framable.
	w = request("/api/desktop/local-wikipedia%2Fcontent/Berlin")
	if w.Header().Get("X-Test-Handler") != "desktop" || w.Header().Get("X-Frame-Options") != "DENY" ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("escaped separator = %d handler=%q xfo=%q csp=%q", w.Code, w.Header().Get("X-Test-Handler"), w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"))
	}
	// The mux redirects unclean paths itself, before any handler runs.
	for _, path := range []string{"/api/desktop/local-wikipedia/content/x/../../status", "/api/desktop/local-wikipedia/content/a//b"} {
		w = request(path)
		if w.Code < 300 || w.Code > 399 || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("%s = %d xfo=%q location=%q", path, w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Location"))
		}
	}
}
