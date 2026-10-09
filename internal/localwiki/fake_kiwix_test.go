package localwiki

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeKiwix serves an OPDS catalog, .meta4 files and two mirrors (m1, m2) plus
// the load-balancer fallback over TLS. Mirror behaviour is set per edition.
// The server listens on 127.0.0.1, so callers that parse its metalinks must
// pass its URL as a trusted host (parseMeta4, readDownload).
type fakeKiwix struct {
	t      *testing.T
	server *httptest.Server

	mu              sync.Mutex
	editions        map[string]*fakeEdition
	catalogStatus   int
	catalogRequests int
	requests        []fakeRequest
}

type fakeRequest struct {
	Path  string
	Range string
}

// Mirror modes: "" serves with Range support, "fail" answers 503,
// "http-redirect" redirects to http://, "range-416" refuses every Range request,
// "range-416-then-fail" refuses Range requests and answers 503 to full ones,
// "ignore-range" answers 200 to Range requests, "html-200" answers every
// request with a short HTML page and status 200, "chunked-html" does the same
// without a Content-Length, "gzip" labels the file as gzip-encoded, "cut" sends
// cutChunk bytes from the requested offset and then drops the connection,
// "slow" sends slowChunk bytes and then waits for unblock() or the client to
// go away.
type fakeEdition struct {
	name         string
	kiwix        string
	variant      Variant
	data         []byte
	sha256       string
	articleCount int
	slowChunk    int
	cutChunk     int

	mu          sync.Mutex
	modes       map[string]string
	release     chan struct{}
	releaseOnce sync.Once
}

func newFakeKiwix(t *testing.T) *fakeKiwix {
	t.Helper()
	f := &fakeKiwix{t: t, editions: map[string]*fakeEdition{}}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeKiwix) addEdition(name string, data []byte) *fakeEdition {
	f.t.Helper()
	parsed, ok := parseEditionName(name)
	if !ok {
		f.t.Fatalf("bad edition name %q", name)
	}
	sum := sha256.Sum256(data)
	e := &fakeEdition{
		name: name, kiwix: parsed.Kiwix, variant: parsed.Variant, data: data, sha256: hex.EncodeToString(sum[:]),
		articleCount: 42, slowChunk: 1024, cutChunk: 64 << 10, modes: map[string]string{}, release: make(chan struct{}),
	}
	f.mu.Lock()
	f.editions[name] = e
	f.mu.Unlock()
	return e
}

func (f *fakeKiwix) edition(name string) *fakeEdition {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.editions[name]
}

func (f *fakeKiwix) setCatalogStatus(status int) {
	f.mu.Lock()
	f.catalogStatus = status
	f.mu.Unlock()
}

func (f *fakeKiwix) catalogCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.catalogRequests
}

func (f *fakeKiwix) requestLog() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeRequest(nil), f.requests...)
}

func (f *fakeKiwix) mirrorURLs(e *fakeEdition) []string {
	return []string{
		f.server.URL + "/m1/" + e.name + ".zim",
		f.server.URL + "/m2/" + e.name + ".zim",
	}
}

func (e *fakeEdition) setMode(mirror, mode string) {
	e.mu.Lock()
	e.modes[mirror] = mode
	e.mu.Unlock()
}

func (e *fakeEdition) mode(mirror string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.modes[mirror]
}

func (e *fakeEdition) unblock() {
	e.releaseOnce.Do(func() { close(e.release) })
}

func (f *fakeKiwix) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, fakeRequest{Path: r.URL.Path, Range: r.Header.Get("Range")})
	f.mu.Unlock()
	switch {
	case r.URL.Path == "/catalog/v2/entries":
		f.serveCatalog(w, r)
	case strings.HasSuffix(r.URL.Path, ".zim.meta4"):
		f.serveMeta4(w, r)
	case strings.HasSuffix(r.URL.Path, ".zim"):
		f.serveZIM(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeKiwix) serveCatalog(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.catalogRequests++
	status := f.catalogStatus
	editions := make([]*fakeEdition, 0, len(f.editions))
	for _, e := range f.editions {
		editions = append(editions, e)
	}
	f.mu.Unlock()
	if status != 0 {
		http.Error(w, "catalog unavailable", status)
		return
	}
	name := r.URL.Query().Get("name")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><feed xmlns="http://www.w3.org/2005/Atom">`)
	for _, e := range editions {
		if "wikipedia_"+e.kiwix+"_all" != name {
			continue
		}
		fmt.Fprintf(&b, `<entry><name>wikipedia_%s_all</name><flavour>%s</flavour><articleCount>%d</articleCount>`+
			`<link rel="%s" type="application/x-zim" href="%s/zim/wikipedia/%s.zim.meta4" length="%d"/></entry>`,
			e.kiwix, e.variant, e.articleCount, acquisitionRel, f.server.URL, e.name, len(e.data))
	}
	b.WriteString(`</feed>`)
	w.Header().Set("Content-Type", "application/atom+xml")
	_, _ = io.WriteString(w, b.String())
}

func (f *fakeKiwix) serveMeta4(w http.ResponseWriter, r *http.Request) {
	e := f.edition(strings.TrimSuffix(path.Base(r.URL.Path), ".zim.meta4"))
	if e == nil {
		http.NotFound(w, r)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?><metalink xmlns="urn:ietf:params:xml:ns:metalink">`+
		`<file name="%s.zim"><size>%d</size><hash type="sha-256">%s</hash>`, e.name, len(e.data), e.sha256)
	for i, mirror := range f.mirrorURLs(e) {
		fmt.Fprintf(&b, `<url priority="%d">%s</url>`, i+1, mirror)
	}
	b.WriteString(`</file></metalink>`)
	w.Header().Set("Content-Type", "application/metalink4+xml")
	_, _ = io.WriteString(w, b.String())
}

func (f *fakeKiwix) serveZIM(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	file := parts[len(parts)-1]
	mirror := "fallback"
	if len(parts) == 2 {
		mirror = parts[0]
	}
	e := f.edition(strings.TrimSuffix(file, ".zim"))
	if e == nil {
		http.NotFound(w, r)
		return
	}
	switch e.mode(mirror) {
	case "fail":
		http.Error(w, "mirror down", http.StatusServiceUnavailable)
	case "http-redirect":
		http.Redirect(w, r, "http://127.0.0.1:9/"+file, http.StatusFound)
	case "range-416":
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes */"+strconv.Itoa(len(e.data)))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		http.ServeContent(w, r, file, time.Time{}, bytes.NewReader(e.data))
	case "range-416-then-fail":
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes */"+strconv.Itoa(len(e.data)))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		http.Error(w, "mirror down", http.StatusServiceUnavailable)
	case "chunked-html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "<html>maintenance</html>\n")
	case "gzip":
		w.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(w, r, file, time.Time{}, bytes.NewReader(e.data))
	case "ignore-range":
		r.Header.Del("Range")
		http.ServeContent(w, r, file, time.Time{}, bytes.NewReader(e.data))
	case "html-200":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html><body>Mirror under maintenance</body></html>")
	case "cut":
		e.serveCut(w, r)
	case "slow":
		chunk := e.slowChunk
		if chunk <= 0 || chunk > len(e.data) {
			chunk = len(e.data) / 4
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(e.data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(e.data[:chunk])
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-e.release:
			_, _ = w.Write(e.data[chunk:])
		case <-r.Context().Done():
		}
	default:
		http.ServeContent(w, r, file, time.Time{}, bytes.NewReader(e.data))
	}
}

// serveCut answers "bytes=N-" (or the whole file) correctly but sends only
// cutChunk bytes before it drops the connection, like a flaky mirror.
func (e *fakeEdition) serveCut(w http.ResponseWriter, r *http.Request) {
	start := 0
	if raw, ok := strings.CutPrefix(r.Header.Get("Range"), "bytes="); ok {
		value, err := strconv.Atoi(strings.TrimSuffix(raw, "-"))
		if err != nil || value < 0 || value >= len(e.data) {
			http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start = value
	}
	end := min(start+e.cutChunk, len(e.data))
	w.Header().Set("Content-Length", strconv.Itoa(len(e.data)-start))
	if start > 0 {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(e.data)-1, len(e.data)))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = w.Write(e.data[start:end])
	if end == len(e.data) {
		return
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	panic(http.ErrAbortHandler)
}

// testPayload returns deterministic bytes for download tests that do not open a ZIM.
func testPayload(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte((i*31 + i/7) % 251)
	}
	return data
}
