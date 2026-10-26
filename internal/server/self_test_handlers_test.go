package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/selftest"
)

func selfTestHTTPServer(t *testing.T) (*Server, *http.ServeMux) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), integrationCtx: context.Background()}
	mux := http.NewServeMux()
	s.registerSelfTestRoutes(mux)
	t.Cleanup(s.closeSelfTests)
	return s, mux
}

func TestSelfTestRoutesAndOrigin(t *testing.T) {
	s, mux := selfTestHTTPServer(t)
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "test-only-session"
	for _, path := range []string{"/api/self-tests", "/api/self-tests/catalog"} {
		r := httptest.NewRecorder()
		mux.ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 401 {
			t.Fatalf("unprotected route %s: %d", path, r.Code)
		}
	}
	s.Cfg.Auth.Enabled = false
	r := httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest("POST", "/api/self-tests", strings.NewReader(`{}`)))
	if r.Code != 403 {
		t.Fatalf("missing origin accepted: %d", r.Code)
	}
	r = httptest.NewRecorder()
	mux.ServeHTTP(r, httptest.NewRequest("GET", "/api/self-tests/catalog", nil))
	var catalog struct {
		Templates []selftest.Template
		Inventory []selftest.Capability
	}
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &catalog) != nil || len(catalog.Templates) != 320 {
		t.Fatalf("catalog response: %d", r.Code)
	}
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private catalog cached")
	}
}

func TestSelfTestBearerAndCentralAdminClassification(t *testing.T) {
	s, admin, desktop, read := newBearerSchemeTestServer(t)
	mux := http.NewServeMux()
	s.registerSelfTestRoutes(mux)
	for _, path := range []string{"/api/self-tests", "/api/self-tests/catalog", "/api/self-tests/" + strings.Repeat("a", 32) + "/results.json"} {
		if !isAdminProtectedPath(path) {
			t.Fatalf("missing central admin classification: %s", path)
		}
	}
	for _, tc := range []struct {
		token string
		want  int
	}{{admin, 200}, {desktop, 403}, {read, 403}} {
		r := httptest.NewRequest(http.MethodGet, "/api/self-tests/catalog", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		authMiddleware(s, mux).ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("bearer boundary: got %d want %d", w.Code, tc.want)
		}
	}
}

func TestSelfTestTelegramInventoryUsesPresenceOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.BotToken = "SYNTHETIC_PRIVATE_TOKEN"
	list := selfTestIntegrations(cfg)
	if len(list) != 1 || list[0].Name != "integration:telegram" {
		t.Fatalf("telegram missing: %+v", list)
	}
	data, _ := json.Marshal(list)
	if strings.Contains(string(data), cfg.Telegram.BotToken) {
		t.Fatal("credential entered inventory")
	}
}

func TestSelfTestRunWithoutProviderReportsBlockedSetup(t *testing.T) {
	_, mux := selfTestHTTPServer(t)
	req := httptest.NewRequest("POST", "http://example.com/api/self-tests", strings.NewReader(`{"profile":"read","case_ids":["SYS-01"]}`))
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var run selftest.Run
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &run) != nil {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	for _, suffix := range []string{"/report.md", "/results.json"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/self-tests/"+run.ID+suffix, nil))
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("download %s: %d", suffix, rec.Code)
		}
	}
	bad := httptest.NewRecorder()
	mux.ServeHTTP(bad, httptest.NewRequest("GET", "/api/self-tests/../../config.yaml", nil))
	if bad.Code == 200 {
		t.Fatal("invalid record path accepted")
	}
}

func TestSelfTestProbeUsesExistingReadOnlyProbe(t *testing.T) {
	s, _ := selfTestHTTPServer(t)
	count := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if r.Method != "GET" || r.URL.Path != "/api/" {
			t.Errorf("unexpected mutation: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"message":"API running."}`)
	}))
	defer upstream.Close()
	s.Cfg.HomeAssistant.Enabled = true
	s.Cfg.HomeAssistant.URL = upstream.URL
	s.Cfg.HomeAssistant.AccessToken = "worthless-probe-test-value"
	out := s.executeSelfTestProbe(context.Background(), selftest.Case{Tool: "integration:home-assistant", TemplateID: "COM-37"}, nil)
	if out.Verdict != "PASS" || count != 1 || len(out.Evidence) != 1 {
		t.Fatalf("probe failed: %+v count %d", out, count)
	}
	encoded, _ := json.Marshal(out)
	if strings.Contains(string(encoded), "worthless-probe-test-value") || strings.Contains(string(encoded), upstream.URL) {
		t.Fatal("probe data entered report")
	}
}
