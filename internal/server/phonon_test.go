package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/security"
)

func TestPhononCatalogRoutesArePassiveAndProtected(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider, cfg.TTS.Language = "phonon", "auto"
	cfg.Server.UILanguage, cfg.Personality.CorePersonality = "de", "secretary"
	cfg.TTS.Phonon.VoiceID = "manual-choice"
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	s := &Server{Cfg: cfg}
	list := requireAdmin(s, handlePhononVoices(s))
	recorder := httptest.NewRecorder()
	list.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.com/api/tts/phonon/voices", nil))
	var data struct {
		Voices  []phonon.Voice `json:"voices"`
		Ready   bool           `json:"ready"`
		Default string         `json:"default_voice_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 200 || len(data.Voices) != 102 || data.Ready || data.Default != phonon.DefaultVoice("de", "secretary") {
		t.Fatalf("catalog response = %d, %d voices, ready %v", recorder.Code, len(data.Voices), data.Ready)
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.DataDir, "phonon")); !os.IsNotExist(err) || cfg.TTS.Phonon.VoiceID != "manual-choice" {
		t.Fatal("passive catalog read installed resources or changed configuration")
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.com/api/tts/phonon/voices", nil)
	request.Header.Set("Authorization", "Bearer invalid-fixture")
	recorder = httptest.NewRecorder()
	list.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatal("non-admin token can read the protected catalog")
	}
	for _, tc := range []struct {
		method, id, origin string
		status             int
	}{
		{http.MethodGet, "missing", "http://example.com", http.StatusMethodNotAllowed},
		{http.MethodPost, "missing", "", http.StatusForbidden},
		{http.MethodPost, "missing", "https://evil.example", http.StatusForbidden},
		{http.MethodPost, "missing", "http://example.com", http.StatusBadRequest},
		{http.MethodPost, "../escape", "http://example.com", http.StatusBadRequest},
	} {
		request := httptest.NewRequest(tc.method, "http://example.com/api/tts/phonon/voices/"+tc.id+"/download", nil)
		if tc.origin != "" {
			request.Header.Set("Origin", tc.origin)
		}
		recorder := httptest.NewRecorder()
		requireAdmin(s, handlePhononVoiceDownload(s)).ServeHTTP(recorder, request)
		if recorder.Code != tc.status {
			t.Fatalf("%s/%s: %d, want %d", tc.method, tc.id, recorder.Code, tc.status)
		}
	}
}

func TestPhononConfigAPIRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(path, []byte("server: {ui_language: de}\ntts: {provider: phonon, language: auto, phonon: {voice_id: \"\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("12", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	for _, patch := range []string{`{"tts":{"phonon":{"voice_id":"3ZKKapPOvuWFcw9f"}}}`, `{"tts":{"language":"fr"}}`} {
		recorder := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
		if recorder.Code != 200 {
			t.Fatalf("config save: %d %s", recorder.Code, recorder.Body.String())
		}
		saved, err := config.Load(path)
		if err != nil || saved.TTS.Phonon.VoiceID != "3ZKKapPOvuWFcw9f" || saved.TTS.Provider != "phonon" || s.ConfigSnapshot().TTS.Phonon.VoiceID != saved.TTS.Phonon.VoiceID {
			t.Fatal("saved voice was lost from disk/live configuration")
		}
	}
	get := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var loaded struct {
		TTS struct {
			Phonon struct {
				VoiceID string `json:"voice_id"`
			} `json:"phonon"`
		} `json:"tts"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &loaded); err != nil || loaded.TTS.Phonon.VoiceID != "3ZKKapPOvuWFcw9f" {
		t.Fatal("config GET omitted the stored voice")
	}
	clear := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(clear, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"tts":{"phonon":{"voice_id":""}}}`)))
	if clear.Code != 200 || s.ConfigSnapshot().TTS.Phonon.VoiceID != "" {
		t.Fatal("automatic persona voice could not be restored")
	}
}

func TestPhononChatDeviceAndTelephoneConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider, cfg.TTS.Language = "phonon", "auto"
	cfg.Personality.CorePersonality, cfg.TTS.Phonon.VoiceID = "servant", "chosen-voice"
	for _, language := range []string{"de", "en-US", "fr-CA", "es", "pt-BR", "it", "ru"} {
		cfg.Server.UILanguage = language
		got := buildChatVoiceOutputTTSConfig(cfg, "")
		if !chatVoiceOutputTTSConfigured(cfg) || !agodeskTTSConfigured(cfg) || got.Language != language || got.Phonon.Persona != "servant" || got.Phonon.VoiceID != "chosen-voice" {
			t.Fatalf("shared local speech configuration lost settings for %s", language)
		}
	}
	cfg.TTS.Language = "fr"
	if got := buildChatVoiceOutputTTSConfig(cfg, "de"); got.Language != "fr" {
		t.Fatal("explicit speech language lost precedence")
	}
}
