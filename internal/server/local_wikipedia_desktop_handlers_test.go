package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/localwiki"
	"aurago/internal/zim"
)

type fakeLocalWikiPage struct {
	mime     string
	body     string
	resolved string
}

type fakeLocalWikiReader struct {
	edition   localwiki.Edition
	fulltext  bool
	hits      []localwiki.SearchHit
	searchErr error
	refs      []localwiki.Ref
	pages     map[string]fakeLocalWikiPage
	random    localwiki.Ref
	main      localwiki.Ref
	refErr    error
	gotQuery  string
	gotLimit  int
	gotLeads  int
}

func (f *fakeLocalWikiReader) Edition() localwiki.Edition { return f.edition }
func (f *fakeLocalWikiReader) Fulltext() bool             { return f.fulltext }

func (f *fakeLocalWikiReader) Search(_ context.Context, query string, limit int, withLeads int) (localwiki.SearchResult, error) {
	f.gotQuery, f.gotLimit, f.gotLeads = query, limit, withLeads
	if f.searchErr != nil {
		return localwiki.SearchResult{}, f.searchErr
	}
	hits := append([]localwiki.SearchHit(nil), f.hits...)
	return localwiki.SearchResult{Edition: f.edition, Fulltext: f.fulltext, Results: hits}, nil
}

func (f *fakeLocalWikiReader) Suggest(_ context.Context, query string, limit int) ([]localwiki.Ref, error) {
	f.gotQuery, f.gotLimit = query, limit
	return f.refs, nil
}

func (f *fakeLocalWikiReader) Content(path string) (localwiki.ContentItem, error) {
	if path == "" {
		return localwiki.ContentItem{}, localwiki.ErrInvalidPath
	}
	page, ok := f.pages[path]
	if !ok {
		return localwiki.ContentItem{}, zim.ErrNotFound
	}
	resolved := page.resolved
	if resolved == "" {
		resolved = path
	}
	return localwiki.ContentItem{Reader: strings.NewReader(page.body), MimeType: page.mime, Size: int64(len(page.body)), ETag: `"fixture-` + resolved + `"`, Path: resolved}, nil
}

func (f *fakeLocalWikiReader) Random() (localwiki.Ref, error) { return f.random, f.refErr }
func (f *fakeLocalWikiReader) Main() (localwiki.Ref, error)   { return f.main, f.refErr }

type fakeLocalWikiBackend struct {
	status   localwiki.Status
	reader   *fakeLocalWikiReader
	acquired int
	released int
}

func (b *fakeLocalWikiBackend) Status() localwiki.Status { return b.status }

func (b *fakeLocalWikiBackend) AcquireReader() (localWikiReader, func(), bool) {
	if b.reader == nil {
		return nil, func() {}, false
	}
	b.acquired++
	return b.reader, func() { b.released++ }, true
}

func newFakeLocalWikiBackend() *fakeLocalWikiBackend {
	edition := localwiki.Edition{Language: "de", Variant: localwiki.VariantNoPic, Date: "2026-10", Name: "wikipedia_de_all_nopic_2026-10", FileName: "wikipedia_de_all_nopic_2026-10.zim", Size: 18600000000, SHA256: "feedface", UUID: "0123456789abcdef0123456789abcdef", ArticleCount: 2900000}
	reader := &fakeLocalWikiReader{
		edition:  edition,
		fulltext: true,
		hits:     []localwiki.SearchHit{{Ref: localwiki.Ref{Title: "Berlin", Path: "Berlin"}, Snippet: "Berlin ist die Hauptstadt.", Lead: "lead must not leave the server"}},
		refs:     []localwiki.Ref{{Title: "Berlin", Path: "Berlin"}, {Title: "Bern", Path: "Bern"}},
		pages: map[string]fakeLocalWikiPage{
			"Berlin":            {mime: "text/html; charset=utf-8", body: "<!doctype html><title>Berlin</title><p>Berlin ist die Hauptstadt.</p>"},
			"Berlin_(Stadt)":    {resolved: "Berlin"},
			"AC/DC":             {mime: "text/html; charset=utf-8", body: "<title>AC/DC</title>"},
			"_assets_/logo.png": {mime: "image/png", body: "\x89PNG\r\n\x1a\nfixture"},
			"Ausbruch":          {resolved: "../status"},
		},
		random: localwiki.Ref{Title: "Bern", Path: "Bern"},
		main:   localwiki.Ref{Title: "Hauptseite", Path: "Hauptseite"},
	}
	return &fakeLocalWikiBackend{
		status: localwiki.Status{
			State: "ready", Progress: 1, Edition: &edition, Fulltext: true, Readable: true,
			UpdateAvailable: &localwiki.UpdateInfo{Date: "2026-11", Size: 19000000000},
			DataDir:         "/srv/private/wikipedia", FreeBytes: 42, RequiredBytes: 7,
			Selection:      localwiki.Selection{Language: "de", Variant: localwiki.VariantNoPic},
			Recommendation: "Check the AuraGo log for details.",
			SystemLanguage: "de",
			Languages:      []localwiki.LanguageInfo{{Code: "de", Name: "Deutsch", Fulltext: true}},
		},
		reader: reader,
	}
}

func newLocalWikiDesktopTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.LocalWikipedia.Enabled = true
	return s, readToken
}

func localWikiDesktopDo(t *testing.T, s *Server, backend localWikiDesktopBackend, method, target, credential string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	switch credential {
	case "":
	case "session":
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	default:
		r.Header.Set("Authorization", "Bearer "+credential)
	}
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	s.serveLocalWikipediaDesktop(w, r, backend)
	return w
}

func decodeLocalWikiJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return body
}

func TestLocalWikiDesktopAPIRequiresDesktopRead(t *testing.T) {
	s, readToken := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	if w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d", w.Code)
	}
	if w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", "not-a-token", nil); w.Code != http.StatusForbidden {
		t.Fatalf("unknown bearer = %d", w.Code)
	}
	for _, credential := range []string{"session", readToken} {
		if w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", credential, nil); w.Code != http.StatusOK {
			t.Fatalf("status with %q = %d %s", credential, w.Code, w.Body.String())
		}
	}
	w := localWikiDesktopDo(t, s, backend, http.MethodPost, "/api/desktop/local-wikipedia/search", "session", nil)
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST = %d allow=%q", w.Code, w.Header().Get("Allow"))
	}
}

func TestLocalWikiDesktopAPIGates(t *testing.T) {
	s, readToken := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()

	s.Cfg.VirtualDesktop.Enabled = false
	w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", "session", nil)
	if w.Code != http.StatusServiceUnavailable || decodeLocalWikiJSON(t, w)["code"] != "desktop_unavailable" {
		t.Fatalf("desktop off = %d %s", w.Code, w.Body.String())
	}
	s.Cfg.VirtualDesktop.Enabled = true

	s.Cfg.LocalWikipedia.Enabled = false
	for credential, canManage := range map[string]bool{"session": true, readToken: false} {
		w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", credential, nil)
		body := decodeLocalWikiJSON(t, w)
		if w.Code != http.StatusServiceUnavailable || body["code"] != "disabled" || body["can_manage"] != canManage {
			t.Fatalf("disabled for %q = %d %v", credential, w.Code, body)
		}
	}
	s.Cfg.LocalWikipedia.Enabled = true

	w = localWikiDesktopDo(t, s, nil, http.MethodGet, "/api/desktop/local-wikipedia/status", "session", nil)
	if w.Code != http.StatusServiceUnavailable || decodeLocalWikiJSON(t, w)["code"] != "unavailable" {
		t.Fatalf("no manager = %d %s", w.Code, w.Body.String())
	}

	backend.status = localwiki.Status{State: "not_installed"}
	backend.reader = nil
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", "session", nil)
	if body := decodeLocalWikiJSON(t, w); w.Code != http.StatusOK || body["state"] != "not_installed" || body["readable"] != false || body["edition"] != nil {
		t.Fatalf("status without edition = %d %s", w.Code, w.Body.String())
	}
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Berlin", "session", nil)
	if w.Code != http.StatusConflict || decodeLocalWikiJSON(t, w)["code"] != "not_ready" {
		t.Fatalf("search without edition = %d %s", w.Code, w.Body.String())
	}

	// The first load of the storage directory: the manager refuses Acquire
	// until it is done, and the status tells the app to poll.
	backend.status = localwiki.Status{State: "not_installed", Loading: true, ErrorCode: localwiki.CodeBusy}
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", readToken, nil)
	if body := decodeLocalWikiJSON(t, w); w.Code != http.StatusOK || body["loading"] != true || body["readable"] != false || body["error_code"] != "busy" {
		t.Fatalf("loading status = %d %s", w.Code, w.Body.String())
	}
	for _, route := range []string{"suggest?q=Ber", "random", "main"} {
		w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/"+route, readToken, nil)
		if w.Code != http.StatusConflict || decodeLocalWikiJSON(t, w)["code"] != "not_ready" {
			t.Fatalf("%s while loading = %d %s", route, w.Code, w.Body.String())
		}
	}
	// Unknown routes are 404 whether or not an edition is open.
	if w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/unknown", readToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown route without edition = %d", w.Code)
	}
}

func TestLocalWikiDesktopStatusIsTheNonAdminSubset(t *testing.T) {
	s, readToken := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", readToken, nil)
	body := decodeLocalWikiJSON(t, w)
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if got := strings.Join(keys, ","); got != "can_manage,edition,fulltext,loading,progress,readable,state,update_available" {
		t.Fatalf("status keys = %s", got)
	}
	edition, _ := body["edition"].(map[string]any)
	if len(edition) != 4 || edition["language"] != "de" || edition["variant"] != "nopic" || edition["date"] != "2026-10" || edition["article_count"] != float64(2900000) {
		t.Fatalf("edition = %v", edition)
	}
	for _, secret := range []string{"/srv/private", "feedface", "0123456789abcdef", ".zim", "recommendation", "AuraGo log", "Deutsch"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("status leaks %q: %s", secret, w.Body.String())
		}
	}
	if update, _ := body["update_available"].(map[string]any); update["date"] != "2026-11" || body["can_manage"] != false || body["readable"] != true || body["loading"] != false {
		t.Fatalf("status = %v", body)
	}
	backend.status.ErrorCode = "fulltext_unsupported"
	body = decodeLocalWikiJSON(t, localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/status", "session", nil))
	if body["error_code"] != "fulltext_unsupported" || body["can_manage"] != true {
		t.Fatalf("session status = %v", body)
	}
}

func TestLocalWikiDesktopSuggestAndSearch(t *testing.T) {
	s, readToken := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	reader := backend.reader

	w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/suggest?q=Ber", readToken, nil)
	body := decodeLocalWikiJSON(t, w)
	if results, _ := body["results"].([]any); w.Code != http.StatusOK || len(results) != 2 || reader.gotQuery != "Ber" || reader.gotLimit != 10 {
		t.Fatalf("suggest = %d %v query=%q limit=%d", w.Code, body, reader.gotQuery, reader.gotLimit)
	}
	body = decodeLocalWikiJSON(t, localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/suggest?q=%20%20", readToken, nil))
	if results, ok := body["results"].([]any); !ok || len(results) != 0 {
		t.Fatalf("blank suggest = %v", body)
	}

	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Hauptstadt&limit=99", readToken, nil)
	body = decodeLocalWikiJSON(t, w)
	results, _ := body["results"].([]any)
	if w.Code != http.StatusOK || len(results) != 1 || reader.gotLimit != 30 || reader.gotLeads != 0 || body["fulltext"] != true {
		t.Fatalf("search = %d %v limit=%d leads=%d", w.Code, body, reader.gotLimit, reader.gotLeads)
	}
	hit, _ := results[0].(map[string]any)
	if _, hasLead := hit["lead"]; hasLead || hit["snippet"] != "Berlin ist die Hauptstadt." || hit["path"] != "Berlin" || hit["title"] != "Berlin" {
		t.Fatalf("hit = %v", hit)
	}
	if edition, _ := body["edition"].(map[string]any); len(edition) != 4 {
		t.Fatalf("search edition = %v", body["edition"])
	}
	localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Berlin", readToken, nil)
	if reader.gotLimit != 20 {
		t.Fatalf("default limit = %d", reader.gotLimit)
	}

	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q="+url.QueryEscape(strings.Repeat("ä", 201)), readToken, nil)
	if w.Code != http.StatusBadRequest || decodeLocalWikiJSON(t, w)["code"] != "query_too_long" {
		t.Fatalf("long query = %d %s", w.Code, w.Body.String())
	}
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/suggest?q="+url.QueryEscape(strings.Repeat("ä", 201)), readToken, nil)
	if w.Code != http.StatusBadRequest || decodeLocalWikiJSON(t, w)["code"] != "query_too_long" {
		t.Fatalf("long suggest = %d %s", w.Code, w.Body.String())
	}
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Berlin&limit=abc", readToken, nil)
	if w.Code != http.StatusBadRequest || decodeLocalWikiJSON(t, w)["code"] != "bad_limit" {
		t.Fatalf("bad limit = %d %s", w.Code, w.Body.String())
	}

	reader.searchErr = localwiki.ErrQueryTooLong
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q="+url.QueryEscape(strings.Repeat("wort ", 17)), readToken, nil)
	if w.Code != http.StatusBadRequest || decodeLocalWikiJSON(t, w)["code"] != "query_too_long" {
		t.Fatalf("too many words = %d %s", w.Code, w.Body.String())
	}
	// Library.Search wraps the context error into ErrSearchBusy when every
	// search slot stayed taken; busy must win over the timeout it carries.
	reader.searchErr = fmt.Errorf("%w: %w", localwiki.ErrSearchBusy, context.DeadlineExceeded)
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Berlin", readToken, nil)
	if w.Code != http.StatusServiceUnavailable || decodeLocalWikiJSON(t, w)["code"] != "busy" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("busy = %d %s retry=%q", w.Code, w.Body.String(), w.Header().Get("Retry-After"))
	}
	reader.searchErr = context.DeadlineExceeded
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Berlin", readToken, nil)
	if w.Code != http.StatusGatewayTimeout || decodeLocalWikiJSON(t, w)["code"] != "timeout" {
		t.Fatalf("timeout = %d %s", w.Code, w.Body.String())
	}
	reader.searchErr = errors.New("boom: /srv/private/wikipedia/x.zim")
	w = localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/search?q=Berlin", readToken, nil)
	if w.Code != http.StatusInternalServerError || decodeLocalWikiJSON(t, w)["code"] != "search_failed" || strings.Contains(w.Body.String(), "boom") {
		t.Fatalf("failure = %d %s", w.Code, w.Body.String())
	}
	if backend.acquired == 0 || backend.acquired != backend.released {
		t.Fatalf("acquire/release = %d/%d", backend.acquired, backend.released)
	}
}

func TestLocalWikiDesktopRandomAndMain(t *testing.T) {
	s, readToken := newLocalWikiDesktopTestServer(t)
	backend := newFakeLocalWikiBackend()
	for route, want := range map[string]string{"random": "Bern", "main": "Hauptseite"} {
		w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/"+route, readToken, nil)
		body := decodeLocalWikiJSON(t, w)
		if w.Code != http.StatusOK || body["path"] != want || body["title"] != want {
			t.Fatalf("%s = %d %v", route, w.Code, body)
		}
	}
	backend.reader.refErr = zim.ErrNotFound
	if w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/main", readToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing main page = %d", w.Code)
	}
	if w := localWikiDesktopDo(t, s, backend, http.MethodGet, "/api/desktop/local-wikipedia/unknown", readToken, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown route = %d", w.Code)
	}
	if backend.acquired != backend.released {
		t.Fatalf("acquire/release = %d/%d", backend.acquired, backend.released)
	}
}
