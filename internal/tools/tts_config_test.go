package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/speechlab"
)

func TestBuildTTSConfigSharesSpeechLabSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ready" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, TTSID: "kokoro", TTSOK: true, Voice: "af_bella"})
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.TTS.Provider = "speech_lab"
	cfg.SpeechLab.Enabled = true
	cfg.SpeechLab.ChatOutputEnabled = true
	cfg.SpeechLab.BaseURL = server.URL
	cfg.SpeechLab.TimeoutSeconds = 30

	var previous TTSConfig
	for i, route := range []string{TTSRouteChat, TTSRouteTool, TTSRouteGlass} {
		got := BuildTTSConfig(cfg, route, "de")
		if err := ensureSpeechLabSnapshot(&got); err != nil {
			t.Fatal(route, err)
		}
		if got.SpeechLab.BaseURL != server.URL || got.SpeechLab.ExpectedTTSID != "kokoro" || got.SpeechLab.Voice != "af_bella" {
			t.Fatalf("%s snapshot = %+v", route, got.SpeechLab)
		}
		if i > 0 && (got.SpeechLab.BaseURL != previous.SpeechLab.BaseURL || got.SpeechLab.ExpectedTTSID != previous.SpeechLab.ExpectedTTSID || got.SpeechLab.Voice != previous.SpeechLab.Voice) {
			t.Fatalf("route %s diverged", route)
		}
		previous = got
	}
}

func TestTTSReadyToolSpeechLabRequiresActiveLab(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "speech_lab"
	cfg.SpeechLab.BaseURL = "http://127.0.0.1:9"
	if TTSReady(cfg, TTSRouteTool) {
		t.Fatal("inactive Speech Lab is ready for the tool")
	}
	if got := BuildTTSConfig(cfg, TTSRouteTool, "de"); got.Provider != "speech_lab" {
		t.Fatalf("provider = %q", got.Provider)
	}
}

func TestGlassSpeechLabWithoutReadyDoesNotSynthesize(t *testing.T) {
	var speechCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/audio/speech" {
			speechCalls++
			t.Error("synthesis ran without a ready snapshot")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"tts_ok":false}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "speech_lab"
	cfg.SpeechLab.Enabled = true
	cfg.SpeechLab.BaseURL = server.URL
	cfg.SpeechLab.TimeoutSeconds = 30
	built := BuildTTSConfig(cfg, TTSRouteGlass, "")
	built.Context = t.Context()
	_, err := TTSSynthesize(built, "hallo")
	if err == nil || !strings.Contains(err.Error(), "Speech Lab is not ready") {
		t.Fatalf("error = %v", err)
	}
	if speechCalls != 0 {
		t.Fatalf("speech calls = %d", speechCalls)
	}
	entries, readErr := os.ReadDir(filepath.Join(cfg.Directories.DataDir, "tts"))
	if readErr == nil && len(entries) != 0 {
		t.Fatalf("cache entries = %d", len(entries))
	}
}
