package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/speechlab"
	"aurago/internal/tools"
	"aurago/ui"
)

type chatVoiceOutputCaptureBroker struct {
	events []chatVoiceOutputCapturedEvent
}

type chatVoiceOutputCapturedEvent struct {
	event   string
	message string
}

func (b *chatVoiceOutputCaptureBroker) Send(event, message string) {
	b.events = append(b.events, chatVoiceOutputCapturedEvent{event: event, message: message})
}

func (b *chatVoiceOutputCaptureBroker) SendJSON(string) {}

func (b *chatVoiceOutputCaptureBroker) SendLLMStreamDelta(string, string, string, int, string) {}

func (b *chatVoiceOutputCaptureBroker) SendLLMStreamDone(string) {}

func (b *chatVoiceOutputCaptureBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}

func (b *chatVoiceOutputCaptureBroker) SendThinkingBlock(string, string, string) {}

func TestMaybeEmitChatVoiceOutputFallbackEmitsAudio(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })

	var gotText string
	var gotCfg tools.TTSConfig
	chatVoiceOutputSynthesize = func(cfg tools.TTSConfig, text string) (string, error) {
		gotCfg = cfg
		gotText = text
		return "fallback.wav", nil
	}

	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Language = "de"
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:7788"
	runCfg := agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	maybeEmitChatVoiceOutputFallback(cfg, logger, runCfg, broker, "**Hallo** <done/>")

	if gotText != "Hallo" {
		t.Fatalf("fallback synthesized text = %q", gotText)
	}
	if gotCfg.Provider != "supertonic" || gotCfg.Language != "de" {
		t.Fatalf("fallback TTS config = provider %q language %q", gotCfg.Provider, gotCfg.Language)
	}
	if len(base.events) != 1 || base.events[0].event != "audio" {
		t.Fatalf("expected one audio event, got %+v", base.events)
	}
	var payload struct {
		Path     string `json:"path"`
		MimeType string `json:"mime_type"`
		Autoplay bool   `json:"autoplay"`
	}
	if err := json.Unmarshal([]byte(base.events[0].message), &payload); err != nil {
		t.Fatalf("decode audio payload: %v", err)
	}
	if payload.Path != "/tts/fallback.wav" || payload.MimeType != "audio/wav" || !payload.Autoplay {
		t.Fatalf("unexpected audio payload: %+v", payload)
	}
	if !broker.hasTTSAudio() {
		t.Fatal("expected broker to record emitted TTS audio")
	}
}

func TestMaybeEmitChatVoiceOutputFallbackSkipsWhenModelAlreadyEmittedTTS(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(tools.TTSConfig, string) (string, error) {
		t.Fatal("fallback should not synthesize after an existing TTS audio event")
		return "", nil
	}

	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	broker.Send("audio", `{"path":"/tts/model.wav","title":"TTS Audio"}`)

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:7788"
	runCfg := agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}

	maybeEmitChatVoiceOutputFallback(cfg, nil, runCfg, broker, "Hallo")

	if len(base.events) != 1 {
		t.Fatalf("expected only the original audio event, got %+v", base.events)
	}
}

func TestChatVoiceOutputSpeechLabPreflightsAndSnapshotsTTS(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ready" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, ASRID: "asr-local", TTSID: "tts-local", ASROK: true, TTSOK: true})
	}))
	defer upstream.Close()

	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	var gotCfg tools.TTSConfig
	chatVoiceOutputSynthesize = func(cfg tools.TTSConfig, _ string) (string, error) {
		gotCfg = cfg
		return "speech-lab.wav", nil
	}
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.SpeechLab = config.SpeechLabConfig{
		Enabled: true, BaseURL: upstream.URL, Language: "de", Voice: "M1", TimeoutSeconds: 2, ChatOutputEnabled: true,
	}
	client, err := speechlab.NewClient(cfg.SpeechLab)
	if err != nil {
		t.Fatal(err)
	}
	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	maybeEmitChatVoiceOutputFallback(cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Hallo", client)

	if gotCfg.Provider != "speech_lab" || gotCfg.SpeechLab.Client != client || gotCfg.SpeechLab.ExpectedTTSID != "tts-local" {
		t.Fatalf("Speech Lab TTS snapshot = provider %q id %q client=%v", gotCfg.Provider, gotCfg.SpeechLab.ExpectedTTSID, gotCfg.SpeechLab.Client == client)
	}
	if len(base.events) != 1 || base.events[0].event != "audio" {
		t.Fatalf("events = %+v", base.events)
	}
}

func TestChatVoiceOutputSpeechLabFailureKeepsTextAndEmitsStructuredError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(speechlab.Ready{ASRID: "asr-local", TTSID: "tts-local", ASROK: true, TTSOK: false, Message: "TTS warming"})
	}))
	defer upstream.Close()

	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(tools.TTSConfig, string) (string, error) {
		t.Fatal("synthesis must not run when Speech Lab TTS is not ready")
		return "", nil
	}
	cfg := &config.Config{}
	cfg.SpeechLab = config.SpeechLabConfig{
		Enabled: true, BaseURL: upstream.URL, Language: "de", Voice: "M1", TimeoutSeconds: 2, ChatOutputEnabled: true,
	}
	client, err := speechlab.NewClient(cfg.SpeechLab)
	if err != nil {
		t.Fatal(err)
	}
	base := &chatVoiceOutputCaptureBroker{}
	broker := newChatVoiceOutputTrackingBroker(base)
	maybeEmitChatVoiceOutputFallback(cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"}, broker, "Die Textantwort bleibt sichtbar.", client)

	if len(base.events) != 1 || base.events[0].event != "speech_lab_error" {
		t.Fatalf("events = %+v", base.events)
	}
	if !strings.Contains(base.events[0].message, `"code":"speech_lab_not_ready"`) ||
		!strings.Contains(base.events[0].message, `"config_path":"/config#speech_lab"`) {
		t.Fatalf("structured Speech Lab error = %s", base.events[0].message)
	}
	if broker.hasTTSAudio() {
		t.Fatal("failed Speech Lab synthesis was recorded as audio")
	}
}

func TestChatVoiceOutputDoesNotUseSpeechLabWithoutChannelOptIn(t *testing.T) {
	original := chatVoiceOutputSynthesize
	t.Cleanup(func() { chatVoiceOutputSynthesize = original })
	chatVoiceOutputSynthesize = func(tools.TTSConfig, string) (string, error) {
		t.Fatal("Speech Lab must not synthesize when chat_output_enabled is false")
		return "", nil
	}
	cfg := &config.Config{}
	cfg.TTS.Provider = "speech_lab"
	cfg.SpeechLab = config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", Language: "de", Voice: "M1", TimeoutSeconds: 2,
	}
	base := &chatVoiceOutputCaptureBroker{}
	maybeEmitChatVoiceOutputFallback(
		cfg, nil, agent.RunConfig{VoiceOutputActive: true, MessageSource: "web_chat"},
		newChatVoiceOutputTrackingBroker(base), "Hallo",
	)
	if len(base.events) != 0 {
		t.Fatalf("unexpected chat voice events: %+v", base.events)
	}
}

func TestChatVoiceOutputTextSummarizesLongStructuredStatus(t *testing.T) {
	input := `Stand jetzt:

**Erledigt:**

Vulkan/iGPU geprueft: AMD Lucienne mit Mesa-Vulkan-Treibern vorhanden
Modell heruntergeladen: gemma-4-E2B-it-qat-q4_0-unquantized-heretic.i1-Q4_K_M.gguf (~2,9 GB) liegt unter /home/aurago/aurago/agent_workspace/models/llama/
Docker-Volume llama-models angelegt und mit Modell befuellt

**In Arbeit / Blockiert:**

Das Docker-Image llama-cpp-vulkan:latest wird gerade im Hintergrund gebaut. Der erste Build-Versuch ist wegen fehlendem Shader-Compiler (glslc) gefloppt. Der zweite Versuch laeuft jetzt mit spirv-tools und glslang-tools, aber ich kann den aktuellen Build-Status gerade nicht abfragen, ohne denselben Tool-Call zu wiederholen.

**Was als Naechstes passiert, sobald das Image fertig ist:**

Container llama-cpp-vulkan wird erstellt mit Port 9999, OpenAI-kompatibler API, Vulkan-Backend und Restart-Policy.`

	got := chatVoiceOutputText(input)
	if len([]rune(got)) > 180 {
		t.Fatalf("spoken summary is too long (%d runes): %q", len([]rune(got)), got)
	}
	for _, want := range []string{
		"Ein Teil ist erledigt",
		"Build l\u00e4uft noch",
		"Details stehen im Chat",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("spoken summary missing %q: %q", want, got)
		}
	}
	for _, forbidden := range []string{
		"gemma-4",
		"/home/aurago",
		"Docker-Volume",
		"glslc",
		"spirv-tools",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("spoken summary should not include detail %q: %q", forbidden, got)
		}
	}
}

func TestChatVoiceOutputTextUsesConfiguredLanguageForLongFallback(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	input := strings.Repeat("Dies ist ein langer Antwortabschnitt mit neutralem Inhalt. ", 8)

	got := chatVoiceOutputText(input, "de")
	if !strings.Contains(got, "Details stehen im Chat.") {
		t.Fatalf("expected German details suffix for German UI language, got %q", got)
	}
	if strings.Contains(got, "Details are in the chat.") {
		t.Fatalf("expected no English details suffix for German UI language, got %q", got)
	}
}
