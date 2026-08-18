package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLiveSpeechLoadsSpeechLabProvider(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "/js/realtime-speech/provider-speech-lab.js") {
		t.Fatal("live-speech module must load the Speech Lab realtime adapter")
	}
	app := readDesktopAssetText(t, "js/desktop/apps/live-speech.js")
	for _, marker := range []string{
		"data-live-speech-lab",
		"data-live-speech-lab-activate",
		"/api/speech-lab/status",
		"/api/speech-lab/deployment/start",
		"/api/realtime-speech/speech-lab/activate",
		"window.AuraRealtimeSpeech.initialize(true)",
		"desktop.live_speech_lab_ready",
	} {
		if !strings.Contains(app, marker) {
			t.Fatalf("live-speech app missing Speech Lab marker %q", marker)
		}
	}
	adapter := readDesktopAssetText(t, "js/realtime-speech/provider-speech-lab.js")
	for _, marker := range []string{
		"AuraRealtimeProviders.speech_lab",
		"/api/realtime-speech/transcribe",
		"/api/realtime-speech/synthesize",
		"aurago_execute",
	} {
		if !strings.Contains(adapter, marker) {
			t.Fatalf("Speech Lab adapter missing marker %q", marker)
		}
	}
	core := readDesktopAssetText(t, "js/realtime-speech/core.js")
	if !strings.Contains(core, `profile.provider === 'speech_lab'`) {
		t.Fatal("realtime core must treat Speech Lab as a keyless profile")
	}
}

func TestDesktopLiveSpeechLabTranslationsExist(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.live_speech_lab_activate",
		"desktop.live_speech_lab_disabled",
		"desktop.live_speech_lab_not_ready",
		"desktop.live_speech_lab_open_config",
		"desktop.live_speech_lab_ready",
		"desktop.live_speech_lab_start",
		"desktop.live_speech_lab_starting",
		"config.realtime_speech.keyless",
		"config.realtime_speech.speech_lab_help",
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		desktop := readLiveSpeechJSON(t, filepath.Join("lang", "desktop", locale+".json"))
		config := readLiveSpeechJSON(t, filepath.Join("lang", "config", locale+".json"))
		for _, key := range keys {
			source := desktop
			if strings.HasPrefix(key, "config.") {
				source = config
			}
			if strings.TrimSpace(source[key]) == "" {
				t.Fatalf("%s missing %s", locale, key)
			}
		}
	}
}

func readLiveSpeechJSON(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}
