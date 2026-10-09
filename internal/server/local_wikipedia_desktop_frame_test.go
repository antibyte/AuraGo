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

// localWikiFrameTestCSP stands in for the content handler's own policy: the
// handler (not the middleware) is what grants `frame-ancestors 'self'`.
const localWikiFrameTestCSP = "sandbox allow-same-origin; default-src 'none'; frame-ancestors 'self'"

func TestSecurityHeadersAllowLocalWikipediaContentToBeFramedSameOrigin(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", localWikiFrameTestCSP)
		w.WriteHeader(http.StatusOK)
	}), true, false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://example.test/api/desktop/local-wikipedia/content/Berlin", nil))
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want empty for same-origin article frames", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != localWikiFrameTestCSP {
		t.Fatalf("Content-Security-Policy = %q, want the handler's policy", got)
	}
	// Only the exact content prefix is exempt: sibling routes, look-alike
	// prefixes and the bare route without a path keep DENY.
	for _, path := range []string{
		"/api/desktop/local-wikipedia/status",
		"/api/desktop/local-wikipedia/search",
		"/api/desktop/local-wikipedia/contentx",
		"/api/desktop/local-wikipedia/content",
		"/api/desktop/local-wikipedia",
		"/api/desktop/local-wikipedia/",
		"/api/desktop/local-wikipediax/content/Berlin",
		"/api/local-wikipedia/content/Berlin",
		"/x/api/desktop/local-wikipedia/content/Berlin",
	} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://example.test"+path, nil))
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("%s X-Frame-Options = %q, want DENY", path, got)
		}
	}
}

// The exemption must not widen framing by itself: a response under the content
// prefix that the handler did not give its own policy (an early 401/404 from
// another layer) still carries the global CSP with frame-ancestors 'none'.
func TestSecurityHeadersLocalWikipediaContentWithoutHandlerPolicyStaysUnframable(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}), false, false)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/desktop/local-wikipedia/content/Berlin", nil))
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("Content-Security-Policy = %q, want frame-ancestors 'none' until a handler overrides it", csp)
	}
}

// The other framing exemptions and the DENY default must be unchanged by the
// Local Wikipedia exception.
func TestSecurityHeadersLocalWikipediaExceptionLeavesOtherFramingRoutesAlone(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), false, false)
	for path, want := range map[string]string{
		"/":                                   "DENY",
		"/desktop":                            "DENY",
		"/api/desktop/files":                  "DENY",
		"/api/desktop/local-wikipedia/random": "DENY",
		"/files/desktop/app/index.html":       "",
		"/api/go2rtc/viewer/cam1":             "",
		"/api/game-maker/preview/p1/":         "",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Frame-Options"); got != want {
			t.Fatalf("%s X-Frame-Options = %q, want %q", path, got, want)
		}
	}
}

// Through the real authMiddleware: an authenticated session cookie reaches the
// content route (same-site iframe GET), a handler may replace the cache headers
// authMiddleware sets for sessions, and the exemption does not leak to the
// sibling API routes.
func TestLocalWikipediaContentFramingSurvivesTheMiddlewareChain(t *testing.T) {
	const secret = "local-wiki-frame-test-secret"
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = secret

	mux := http.NewServeMux()
	mux.HandleFunc("/api/desktop/local-wikipedia/content/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", localWikiFrameTestCSP)
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
		if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "" || w.Header().Get("Content-Security-Policy") != localWikiFrameTestCSP ||
			w.Header().Get("Cache-Control") != "private, max-age=86400" || w.Header().Get("Pragma") != "" {
			t.Fatalf("%s = %d xfo=%q csp=%q cache=%q pragma=%q", path, w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"), w.Header().Get("Cache-Control"), w.Header().Get("Pragma"))
		}
	}
	w := request("/api/desktop/local-wikipedia/status", true)
	if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("status = %d xfo=%q cache=%q", w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Cache-Control"))
	}
	// Unauthenticated requests are still refused before the handler runs and
	// keep the unframable global policy.
	w = request("/api/desktop/local-wikipedia/content/Berlin", false)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatalf("anonymous content = %d csp=%q, want 401 with frame-ancestors 'none'", w.Code, w.Header().Get("Content-Security-Policy"))
	}
}

// The real Desktop handler behind the real middleware chain: article and asset
// responses are framable with the content CSP and their own cache policy,
// framable error pages too, while the JSON routes stay DENY and no-store.
func TestLocalWikiContentSurvivesTheMiddlewareChain(t *testing.T) {
	s, _ := newLocalWikiDesktopTestServer(t)
	s.Logger = slog.Default()
	backend := newFakeLocalWikiBackend()
	mux := http.NewServeMux()
	mux.HandleFunc(localWikiDesktopPrefix, func(w http.ResponseWriter, r *http.Request) {
		s.serveLocalWikipediaDesktop(w, r, backend)
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
		if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "" || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP ||
			w.Header().Get("Cache-Control") != "private, max-age=86400" || w.Header().Get("Pragma") != "" {
			t.Fatalf("%s = %d xfo=%q csp=%q cache=%q pragma=%q", path, w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"), w.Header().Get("Cache-Control"), w.Header().Get("Pragma"))
		}
	}
	w := request("/api/desktop/local-wikipedia/content/Gibt_es_nicht")
	if w.Code != http.StatusNotFound || w.Header().Get("X-Frame-Options") != "" || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP {
		t.Fatalf("missing article = %d xfo=%q csp=%q", w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Content-Security-Policy"))
	}
	w = request("/api/desktop/local-wikipedia/status")
	if w.Code != http.StatusOK || w.Header().Get("X-Frame-Options") != "DENY" || w.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'") {
		t.Fatalf("status = %d xfo=%q cache=%q csp=%q", w.Code, w.Header().Get("X-Frame-Options"), w.Header().Get("Cache-Control"), w.Header().Get("Content-Security-Policy"))
	}
}
