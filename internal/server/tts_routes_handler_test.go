package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestTTSRoutesHandlerEffectiveProviders(t *testing.T) {
	for _, tt := range []struct {
		name      string
		configure func(*config.Config)
		providers [3]string
		ready     [3]bool
	}{
		{name: "unconfigured"},
		{name: "phonon", configure: func(c *config.Config) { c.TTS.Provider = "phonon" }, providers: [3]string{"phonon", "phonon", "phonon"}, ready: [3]bool{true, true, true}},
		{name: "legacy piper", configure: func(c *config.Config) { c.TTS.Piper.Enabled = true }, providers: [3]string{"piper", "piper", "piper"}, ready: [3]bool{true, true, true}},
		{name: "speech lab chat override", configure: func(c *config.Config) {
			c.TTS.Provider = "phonon"
			c.SpeechLab.Enabled = true
			c.SpeechLab.BaseURL = "http://127.0.0.1:9"
			c.SpeechLab.ChatOutputEnabled = true
		}, providers: [3]string{"speech_lab", "phonon", "phonon"}, ready: [3]bool{true, true, true}},
		{name: "inactive speech lab", configure: func(c *config.Config) { c.TTS.Provider = "speech_lab" }, providers: [3]string{"", "speech_lab", "speech_lab"}},
		{name: "missing key", configure: func(c *config.Config) { c.TTS.Provider = "openai" }, providers: [3]string{"openai", "openai", "openai"}},
		{name: "configured key", configure: func(c *config.Config) {
			c.TTS.Provider = "openai"
			c.TTS.OpenAI.APIKey = "route-test-secret"
			c.TTS.OpenAI.VoiceID = "route-test-voice"
		}, providers: [3]string{"openai", "openai", "openai"}, ready: [3]bool{true, true, true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			if tt.configure != nil {
				tt.configure(cfg)
			}
			rec := httptest.NewRecorder()
			handleTTSRoutes(&Server{Cfg: cfg}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tts/routes", nil))
			if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("status = %d, headers = %v", rec.Code, rec.Header())
			}
			var body struct {
				Routes []map[string]any `json:"routes"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Routes) != 4 {
				t.Fatalf("route count = %d", len(body.Routes))
			}
			for i, code := range []string{"web_chat", "tool", "glass", "live_speech"} {
				route := body.Routes[i]
				provider, ready := "", false
				if i < 3 {
					provider, ready = tt.providers[i], tt.ready[i]
				}
				if len(route) != 3 || route["code"] != code || route["provider"] != provider || route["ready"] != ready {
					t.Errorf("route %d = %#v, want %s/%s/%t", i, route, code, provider, ready)
				}
			}
			for _, hidden := range []string{"route-test-secret", "route-test-voice", "http://"} {
				if strings.Contains(rec.Body.String(), hidden) {
					t.Errorf("response includes private configuration")
				}
			}
		})
	}
}

func TestTTSRoutesHandlerLiveSpeech(t *testing.T) {
	for _, tt := range []struct {
		name, provider, key string
		disabled, missing   bool
		wantProvider        string
		ready               bool
	}{
		{name: "missing key", provider: "openai", wantProvider: "openai"},
		{name: "configured key", provider: "openai", key: "live-route-test-secret", wantProvider: "openai", ready: true},
		{name: "keyless provider", provider: "speech_lab", wantProvider: "speech_lab", ready: true},
		{name: "unknown provider", provider: "unknown", wantProvider: "unknown"},
		{name: "disabled profile", provider: "speech_lab", disabled: true},
		{name: "missing profile", provider: "speech_lab", missing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.RealtimeSpeech.Enabled = true
			cfg.RealtimeSpeech.DefaultProfile = "selected"
			cfg.RealtimeSpeech.Profiles = []config.RealtimeSpeechProfile{
				{ID: "other", Provider: "speech_lab", Enabled: true},
				{ID: "selected", Provider: tt.provider, Enabled: !tt.disabled, APIKey: tt.key, Voice: "live-route-test-voice"},
			}
			if tt.missing {
				cfg.RealtimeSpeech.DefaultProfile = "absent"
			}
			rec := httptest.NewRecorder()
			handleTTSRoutes(&Server{Cfg: cfg}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tts/routes", nil))
			var body struct {
				Routes []struct {
					Code, Provider string
					Ready          bool
				}
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Routes) != 4 {
				t.Fatalf("route count = %d", len(body.Routes))
			}
			live := body.Routes[3]
			if live.Code != "live_speech" || live.Provider != tt.wantProvider || live.Ready != tt.ready {
				t.Fatalf("live speech route = %+v", live)
			}
			if strings.Contains(rec.Body.String(), "live-route-test-") {
				t.Fatal("response includes private configuration")
			}
		})
	}
}

func TestTTSRoutesHandlerMethodAndAdminGuard(t *testing.T) {
	rec := httptest.NewRecorder()
	handleTTSRoutes(&Server{}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tts/routes", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handleTTSRoutes(&Server{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tts/routes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("empty server status = %d", rec.Code)
	}
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "tts-route-test-session"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	s.registerConfigAPIRoutes(mux, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tts/routes", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", rec.Code)
	}
}
