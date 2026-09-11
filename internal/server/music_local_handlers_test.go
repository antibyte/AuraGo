package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/acestep"
	"aurago/internal/config"
)

func TestLocalMusicAdminAndMethodGates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Docker.Enabled = true
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	manager := acestep.New(cfg, nil, nil)
	defer manager.Close()
	s := &Server{Cfg: cfg, LocalMusic: manager}
	mux := http.NewServeMux()
	registerLocalMusicRoutes(mux, s)
	for _, test := range []struct {
		method, path, body, token string
		code                      int
	}{
		{"GET", "action", "", "", 405}, {"GET", "probe", "", "", 405}, {"POST", "status", "", "", 405},
		{"GET", "status", "", "invalid", 403}, {"POST", "action", `{"action":"delete"}`, "", 409},
		{"GET", "status", "", "", 200},
	} {
		req := httptest.NewRequest(test.method, "/api/music-generation/local/"+test.path, strings.NewReader(test.body))
		req.Header.Set("Origin", "http://example.com")
		if test.token != "" {
			req.Header.Set("Authorization", "Bearer "+test.token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != test.code {
			t.Errorf("%s %s: %d %s", test.method, test.path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), acestep.VaultKey) {
			t.Fatal("runtime secret in status")
		}
	}
	cfg.Docker.ReadOnly = true
	manager.Configure(cfg)
	req := httptest.NewRequest("POST", "/api/music-generation/local/action", strings.NewReader(`{"action":"start"}`))
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal("read-only start accepted")
	}
	if !isAdminProtectedPath("/api/music-generation/local/status") {
		t.Fatal("missing global admin classification")
	}
	req.Header.Set("Origin", "https://foreign.example")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), "csrf_check_failed") {
		t.Fatal("foreign origin accepted")
	}
}

func TestNoisemakerLocalNoCloudLyrics(t *testing.T) {
	cfg := noisemakerTestConfig(t)
	cfg.MusicGeneration.Enabled = true
	cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	cfg.ResolveProviders()
	s := &Server{Cfg: cfg, LLMClient: noisemakerFakeChatClient{content: "must not be used"}}
	rec := httptest.NewRecorder()
	handleNoisemakerState(s).ServeHTTP(rec, httptest.NewRequest("GET", "/api/desktop/noisemaker/state", nil))
	var state map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &state)
	if state["enabled"] != true || state["supports_controls"] != true || state["llm_available"] != false {
		t.Fatalf("capabilities %v", state)
	}
	rec = httptest.NewRecorder()
	handleNoisemakerEnhance(s).ServeHTTP(rec, httptest.NewRequest("POST", "/api/desktop/noisemaker/enhance", strings.NewReader(`{"kind":"lyrics","text":"Piano"}`)))
	if rec.Code != 400 {
		t.Fatal("local provider allowed cloud lyric helper")
	}
}
