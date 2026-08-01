package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readSpeechLabUIFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestConfigSpeechLabSectionUsesNarrowNativeAPIs(t *testing.T) {
	mainJS := readSpeechLabUIFile(t, "js/config/main.js")
	for _, wanted := range []string{
		"{ key: 'speech_lab'",
		"speech_lab: { m: 'speech_lab', fn: 'renderSpeechLabSection' }",
	} {
		if !strings.Contains(mainJS, wanted) {
			t.Fatalf("config main.js missing %q", wanted)
		}
	}

	module := readSpeechLabUIFile(t, "cfg/speech_lab.js")
	for _, wanted := range []string{
		"function renderSpeechLabSection(",
		"/api/speech-lab/status",
		"/api/speech-lab/capability",
		"/api/speech-lab/catalog",
		"/api/speech-lab/suggestions",
		"/api/speech-lab/stack",
		"method: 'PUT'",
		"showConfirm(",
		"speechLabShowExperimental",
		"speech_lab.chat_llm_provider_id",
		"/api/providers",
		"provider.runtime_chat?.eligible === true",
		"provider.runtime_chat?.configured !== true",
		"backend.default_voice",
		"speechLabStatus?.voice",
		"const SPEECH_LAB_BROWSER_PORT = '8766'",
		"function speechLabBrowserURL(",
		"new URL(window.location.href)",
		"target=\"_blank\"",
	} {
		if !strings.Contains(module, wanted) {
			t.Fatalf("Speech Lab config module missing %q", wanted)
		}
	}
	for _, forbidden := range []string{"<iframe", "llm_id", "/api/proxy", "hf_token", "speechLabField('speech_lab.voice'", "speechLabField('speech_lab.advanced_ui_url'", "data.voice = 'M1'"} {
		if strings.Contains(strings.ToLower(module), forbidden) {
			t.Fatalf("Speech Lab config module contains forbidden surface %q", forbidden)
		}
	}
}

func TestConfigFieldGroupsKeepLabelCardsInNormalFlow(t *testing.T) {
	css := readSpeechLabUIFile(t, "css/config.css")
	start := strings.Index(css, ".field-group {")
	if start < 0 {
		t.Fatal("config.css is missing the shared field-group rule")
	}
	end := strings.Index(css[start:], "}")
	if end < 0 {
		t.Fatal("config.css field-group rule is not closed")
	}
	rule := css[start : start+end]
	if !strings.Contains(rule, "display: block;") {
		t.Fatal("label-based field groups must be block-level to preserve vertical form flow")
	}
}

func TestSpeechLabChatRecorderIsPCMWorkletOnly(t *testing.T) {
	recorder := readSpeechLabUIFile(t, "js/chat/modules/speech-lab-recorder.js")
	for _, wanted := range []string{
		"const MAX_DURATION_MS = 120000",
		"const TARGET_SAMPLE_RATE = 16000",
		"const MAX_WAV_BYTES = 8 * 1024 * 1024",
		"audioWorklet.addModule('/js/chat/modules/speech-lab-worklet.js')",
		"new AudioWorkletNode",
		"text(0, 'RIFF')",
		"text(8, 'WAVE')",
		"view.setUint16(20, 1, true)",
		"view.setUint16(34, 16, true)",
		"form.append('audio', wav, 'speech-lab.wav')",
		"href=\"/config#speech_lab\"",
	} {
		if !strings.Contains(recorder, wanted) {
			t.Fatalf("Speech Lab recorder missing %q", wanted)
		}
	}
	for _, forbidden := range []string{"MediaRecorder", "audio/webm", "audio/mpeg", "SpeechRecognition"} {
		if strings.Contains(recorder, forbidden) {
			t.Fatalf("Speech Lab recorder contains forbidden fallback %q", forbidden)
		}
	}

	bootstrap := readSpeechLabUIFile(t, "js/chat/main/bootstrap.js")
	if !strings.Contains(bootstrap, "const useBrowserSTT = !useSpeechLabSTT") {
		t.Fatal("browser speech recognition is not disabled when Speech Lab input is selected")
	}
	if !strings.Contains(bootstrap, "pendingSpeechLabInput = true") {
		t.Fatal("Speech Lab transcription does not mark the next chat turn")
	}
	network := readSpeechLabUIFile(t, "js/chat/main/network-submit.js")
	if !strings.Contains(network, "pendingSpeechLabInput = false") ||
		!strings.Contains(network, "X-AuraGo-Speech-Lab-Input") {
		t.Fatal("Speech Lab chat marker is not consumed exactly once by the next request")
	}
	bundle := readSpeechLabUIFile(t, "js/chat/bundles/chat-runtime.bundle.js")
	if !strings.Contains(bundle, "/* ui/js/chat/modules/speech-lab-recorder.js */") {
		t.Fatal("chat runtime bundle does not contain the Speech Lab recorder")
	}
}

func TestConfigSpeechLabTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	valuesByLocale := make(map[string]map[string]string, len(locales))
	for _, locale := range locales {
		path := "lang/config/speech_lab/" + locale + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(readSpeechLabUIFile(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		valuesByLocale[locale] = values
	}
	english := valuesByLocale["en"]
	if len(english) == 0 {
		t.Fatal("English Speech Lab translations are empty")
	}
	for _, locale := range locales {
		values := valuesByLocale[locale]
		if len(values) != len(english) {
			t.Fatalf("%s has %d Speech Lab keys, want %d", locale, len(values), len(english))
		}
		for key := range english {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty Speech Lab translation %q", locale, key)
			}
		}
		if locale != "en" && values["config.section.speech_lab.desc"] == english["config.section.speech_lab.desc"] {
			t.Fatalf("%s uses the English Speech Lab description", locale)
		}
	}
}

func TestSpeechLabRecorderTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"chat.speech_lab_local_recording", "chat.speech_lab_settings",
		"chat.speech_lab_audio_worklet_required", "chat.speech_lab_asr_not_ready",
		"chat.speech_lab_microphone_denied", "chat.speech_lab_recorder_start_failed",
		"chat.speech_lab_no_audio", "chat.speech_lab_too_large",
		"chat.speech_lab_transcription_failed", "chat.speech_lab_transcribe",
		"chat.speech_lab_browser_url_missing",
	}
	for _, locale := range locales {
		path := "lang/chat/" + locale + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(readSpeechLabUIFile(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty Speech Lab recorder translation %q", locale, key)
			}
		}
	}
}
