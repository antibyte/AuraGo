package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/localwiki"
)

// newLocalWikipediaTestServer wires a manager to a local TLS catalog that
// offers one German nopic edition of 1000 bytes (no mirrors).
func newLocalWikipediaTestServer(t *testing.T, enabled bool, free int64) (*Server, *http.ServeMux) {
	t.Helper()
	return newLocalWikipediaTestServerWithDisk(t, enabled, func(string) (int64, error) { return free, nil })
}

func newLocalWikipediaTestServerWithDisk(t *testing.T, enabled bool, freeDisk func(string) (int64, error)) (*Server, *http.ServeMux) {
	t.Helper()
	catalog := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/catalog/v2/entries":
			fmt.Fprint(w, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><name>wikipedia_de_all</name><flavour>nopic</flavour>`+
				`<articleCount>7</articleCount><link rel="http://opds-spec.org/acquisition/open-access" `+
				`href="/zim/wikipedia/wikipedia_de_all_nopic_2026-10.zim.meta4" length="1000"/></entry></feed>`)
		case strings.HasSuffix(r.URL.Path, ".zim.meta4"):
			fmt.Fprintf(w, `<metalink xmlns="urn:ietf:params:xml:ns:metalink"><file name="wikipedia_de_all_nopic_2026-10.zim">`+
				`<size>1000</size><hash type="sha-256">%s</hash></file></metalink>`, strings.Repeat("a", 64))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(catalog.Close)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Agent.SystemLanguage = "Deutsch"
	cfg.LocalWikipedia = config.LocalWikipediaConfig{Enabled: enabled, AgentAccess: true, Variant: "nopic", UpdateCheck: true}
	manager := localwiki.NewManager(localwiki.Deps{
		HTTPClient:     catalog.Client(),
		FreeDiskBytes:  freeDisk,
		CatalogBaseURL: catalog.URL,
	})
	manager.Configure(localwiki.SettingsFromConfig(cfg))
	manager.Start(context.Background())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	// The first load and the free-space measurement run in the background; a
	// disabled integration measures nothing.
	wantFree := int64(-1)
	if free, err := freeDisk(cfg.Directories.DataDir); err == nil && enabled {
		wantFree = free
	}
	deadline := time.Now().Add(10 * time.Second)
	for status := manager.Status(); status.Loading || status.FreeBytes != wantFree; status = manager.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("the manager did not finish its first load and measurement: %+v", status)
		}
		time.Sleep(5 * time.Millisecond)
	}
	s := &Server{Cfg: cfg, Logger: slog.Default(), LocalWiki: manager}
	mux := http.NewServeMux()
	registerLocalWikipediaRoutes(mux, s)
	return s, mux
}

func localWikipediaRequest(mux *http.ServeMux, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Origin", "http://example.com")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func decodeLocalWikipediaBodyMap(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return body
}

func TestLocalWikipediaRoutesMethodsAndAdminGate(t *testing.T) {
	s, mux := newLocalWikipediaTestServer(t, true, 1<<40)
	status := localWikipediaRequest(mux, http.MethodGet, "/api/local-wikipedia/status", "", nil)
	if status.Code != http.StatusOK || status.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("GET status = %d %s", status.Code, status.Body.String())
	}
	var decoded localwiki.Status
	if err := json.Unmarshal(status.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.State != localwiki.StateNotInstalled || decoded.Selection.Language != "de" ||
		decoded.DataDir != filepath.Join(s.Cfg.Directories.DataDir, "wikipedia") || len(decoded.Languages) != 16 {
		t.Fatalf("status = %+v", decoded)
	}
	// The status is passed through unchanged, including fields clients rely on
	// to decide whether content is available.
	if raw := decodeLocalWikipediaBodyMap(t, status); raw["readable"] != false || raw["free_bytes"] != float64(1<<40) {
		t.Fatalf("status body = %v", raw)
	}
	for _, tc := range []struct{ method, route string }{
		{http.MethodPost, "status"}, {http.MethodPost, "catalog"}, {http.MethodGet, "install"},
		{http.MethodGet, "cancel"}, {http.MethodGet, "delete"}, {http.MethodGet, "check-update"},
	} {
		if got := localWikipediaRequest(mux, tc.method, "/api/local-wikipedia/"+tc.route, "", nil).Code; got != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s = %d, want 405", tc.method, tc.route, got)
		}
	}
	if got := localWikipediaRequest(mux, http.MethodGet, "/api/local-wikipedia/status", "", map[string]string{"Authorization": "Bearer not-a-token"}).Code; got != http.StatusForbidden {
		t.Fatalf("invalid bearer = %d, want 403", got)
	}
	if got := localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/delete", "", map[string]string{"Origin": "https://foreign.example"}); got.Code != http.StatusForbidden ||
		!strings.Contains(got.Body.String(), "csrf_check_failed") {
		t.Fatalf("foreign origin delete = %d %s", got.Code, got.Body.String())
	}
	for _, route := range []string{"catalog", "status", "install", "cancel", "delete", "check-update"} {
		if !isAdminProtectedPath("/api/local-wikipedia/" + route) {
			t.Fatalf("/api/local-wikipedia/%s is not admin protected", route)
		}
	}
}

func TestLocalWikipediaInstallMapsPreflightErrors(t *testing.T) {
	_, disabled := newLocalWikipediaTestServer(t, false, 1<<40)
	got := localWikipediaRequest(disabled, http.MethodPost, "/api/local-wikipedia/install", "{}", nil)
	if got.Code != http.StatusConflict || decodeLocalWikipediaBodyMap(t, got)["error_code"] != localwiki.CodeDisabled {
		t.Fatalf("disabled install = %d %s", got.Code, got.Body.String())
	}

	_, mux := newLocalWikipediaTestServer(t, true, 10)
	got = localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/install", "", nil)
	body := decodeLocalWikipediaBodyMap(t, got)
	if got.Code != http.StatusUnprocessableEntity || body["error_code"] != localwiki.CodeInsufficientDiskSpace ||
		body["required_bytes"] != float64(1000+1<<30) || body["free_bytes"] != float64(10) || body["can_delete_old"] != false {
		t.Fatalf("insufficient space = %d %v", got.Code, body)
	}
	if body["error"] != localwiki.CodeInsufficientDiskSpace || body["recommendation"] != localwiki.Recommendation(localwiki.CodeInsufficientDiskSpace) {
		t.Fatalf("error body = %v", body)
	}
	for _, payload := range []string{
		`{"replace_mode":"everything"}`, `{"unknown":true}`, `{`,
		`{} {}`, `{}x`, `{"replace_mode":"keep_old"}{"confirm_unknown_space":true}`, `{} null`,
	} {
		if got := localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/install", payload, nil); got.Code != http.StatusBadRequest ||
			decodeLocalWikipediaBodyMap(t, got)["error_code"] != "invalid_request" {
			t.Fatalf("install %s = %d %s, want 400 invalid_request", payload, got.Code, got.Body.String())
		}
	}
	// Whitespace after the object is not trailing data: the request reaches the
	// space check again.
	for _, payload := range []string{"{}\n", " \r\n\t", `{"replace_mode":"keep_old"}  `} {
		if got := localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/install", payload, nil); got.Code != http.StatusUnprocessableEntity {
			t.Fatalf("install %q = %d %s, want 422", payload, got.Code, got.Body.String())
		}
	}
	got = localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/cancel", "", nil)
	if got.Code != http.StatusConflict || decodeLocalWikipediaBodyMap(t, got)["error_code"] != localwiki.CodeNoOperation {
		t.Fatalf("cancel without download = %d %s", got.Code, got.Body.String())
	}
	if got := localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/delete", "", nil); got.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", got.Code, got.Body.String())
	}
	if got := localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/check-update", "", nil); got.Code != http.StatusOK {
		t.Fatalf("check-update = %d %s", got.Code, got.Body.String())
	}
}

func TestLocalWikipediaInstallUnknownFreeSpaceNeedsConfirmation(t *testing.T) {
	s, mux := newLocalWikipediaTestServerWithDisk(t, true, func(string) (int64, error) {
		return 0, errors.New("statfs unsupported")
	})
	got := localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/install", "{}", nil)
	if got.Code != http.StatusConflict || decodeLocalWikipediaBodyMap(t, got)["error_code"] != localwiki.CodeFreeSpaceUnknown {
		t.Fatalf("install without confirmation = %d %s", got.Code, got.Body.String())
	}
	got = localWikipediaRequest(mux, http.MethodPost, "/api/local-wikipedia/install", `{"confirm_unknown_space":true}`, nil)
	if got.Code != http.StatusAccepted || decodeLocalWikipediaBodyMap(t, got)["status"] != "accepted" {
		t.Fatalf("confirmed install = %d %s", got.Code, got.Body.String())
	}
	// The fake catalog serves no edition file: the background download ends on
	// its own and the status reports it, so no goroutine outlives the test.
	deadline := time.Now().Add(10 * time.Second)
	for s.LocalWiki.Status().OperationInProgress {
		if time.Now().After(deadline) {
			t.Fatal("the background download did not end")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLocalWikipediaCatalogRoute(t *testing.T) {
	_, mux := newLocalWikipediaTestServer(t, true, 1<<40)
	got := localWikipediaRequest(mux, http.MethodGet, "/api/local-wikipedia/catalog?lang=", "", nil)
	var info localwiki.CatalogInfo
	if err := json.Unmarshal(got.Body.Bytes(), &info); err != nil || got.Code != http.StatusOK {
		t.Fatalf("catalog = %d %s", got.Code, got.Body.String())
	}
	if info.Language != "de" || info.Variants[localwiki.VariantNoPic].Size != 1000 {
		t.Fatalf("empty lang must resolve to the system language: %+v", info)
	}
	if got := localWikipediaRequest(mux, http.MethodGet, "/api/local-wikipedia/catalog?lang=nb", "", nil); got.Code != http.StatusBadRequest {
		t.Fatalf("unknown language = %d", got.Code)
	}
}

func TestLocalWikipediaUnavailableWithoutManager(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	mux := http.NewServeMux()
	registerLocalWikipediaRoutes(mux, s)
	if got := localWikipediaRequest(mux, http.MethodGet, "/api/local-wikipedia/status", "", nil); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("status without manager = %d", got.Code)
	}
}

func TestLocalWikipediaErrorStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{&localwiki.InsufficientSpaceError{Required: 2, Available: 1}, http.StatusUnprocessableEntity},
		{fmt.Errorf("%w: relative", localwiki.ErrDataDirInvalid), http.StatusUnprocessableEntity},
		{localwiki.ErrBusy, http.StatusConflict},
		{localwiki.ErrDisabled, http.StatusConflict},
		{localwiki.ErrUnknownFreeSpace, http.StatusConflict},
		{localwiki.ErrAlreadyInstalled, http.StatusConflict},
		{localwiki.ErrNoOperation, http.StatusConflict},
		{fmt.Errorf("%w: timeout", localwiki.ErrCatalogUnreachable), http.StatusBadGateway},
		{localwiki.ErrUnknownLanguage, http.StatusBadRequest},
		{errors.New(`open C:\secret\path: denied`), http.StatusInternalServerError},
	} {
		recorder := httptest.NewRecorder()
		writeLocalWikipediaManagerError(nil, recorder, tc.err)
		if recorder.Code != tc.status {
			t.Fatalf("%v = %d, want %d", tc.err, recorder.Code, tc.status)
		}
		if strings.Contains(recorder.Body.String(), "secret") {
			t.Fatalf("error details leaked: %s", recorder.Body.String())
		}
	}
}
