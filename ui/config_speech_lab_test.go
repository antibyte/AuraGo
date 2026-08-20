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
	// Git may materialize text=auto JavaScript files with CRLF on Windows.
	// Keep source-shape assertions independent of the checkout line ending.
	return strings.ReplaceAll(string(data), "\r\n", "\n")
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
		"cfg-section active speech-lab-section",
		"speech-lab-stack-editor",
		"speech-lab-stack-field",
		"speech-lab-experimental-row",
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
		"if (!/^https?:$/.test(url.protocol)",
		"btn-speech-lab",
		"function speechLabStage(",
		"function speechLabIsASR(",
		"const asr = backends.filter(speechLabIsASR)",
		"host_agent_online",
		"vram_total_gb",
		"capability_accelerators",
		"deployment.cleanup_available === true",
		"if (!managed && !cleanupAvailable)",
		"deployment.requested_bundle || ''",
		"speech_lab.deployment.gpu_backend",
		"speechLabHardwareProfileField",
		"speechLabApplyHardwareProfile",
		"hardware_save_first",
		"hardware_auto_fallback",
		"S2S_GPU=auto",
		"S2S_GPU=vulkan",
		"GGML_BACKEND",
		"requestedBundle !== installedBundle",
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
	if strings.Contains(module, `url.protocol = 'http:'`) {
		t.Fatal("Speech Lab browser URL must preserve HTTPS for the embedded Tailscale TLS listener")
	}
}

func TestSpeechLabSectionKeepsAsyncFieldsInNormalFlow(t *testing.T) {
	css := readSpeechLabUIFile(t, "css/config-workspace.css")
	for _, wanted := range []string{
		".pw-page .speech-lab-section {",
		"display: block;",
		"overflow: visible;",
		".pw-page .speech-lab-section > .field-group",
		"position: static;",
		".pw-page .speech-lab-section .field-input",
		"max-width: 100%;",
		".pw-page .speech-lab-section .btn-speech-lab {",
		"linear-gradient(135deg, var(--pw-accent-strong), var(--pw-accent))",
	} {
		if !strings.Contains(css, wanted) {
			t.Fatalf("Speech Lab layout guard missing %q", wanted)
		}
	}
}

func TestSpeechLabASRSelectionUsesExplicitCatalogStage(t *testing.T) {
	module := readSpeechLabUIFile(t, "cfg/speech_lab.js")
	if !strings.Contains(module, "function speechLabIsASR(backend) {\n    return speechLabStage(backend) === 'asr';") {
		t.Fatal("Speech Lab ASR selection must require the explicit catalog asr stage")
	}
	if strings.Contains(module, "backends.filter(item => !speechLabIsTTS(item))") {
		t.Fatal("non-TTS catalog entries must not be exposed as ASR backends")
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
		"state: 'idle'",
		"this.state = 'starting'",
		"this.state = 'stopping'",
		"this.audioContext = new AudioContext()",
		"await this.audioContext.resume()",
		"await this._ensureAudioContextRunning()",
		"audioWorklet.addModule('/js/chat/modules/speech-lab-worklet.js')",
		"new AudioWorkletNode",
		"text(0, 'RIFF')",
		"text(8, 'WAVE')",
		"view.setUint16(20, 1, true)",
		"view.setUint16(34, 16, true)",
		"form.append('audio', wav, 'speech-lab.wav')",
		"payload.error === 'speech_lab_no_speech'",
		"this._t('speech_lab_no_audio'",
		"href=\"/config#speech_lab\"",
	} {
		if !strings.Contains(recorder, wanted) {
			t.Fatalf("Speech Lab recorder missing %q", wanted)
		}
	}
	for _, forbidden := range []string{"MediaRecorder", "createScriptProcessor", "audio/webm", "audio/mpeg", "SpeechRecognition"} {
		if strings.Contains(recorder, forbidden) {
			t.Fatalf("Speech Lab recorder contains forbidden fallback %q", forbidden)
		}
	}
	start := strings.Index(recorder, "async start() {")
	if start < 0 {
		t.Fatal("Speech Lab recorder is missing start()")
	}
	contextStart := strings.Index(recorder[start:], "this.audioContext = new AudioContext()")
	firstAwait := strings.Index(recorder[start:], "const status = await this.refreshStatus()")
	if contextStart < 0 || firstAwait < 0 || contextStart > firstAwait {
		t.Fatal("Speech Lab AudioContext must be created before the first status await in start()")
	}

	bootstrap := readSpeechLabUIFile(t, "js/chat/main/bootstrap.js")
	if !strings.Contains(bootstrap, "const outcome = speechLabRecorder ? await speechLabRecorder.start() : 'browser'") ||
		!strings.Contains(bootstrap, "if (browserSTT)") {
		t.Fatal("browser speech recognition fallback is not selected explicitly")
	}
	if !strings.Contains(bootstrap, "if (outcome === 'failed' || outcome === 'busy') return") {
		t.Fatal("failed Speech Lab starts must not silently switch providers")
	}
	if !strings.Contains(bootstrap, "await speechLabRecorder.send()") ||
		!strings.Contains(bootstrap, "voiceBtn.setAttribute('aria-busy'") {
		t.Fatal("Speech Lab start and stop transitions must lock the microphone action")
	}
	if !strings.Contains(bootstrap, "pendingSpeechLabTurnToken = String(turnToken || '')") {
		t.Fatal("Speech Lab transcription does not retain the single-use turn token")
	}
	network := readSpeechLabUIFile(t, "js/chat/main/network-submit.js")
	if !strings.Contains(network, "pendingSpeechLabTurnToken = ''") ||
		!strings.Contains(network, "X-AuraGo-Speech-Lab-Turn-Token") {
		t.Fatal("Speech Lab turn token is not consumed exactly once by the next request")
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
