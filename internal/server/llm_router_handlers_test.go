package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

func routerServerFixture(t *testing.T) *Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := `providers:
  - id: main
    type: openai
    model: gpt-4o
  - id: code
    type: openai
    model: gpt-4o-mini
llm:
  provider: main
llm_router:
  enabled: true
  areas:
    coding: {provider: code, model: ""}
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	cfg.Providers[1].APIKey = "router-test-fixture"
	vault, err := security.NewVault(strings.Repeat("56", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return &Server{Cfg: cfg, Vault: vault, LLMClient: llm.NewClient(cfg), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestLLMRouterPreviewAndAdminContract(t *testing.T) {
	s := routerServerFixture(t)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer upstream.Close()
	s.Cfg.Providers[1].BaseURL = upstream.URL
	mux := http.NewServeMux()
	registerLLMRouterRoutes(mux, s)
	for _, tc := range []struct {
		method, path, body, origin, token string
		status                            int
	}{
		{"POST", "preview", `{"text":"Implement a Go function"}`, "", "", 200},
		{"GET", "status", "", "", "", 200},
		{"GET", "preview", "", "", "", 405},
		{"POST", "preview", `{"text":"hello"}`, "http://attacker.invalid", "", 403},
		{"POST", "preview", `{"text":"hello"} {}`, "", "", 400},
		{"POST", "preview", `{"text":"hello","provider":"arbitrary"}`, "", "", 400},
		{"POST", "preview", `{"text":"hello"}`, "", "invalid", 403},
		{"POST", "preview", `{"text":"` + strings.Repeat("a", 17000) + `"}`, "", "", 400},
	} {
		req := httptest.NewRequest(tc.method, "http://localhost/api/llm-router/"+tc.path, strings.NewReader(tc.body))
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %s status=%d want %d: %s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "router-test-fixture") || strings.Contains(rec.Body.String(), "Implement a Go function") {
			t.Fatal("preview exposed credentials or prompt")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("local preview contacted a provider")
	}
	if !isAdminProtectedPath("/api/llm-router/preview") {
		t.Fatal("router lost admin token policy")
	}
	s.Cfg.Auth.Enabled = true
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/llm-router/status", nil))
	if rec.Code != 401 {
		t.Fatal("router status accessible without authentication")
	}
}

func TestLLMRouterConfigSaveClearAndReferences(t *testing.T) {
	s := routerServerFixture(t)
	if refs := providerReferences(s.Cfg, "code"); len(refs) != 1 || refs[0].Path != "llm_router.areas.coding.provider" {
		t.Fatalf("missing deletion guard: %+v", refs)
	}
	before, _ := os.ReadFile(s.Cfg.ConfigPath)
	rejected := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rejected, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"llm_router":{"areas":{"coding":{"provider":"missing"}}}}`)))
	if rejected.Code != 400 {
		t.Fatalf("accepted missing provider: %d", rejected.Code)
	}
	after, _ := os.ReadFile(s.Cfg.ConfigPath)
	if string(before) != string(after) {
		t.Fatal("invalid router save changed the file")
	}
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"llm_router":{"enabled":false,"helper_fallback":false,"helper_max_calls_per_hour":0,"areas":{"coding":{"provider":"","model":""}}}}`)))
	if rec.Code != 200 {
		t.Fatalf("save failed %d: %s", rec.Code, rec.Body.String())
	}
	reloaded, err := config.Load(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.LLMRouter.Enabled || reloaded.LLMRouter.HelperFallback || reloaded.LLMRouter.HelperMaxCallsPerHour != 0 || reloaded.LLMRouter.Areas["coding"].Provider != "" {
		t.Fatal("explicit clear/false/zero lost during save")
	}
	get := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(get, httptest.NewRequest("GET", "/api/config", nil))
	var data struct {
		LLMRouter config.LLMRouterConfig `json:"llm_router"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.LLMRouter.Areas) != 9 {
		t.Fatalf("GET lost default assignments: %s", get.Body.String())
	}
}

func TestLLMRouterConfigSaveAgnes(t *testing.T) {
	s := routerServerFixture(t)
	s.Cfg.Providers = append(s.Cfg.Providers, config.ProviderEntry{ID: "agnesai", Type: "agnes", Model: "agnes-3.0-flash"})
	// Runtime Save deliberately preserves the independently managed provider list.
	data, err := yaml.Marshal(map[string]any{"providers": s.Cfg.Providers, "llm": map[string]string{"provider": "main"}, "llm_router": s.Cfg.LLMRouter})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Cfg.ConfigPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"", "agnes-2.5-flash", "agnes-3.0-flash"} {
		patch, err := json.Marshal(map[string]any{"llm_router": map[string]any{"areas": map[string]config.LLMRouterTarget{"coding": {Provider: "agnesai", Model: model}}}})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest("PUT", "/api/config", strings.NewReader(string(patch))))
		if rec.Code != http.StatusOK {
			t.Fatalf("Agnes save failed: %d %s", rec.Code, rec.Body.String())
		}
		reloaded, err := config.Load(s.Cfg.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		if target := reloaded.LLMRouter.Areas["coding"]; target.Provider != "agnesai" || target.Model != model {
			t.Fatalf("Agnes assignment lost during reload: %+v", target)
		}
	}
	before, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{"agnes-image-2.1-flash", "agnes-video-v2.0"} {
		patch, err := json.Marshal(map[string]any{"llm_router": map[string]any{"areas": map[string]config.LLMRouterTarget{"coding": {Provider: "agnesai", Model: model}}}})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest("PUT", "/api/config", strings.NewReader(string(patch))))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("Agnes media override accepted: %d", rec.Code)
		}
		after, err := os.ReadFile(s.Cfg.ConfigPath)
		if err != nil || string(before) != string(after) {
			t.Fatal("invalid Agnes media save changed the configuration", err)
		}
	}
}
