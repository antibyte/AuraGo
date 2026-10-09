package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func localWikiContentGet(t *testing.T, s *Server, backend localWikiDesktopBackend, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/content/"+path, "session", headers)
}

func TestLocalWikiContentCSPMatchesTheSpec(t *testing.T) {
	// The spec policy plus connect-src 'none': link pings and beacons from a
	// ZIM document must never reach AuraGo with the session cookie.
	const spec = "sandbox allow-same-origin allow-popups allow-popups-to-escape-sandbox; default-src 'self'; script-src 'none'; connect-src 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; form-action 'none'; frame-ancestors 'self'"
	if localWikipediaContentCSP != spec {
		t.Fatalf("content CSP = %q, want the spec value", localWikipediaContentCSP)
	}
	if strings.Contains(localWikipediaContentCSP, "allow-scripts") {
		t.Fatal("ZIM documents must never run scripts")
	}
}

func TestLocalWikiContentHeaders(t *testing.T) {
	s, _ := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	w := localWikiContentGet(t, s, backend, "Berlin", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Hauptstadt") {
		t.Fatalf("content = %d %s", w.Code, w.Body.String())
	}
	for header, want := range map[string]string{
		"Content-Type":            "text/html; charset=utf-8",
		"Content-Security-Policy": localWikipediaContentCSP,
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "private, max-age=86400",
		"Referrer-Policy":         "no-referrer",
		"ETag":                    `"fixture-Berlin"`,
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	w = localWikiContentGet(t, s, backend, "_assets_/logo.png", nil)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP {
		t.Fatalf("image = %d %q %q", w.Code, w.Header().Get("Content-Type"), w.Header().Get("Content-Security-Policy"))
	}
	w = localWikiContentGet(t, s, backend, "AC/DC", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "AC/DC") {
		t.Fatalf("path with slash = %d %s", w.Code, w.Body.String())
	}
	w = localWikiDesktopDo(t, s, backend, http.MethodHead, "/api/desktop/local-wikipedia/content/Berlin", "session", nil)
	if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD = %d body=%d length=%q", w.Code, w.Body.Len(), w.Header().Get("Content-Length"))
	}
	// Content without a MIME type is never sniffed into something active.
	backend.reader.pages["blob"] = fakeLocalWikiPage{body: "<script>alert(1)</script>"}
	w = localWikiContentGet(t, s, backend, "blob", nil)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/octet-stream" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("untyped blob = %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if backend.acquired == 0 || backend.acquired != backend.released {
		t.Fatalf("acquire/release = %d/%d", backend.acquired, backend.released)
	}
}

func TestLocalWikiContentRangeAndConditionalRequests(t *testing.T) {
	s, _ := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	body := backend.reader.pages["Berlin"].body
	w := localWikiContentGet(t, s, backend, "Berlin", map[string]string{"Range": "bytes=0-8"})
	if w.Code != http.StatusPartialContent || w.Body.String() != body[:9] || w.Header().Get("Content-Range") != "bytes 0-8/"+strconv.Itoa(len(body)) {
		t.Fatalf("range = %d %q %q", w.Code, w.Body.String(), w.Header().Get("Content-Range"))
	}
	if w := localWikiContentGet(t, s, backend, "Berlin", map[string]string{"If-None-Match": `"fixture-Berlin"`}); w.Code != http.StatusNotModified || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP {
		t.Fatalf("If-None-Match = %d csp=%q", w.Code, w.Header().Get("Content-Security-Policy"))
	}
	w = localWikiContentGet(t, s, backend, "Berlin", map[string]string{"Range": "bytes=0-3", "If-Range": `"stale"`})
	if w.Code != http.StatusOK || w.Body.String() != body {
		t.Fatalf("stale If-Range = %d %q", w.Code, w.Body.String())
	}
}

func TestLocalWikiContentRedirectsToTheResolvedPath(t *testing.T) {
	s, _ := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	w := localWikiContentGet(t, s, backend, "Berlin_(Stadt)", nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/api/desktop/local-wikipedia/content/Berlin" || w.Header().Get("Cache-Control") != "no-store" ||
		w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("redirect = %d location=%q cache=%q csp=%q", w.Code, w.Header().Get("Location"), w.Header().Get("Cache-Control"), w.Header().Get("Content-Security-Policy"))
	}
	w = localWikiContentGet(t, s, backend, "Ausbruch", nil)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `content="not_found"`) {
		t.Fatalf("dot-segment redirect = %d %s", w.Code, w.Body.String())
	}
}

func TestLocalWikiContentURLEscapesSegments(t *testing.T) {
	got, ok := localWikiContentURL("Café (Begriffsklärung)/Teil?1")
	if !ok || got != "/api/desktop/local-wikipedia/content/Caf%C3%A9%20%28Begriffskl%C3%A4rung%29/Teil%3F1" {
		t.Fatalf("content URL = %q %v", got, ok)
	}
	for _, path := range []string{"..", "a/../b", "./a", "a/."} {
		if _, ok := localWikiContentURL(path); ok {
			t.Fatalf("%q must not be addressable", path)
		}
	}
}

func TestLocalWikiContentErrorsAreFramableHTML(t *testing.T) {
	s, _ := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	check := func(name string, w *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if w.Code != status || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || w.Header().Get("Content-Security-Policy") != localWikipediaContentCSP ||
			w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("Cache-Control") != "no-store" ||
			!strings.Contains(w.Body.String(), `name="aurago-local-wikipedia-error" content="`+code+`"`) {
			t.Fatalf("%s = %d %q %s", name, w.Code, w.Header().Get("Content-Type"), w.Body.String())
		}
	}
	check("missing", localWikiContentGet(t, s, backend, "Gibt_es_nicht", nil), http.StatusNotFound, "not_found")
	check("empty", localWikiContentGet(t, s, backend, "", nil), http.StatusNotFound, "not_found")
	check("post", localWikiDesktopDo(t, s, backend, http.MethodPost, "/api/desktop/local-wikipedia/content/Berlin", "session", nil), http.StatusMethodNotAllowed, "method_not_allowed")
	s.Cfg.LocalWikipedia.Enabled = false
	check("disabled", localWikiContentGet(t, s, backend, "Berlin", nil), http.StatusServiceUnavailable, "disabled")
	s.Cfg.LocalWikipedia.Enabled = true
	s.Cfg.VirtualDesktop.Enabled = false
	check("desktop off", localWikiContentGet(t, s, backend, "Berlin", nil), http.StatusServiceUnavailable, "desktop_unavailable")
	s.Cfg.VirtualDesktop.Enabled = true
	check("no manager", localWikiDesktopDo(t, s, nil, http.MethodGet, "/api/desktop/local-wikipedia/content/Berlin", "session", nil), http.StatusServiceUnavailable, "unavailable")
	backend.reader = nil
	check("not ready", localWikiContentGet(t, s, backend, "Berlin", nil), http.StatusConflict, "not_ready")
}
