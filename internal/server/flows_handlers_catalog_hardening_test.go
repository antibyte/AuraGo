package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/ui"
)

// c19NodeTypes calls GET /node-types with an optional If-None-Match header.
func c19NodeTypes(t *testing.T, s *Server, token, lang, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/flows/node-types?lang="+lang, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	if ifNoneMatch != "" {
		r.Header.Set("If-None-Match", ifNoneMatch)
	}
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	return w
}

// c19Builds returns how many palette answers the server built.
func c19Builds(s *Server) int {
	s.flowNodeTypes.mu.Lock()
	defer s.flowNodeTypes.mu.Unlock()
	return s.flowNodeTypes.builds
}

var c19I18nOnce sync.Once

// c19LoadTranslations loads the real UI translations once, so answers differ per language.
func c19LoadTranslations() {
	c19I18nOnce.Do(func() { i18n.Load(ui.Content, slog.New(slog.NewTextHandler(io.Discard, nil))) })
}

// c19Gzip compresses data as the gzip middleware does.
func c19Gzip(t *testing.T, data []byte) int {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Len()
}

// A1/A2: the palette answer is built once per language and key, carries an ETag and
// "no-cache" (not flowsJSON's no-store), and a registry change rebuilds it.
func TestC19NodeTypesAreCachedPerLanguageAndGeneration(t *testing.T) {
	c19LoadTranslations()
	s, token := newFlowsTestServer(t)
	first := c19NodeTypes(t, s, token, "en", "")
	if first.Code != http.StatusOK {
		t.Fatalf("node-types = %d %s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) || len(etag) != len(`W/""`)+32 {
		t.Fatalf("ETag = %q, want W/ and 32 hex digits", etag)
	}
	if cc := first.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		t.Fatalf("Cache-Control = %q, want no-cache without no-store", cc)
	}
	if ct := first.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if c19Builds(s) != 1 {
		t.Fatalf("builds after the first request = %d", c19Builds(s))
	}
	second := c19NodeTypes(t, s, token, "en", "")
	if c19Builds(s) != 1 || second.Header().Get("ETag") != etag || !bytes.Equal(second.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("the second request was no cache hit: builds %d, ETag %q", c19Builds(s), second.Header().Get("ETag"))
	}
	de := c19NodeTypes(t, s, token, "de", "")
	if c19Builds(s) != 2 || de.Header().Get("ETag") == etag {
		t.Fatalf("German: builds %d, ETag %q (English %q)", c19Builds(s), de.Header().Get("ETag"), etag)
	}
	c19NodeTypes(t, s, token, "en", "")
	c19NodeTypes(t, s, token, "deutsch", "") // normalised to de
	if c19Builds(s) != 2 {
		t.Fatalf("both languages must stay cached; builds = %d", c19Builds(s))
	}

	s.Flows.Registry().Replace(&flows.NodeDef{Type: "c19.extra", Category: "logic", LabelKey: "c19.extra"})
	changed := c19NodeTypes(t, s, token, "en", etag)
	if changed.Code != http.StatusOK || c19Builds(s) != 3 || changed.Header().Get("ETag") == etag ||
		!strings.Contains(changed.Body.String(), `"c19.extra"`) {
		t.Fatalf("after a registry change: %d, builds %d, ETag %q", changed.Code, c19Builds(s), changed.Header().Get("ETag"))
	}
	if again := c19NodeTypes(t, s, token, "de", ""); c19Builds(s) != 4 || !strings.Contains(again.Body.String(), `"c19.extra"`) {
		t.Fatalf("a registry change must drop every language; builds %d", c19Builds(s))
	}
}

// A1: If-None-Match is compared weakly; a match is 304 without a body.
func TestC19NodeTypesAnswerIfNoneMatch(t *testing.T) {
	s, token := newFlowsTestServer(t)
	etag := c19NodeTypes(t, s, token, "en", "").Header().Get("ETag")
	strong := strings.TrimPrefix(etag, "W/")
	for _, header := range []string{etag, strong, `W/"0000", ` + etag, "*"} {
		w := c19NodeTypes(t, s, token, "en", header)
		if w.Code != http.StatusNotModified || w.Body.Len() != 0 || w.Header().Get("ETag") != etag {
			t.Fatalf("If-None-Match %q = %d, %d bytes, ETag %q", header, w.Code, w.Body.Len(), w.Header().Get("ETag"))
		}
		if cc := w.Header().Get("Cache-Control"); strings.Contains(cc, "no-store") {
			t.Fatalf("304 Cache-Control = %q", cc)
		}
	}
	for _, header := range []string{`W/"0000"`, `"` + strings.Repeat("0", 32) + `"`, " "} {
		if w := c19NodeTypes(t, s, token, "en", header); w.Code != http.StatusOK || w.Body.Len() == 0 {
			t.Fatalf("If-None-Match %q = %d, %d bytes", header, w.Code, w.Body.Len())
		}
	}
	if c19Builds(s) != 1 {
		t.Fatalf("revalidation must not rebuild; builds = %d", c19Builds(s))
	}
}

// A2: the cache key is (generation, configuration snapshot, language), stale puts are not
// kept, and the languages are bounded.
func TestC19NodeTypesCacheKey(t *testing.T) {
	var c flowNodeTypesCache
	cfgA, cfgB := &config.Config{}, &config.Config{}
	a := flowNodeTypesAnswer{body: []byte("a"), etag: `W/"a"`}
	c.put(1, cfgA, cfgA, "en", a)
	if got, ok := c.get(1, cfgA, "en"); !ok || got.etag != a.etag {
		t.Fatal("a put must be found")
	}
	for _, miss := range []struct {
		gen  uint64
		cfg  *config.Config
		lang string
	}{{2, cfgA, "en"}, {1, cfgB, "en"}, {1, cfgA, "de"}} {
		if _, ok := c.get(miss.gen, miss.cfg, miss.lang); ok {
			t.Fatalf("get(%d, %p, %s) must miss", miss.gen, miss.cfg, miss.lang)
		}
	}
	c.put(1, cfgB, cfgA, "de", a) // built for a configuration that is no longer current
	if _, ok := c.get(1, cfgB, "de"); ok {
		t.Fatal("an answer for a replaced configuration must not be kept")
	}
	if _, ok := c.get(1, cfgA, "en"); !ok {
		t.Fatal("a refused put must not drop the cached answers")
	}
	c.put(3, cfgA, cfgA, "en", a)
	c.put(2, cfgA, cfgA, "de", a) // a slow request of an older generation
	if _, ok := c.get(3, cfgA, "en"); !ok {
		t.Fatal("an older generation must not replace a newer one")
	}
	if _, ok := c.get(2, cfgA, "de"); ok {
		t.Fatal("an older generation must not be kept")
	}
	for i := range flowNodeTypesCacheLangs + 5 {
		c.put(3, cfgA, cfgA, string(rune('a'+i))+"x", a)
	}
	c.mu.Lock()
	n := len(c.answers)
	c.mu.Unlock()
	if n > flowNodeTypesCacheLangs {
		t.Fatalf("%d languages cached, the bound is %d", n, flowNodeTypesCacheLangs)
	}
	c.put(4, cfgB, cfgB, "en", a)
	if _, ok := c.get(3, cfgA, "en"); ok {
		t.Fatal("a new key must drop the old answers")
	}
}

// A1: the gzip middleware compresses the answer by its content type and keeps the ETag;
// a 304 passes uncompressed.
func TestC19NodeTypesAreGzipped(t *testing.T) {
	s, token := newFlowsTestServer(t)
	handler := gzipMiddleware(http.HandlerFunc(s.handleFlows))
	plain := c19NodeTypes(t, s, token, "en", "")
	call := func(ifNoneMatch string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/desktop/flows/node-types?lang=en", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Accept-Encoding", "gzip, deflate")
		if ifNoneMatch != "" {
			r.Header.Set("If-None-Match", ifNoneMatch)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := call("")
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("ETag") != plain.Header().Get("ETag") {
		t.Fatalf("gzip answer = %d, encoding %q, ETag %q", w.Code, w.Header().Get("Content-Encoding"), w.Header().Get("ETag"))
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(body, plain.Body.Bytes()) {
		t.Fatalf("the decompressed answer differs from the plain one (%v)", err)
	}
	notModified := call(plain.Header().Get("ETag"))
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 || notModified.Header().Get("Content-Encoding") != "" {
		t.Fatalf("gzip 304 = %d, %d bytes, encoding %q", notModified.Code, notModified.Body.Len(), notModified.Header().Get("Content-Encoding"))
	}
}

// A1: the real answer sizes, for the test server and for the full tool configuration of
// 1c-12 (every integration on), with real translations.
func TestC19NodeTypesPayloadSize(t *testing.T) {
	c19LoadTranslations()
	s, token := newFlowsTestServer(t)
	for lang, category := range map[string]string{"en": `"Triggers"`, "de": `"Auslöser"`} {
		w := c19NodeTypes(t, s, token, lang, "")
		if !strings.Contains(w.Body.String(), category) {
			t.Fatalf("the %s answer is not translated (no %s)", lang, category)
		}
		var body struct {
			NodeTypes []map[string]any `json:"node_types"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		t.Logf("test server, %s: %d node types, %d bytes, %d bytes gzipped", lang, len(body.NodeTypes), w.Body.Len(), c19Gzip(t, w.Body.Bytes()))
	}

	full := &Server{Cfg: c12FullToolConfig(t), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	env := newFlowCatalogEnv(full)
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, env); err != nil {
		t.Fatal(err)
	}
	env.refreshRegistry(reg, full.ConfigSnapshot())
	for _, lang := range []string{"en", "de"} {
		answer, err := flowNodeTypesBody(reg, lang)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(answer.body), `"categories"`) {
			t.Fatal("the answer has no categories")
		}
		t.Logf("full configuration, %s: %d node types, %d bytes, %d bytes gzipped", lang, len(reg.All()), len(answer.body), c19Gzip(t, answer.body))
	}
}

// A2: concurrent palette requests while the registry changes always get a complete answer
// (run under -race on aurago-test).
func TestC19NodeTypesConcurrentRequests(t *testing.T) {
	s, token := newFlowsTestServer(t)
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 4 {
				lang := []string{"en", "de"}[(i+j)%2]
				w := c19NodeTypes(t, s, token, lang, "")
				if w.Code != http.StatusOK || !json.Valid(w.Body.Bytes()) {
					errs <- w.Body.String()
				}
			}
		}()
	}
	for i := range 4 {
		s.Flows.Registry().Replace(&flows.NodeDef{Type: "c19.concurrent", Category: "logic", LabelKey: "c19", Version: i + 1})
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("a concurrent request failed: %.200s", e)
	}
	final := c19NodeTypes(t, s, token, "en", "")
	if !strings.Contains(final.Body.String(), `"c19.concurrent"`) {
		t.Fatal("the answer after the changes misses the last definition")
	}
}
