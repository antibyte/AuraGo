package server

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
)

// gzipPool reuses gzip writers to reduce allocation pressure on static asset serving.
var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

// gzipMiddleware compresses eligible UI static and HTML responses when the client
// advertises Accept-Encoding: gzip. Streaming endpoints, WebSocket upgrades, Range
// requests, and already-compressed binary types are skipped.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !gzipRequestEligible(r) {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w, req: r}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

func gzipRequestEligible(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("Range") != "" {
		return false
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	// Never wrap long-lived streams or desktop realtime sockets.
	p := r.URL.Path
	if p == "/events" || strings.HasPrefix(p, "/events/") {
		return false
	}
	if strings.HasSuffix(p, "/ws") || strings.Contains(p, "/ws/") {
		return false
	}
	if strings.HasPrefix(p, "/files/") {
		// User media / workspace downloads: may be binary and Range-heavy.
		return false
	}
	return true
}

func gzipPathEligible(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".js", ".css", ".json", ".svg", ".html", ".htm", ".txt", ".xml", ".map", ".mjs":
		return true
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".otf",
		".wasm", ".onnx", ".gz", ".br", ".mp3", ".mp4", ".webm", ".pdf", ".zip":
		return false
	}
	// HTML pages without extension (/, /desktop, /config, …)
	if ext == "" {
		return true
	}
	return false
}

func gzipContentTypeEligible(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return false
	}
	// Strip parameters (; charset=utf-8)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch {
	case strings.HasPrefix(ct, "text/"):
		return true
	case ct == "application/javascript", ct == "text/javascript", ct == "application/x-javascript":
		return true
	case ct == "application/json", ct == "application/manifest+json":
		return true
	case ct == "image/svg+xml", ct == "application/xml":
		return true
	case strings.HasSuffix(ct, "+json"), strings.HasSuffix(ct, "+xml"):
		return true
	default:
		return false
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	req           *http.Request
	gz            *gzip.Writer
	wroteHeader   bool
	status        int
	skip          bool
	headerWritten bool
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	g.status = status
	if g.status == 0 {
		g.status = http.StatusOK
	}
	// The downstream WriteHeader is deferred to the first Write (or close) so
	// handlers without an explicit Content-Type still benefit from net/http's
	// content sniffing; committing headers here would drop the Content-Type.
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		g.WriteHeader(http.StatusOK)
	}
	if !g.headerWritten {
		g.beginResponse(b)
	}
	if g.skip || g.gz == nil {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

// beginResponse finalizes the response headers on the first body write: it
// sniffs the Content-Type when the handler did not set one, decides between
// compression and passthrough, and only then commits the status downstream.
func (g *gzipResponseWriter) beginResponse(first []byte) {
	g.headerWritten = true
	if g.Header().Get("Content-Type") == "" && len(first) > 0 {
		g.Header().Set("Content-Type", http.DetectContentType(first))
	}
	if g.shouldSkipCompression() {
		g.skip = true
		g.ResponseWriter.WriteHeader(g.status)
		return
	}

	// Compression will rewrite length; remove pre-set length from upstream.
	g.Header().Del("Content-Length")
	g.Header().Set("Content-Encoding", "gzip")
	g.Header().Add("Vary", "Accept-Encoding")

	zw := gzipPool.Get().(*gzip.Writer)
	zw.Reset(g.ResponseWriter)
	g.gz = zw
	g.ResponseWriter.WriteHeader(g.status)
}

func (g *gzipResponseWriter) Flush() {
	if !g.headerWritten && g.wroteHeader {
		// A flush before any body byte would commit headers downstream without
		// a Content-Type; fall back to uncompressed passthrough instead.
		g.headerWritten = true
		g.skip = true
		g.ResponseWriter.WriteHeader(g.status)
	}
	if g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipResponseWriter) close() {
	if !g.headerWritten {
		// Header-only responses (no body ever written) forward their status.
		if !g.wroteHeader {
			g.WriteHeader(http.StatusOK)
		}
		g.headerWritten = true
		g.skip = true
		g.ResponseWriter.WriteHeader(g.status)
	}
	if g.gz != nil {
		_ = g.gz.Close()
		gzipPool.Put(g.gz)
		g.gz = nil
	}
}

// Hijack allows WebSocket upgrades that slipped past pre-checks to reach the
// underlying ResponseWriter without a gzip wrapper in the way.
func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if g.gz != nil {
		return nil, nil, fmt.Errorf("gzip: cannot hijack after compression started")
	}
	h, ok := g.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("gzip: underlying ResponseWriter does not support hijacking")
	}
	g.skip = true
	return h.Hijack()
}

func (g *gzipResponseWriter) shouldSkipCompression() bool {
	if g.status < 200 || g.status >= 300 {
		return true
	}
	if g.Header().Get("Content-Encoding") != "" {
		// Already encoded by upstream.
		return true
	}
	if ct := g.Header().Get("Content-Type"); strings.HasPrefix(strings.ToLower(ct), "text/event-stream") {
		return true
	}
	// Prefer explicit Content-Type from handler; fall back to path extension.
	if ct := g.Header().Get("Content-Type"); ct != "" {
		if !gzipContentTypeEligible(ct) {
			return true
		}
		return false
	}
	if g.req != nil && !gzipPathEligible(g.req.URL.Path) {
		return true
	}
	return false
}
