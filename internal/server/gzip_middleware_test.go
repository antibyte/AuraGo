package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipMiddlewareCompressesJavaScript(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("console.log('hello desktop');\n", 200)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})

	req := httptest.NewRequest(http.MethodGet, "/js/desktop/bundles/main.bundle.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("Vary missing Accept-Encoding: %q", rec.Header().Get("Vary"))
	}
	raw, err := io.ReadAll(mustGzipReader(t, rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if string(raw) != body {
		t.Fatalf("decompressed body mismatch (len %d vs %d)", len(raw), len(body))
	}
	if rec.Body.Len() >= len(body) {
		t.Fatalf("expected compressed body smaller than %d, got %d", len(body), rec.Body.Len())
	}
}

func TestGzipMiddlewareSkipsWithoutAcceptEncoding(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("x", 5000)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte(body))
	})
	req := httptest.NewRequest(http.MethodGet, "/css/desktop-shell.bundle.css", nil)
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("unexpected Content-Encoding %q", rec.Header().Get("Content-Encoding"))
	}
	if rec.Body.String() != body {
		t.Fatal("body changed without gzip")
	}
}

func TestGzipMiddlewareSkipsPNG(t *testing.T) {
	t.Parallel()

	// Minimal fake binary payload
	body := bytes.Repeat([]byte{0x89, 0x50, 0x4e, 0x47}, 500)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	})
	req := httptest.NewRequest(http.MethodGet, "/img/icon.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("PNG must not be gzipped, got %q", rec.Header().Get("Content-Encoding"))
	}
}

func TestGzipMiddlewareSkipsWebSocketUpgrade(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusSwitchingProtocols)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/ws", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("websocket upgrade must not be gzipped")
	}
	if rec.Code != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestGzipMiddlewareSkipsEventsPath(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: hi\n\n"))
	})
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("SSE /events must not be gzipped")
	}
}

func TestGzipMiddlewareSkipsRangeRequests(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("abcdefghij", 1000)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte(body))
	})
	req := httptest.NewRequest(http.MethodGet, "/js/foo.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-99")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("Range requests must not be gzipped")
	}
}

func TestGzipMiddlewarePreservesSniffedContentType(t *testing.T) {
	t.Parallel()

	// HTML template handlers (/, /desktop, /config, …) do not set an explicit
	// Content-Type; net/http sniffing must survive gzip compression so that
	// browsers honoring X-Content-Type-Options: nosniff still render the page.
	body := "<!DOCTYPE html>\n<html><head><title>desktop</title></head><body>" + strings.Repeat("x", 600) + "</body></html>"
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	req := httptest.NewRequest(http.MethodGet, "/desktop", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("sniffed Content-Type lost under gzip, got %q", ct)
	}
	raw, err := io.ReadAll(mustGzipReader(t, rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if string(raw) != body {
		t.Fatal("decompressed body mismatch")
	}
}

func TestGzipMiddlewareHeaderOnlyResponseKeepsStatus(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	gzipMiddleware(inner).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Header().Get("Content-Encoding") != "" {
		t.Fatal("header-only response must not be gzipped")
	}
}

func mustGzipReader(t *testing.T, b []byte) io.Reader {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}
