package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func newECalcTestServer(authEnabled bool) *Server {
	cfg := &config.Config{}
	if authEnabled {
		cfg.Auth.Enabled = true
		cfg.Auth.SessionSecret = "ecalc-test-secret"
	}
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func serveECalc(s *Server, method, target string, header map[string]string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerECalcRoutes(mux, s)
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeECalcCatalog(t *testing.T, rr *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatalf("catalog is not a JSON object: %v\n%s", err, rr.Body.String())
	}
	return doc
}

func ecalcLang(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var lang string
	if err := json.Unmarshal(decodeECalcCatalog(t, rr)["lang"], &lang); err != nil {
		t.Fatalf("catalog lang: %v", err)
	}
	return lang
}

func TestECalcCatalogServesLocalizedJSONWithETag(t *testing.T) {
	rr := serveECalc(newECalcTestServer(false), http.MethodGet, "/api/ecalc/catalog?lang=de", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
	if etag := rr.Header().Get("ETag"); !strings.HasPrefix(etag, `"ecalc-`) || !strings.HasSuffix(etag, `"`) {
		t.Fatalf("ETag = %q, want a quoted ecalc- tag", etag)
	}
	doc := decodeECalcCatalog(t, rr)
	for _, key := range []string{"version", "lang", "units", "categories", "items"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("catalog is missing %q: %s", key, rr.Body.String())
		}
	}
	if got := ecalcLang(t, rr); got != "de" {
		t.Fatalf("lang = %q, want de", got)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(doc["items"], &items); err != nil || len(items) == 0 {
		t.Fatalf("items must be a non-empty array: %v (%s)", err, doc["items"])
	}
}

func TestECalcCatalogFallsBackToEnglish(t *testing.T) {
	s := newECalcTestServer(false)
	english := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=en", nil)
	for _, target := range []string{"/api/ecalc/catalog", "/api/ecalc/catalog?lang=", "/api/ecalc/catalog?lang=xx"} {
		rr := serveECalc(s, http.MethodGet, target, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", target, rr.Code)
		}
		if got := ecalcLang(t, rr); got != "en" {
			t.Fatalf("%s: lang = %q, want en", target, got)
		}
		if rr.Header().Get("ETag") != english.Header().Get("ETag") {
			t.Fatalf("%s: ETag %q differs from the English one %q", target, rr.Header().Get("ETag"), english.Header().Get("ETag"))
		}
	}
}

func TestECalcCatalogAnswers304ForMatchingETag(t *testing.T) {
	s := newECalcTestServer(false)
	first := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=de", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first answer carries no ETag")
	}
	for _, match := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
		rr := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=de", map[string]string{"If-None-Match": match})
		if rr.Code != http.StatusNotModified {
			t.Fatalf("If-None-Match %q: status = %d, want 304", match, rr.Code)
		}
		if rr.Body.Len() != 0 {
			t.Fatalf("If-None-Match %q: a 304 must not carry a body, got %d bytes", match, rr.Body.Len())
		}
		if got := rr.Header().Get("ETag"); got != etag {
			t.Fatalf("If-None-Match %q: 304 ETag = %q, want %q", match, got, etag)
		}
	}
	stale := serveECalc(s, http.MethodGet, "/api/ecalc/catalog?lang=de", map[string]string{"If-None-Match": `"ecalc-0000000000000000"`})
	if stale.Code != http.StatusOK || stale.Body.Len() == 0 {
		t.Fatalf("a stale If-None-Match must get the full catalog, got status %d with %d bytes", stale.Code, stale.Body.Len())
	}
}

func TestECalcCatalogIsGetOnly(t *testing.T) {
	s := newECalcTestServer(false)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		rr := serveECalc(s, method, "/api/ecalc/catalog", nil)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: status = %d, want 405", method, rr.Code)
		}
		if got := rr.Header().Get("Allow"); got != http.MethodGet {
			t.Fatalf("%s: Allow = %q, want GET", method, got)
		}
	}
}

func TestECalcCatalogRequiresASession(t *testing.T) {
	rr := serveECalc(newECalcTestServer(true), http.MethodGet, "/api/ecalc/catalog", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a session; body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "items") {
		t.Fatalf("an unauthenticated answer must not carry the catalog: %s", rr.Body.String())
	}
}
