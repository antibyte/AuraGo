package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/i18n"
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
