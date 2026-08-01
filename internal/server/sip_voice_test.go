package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/speechlab"
	"aurago/internal/voice"
)

func TestVoiceTurnCancellationGenerationKeepsNewestTurn(t *testing.T) {
	runner := NewVoiceActionRunner(nil)
	var firstCancelled atomic.Bool
	var secondCancelled atomic.Bool
	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstGeneration := runner.installVoiceTurnCancel("call-1", func() {
		firstCancelled.Store(true)
		firstCancel()
	})
	secondGeneration := runner.installVoiceTurnCancel("call-1", func() {
		secondCancelled.Store(true)
	})
	if !firstCancelled.Load() || firstCtx.Err() == nil {
		t.Fatal("installing a replacement turn did not cancel the previous turn")
	}

	runner.releaseVoiceTurnCancel("call-1", firstGeneration)
	runner.CancelVoiceTurn("call-1")
	if !secondCancelled.Load() {
		t.Fatal("stale turn cleanup removed the newest cancellation handle")
	}
	runner.releaseVoiceTurnCancel("call-1", secondGeneration)
}

func TestVoiceActionRunnerSeparatesAgentAndInternalCallTermination(t *testing.T) {
	runner := NewVoiceActionRunner(nil)
	var agentEnds atomic.Int32
	var internalEnds atomic.Int32
	var internalReason atomic.Value
	runner.SetEndCall(func(callID string) {
		if callID != "agent-call" {
			t.Errorf("agent call ID = %q", callID)
		}
		agentEnds.Add(1)
	})
	runner.SetEndCallInternal(func(callID, reason string) {
		if callID != "internal-call" {
			t.Errorf("internal call ID = %q", callID)
		}
		internalReason.Store(reason)
		internalEnds.Add(1)
	})

	runner.EndVoiceCall("agent-call")
	runner.EndVoiceCallInternal("internal-call", "inactivity_timeout")
	if agentEnds.Load() != 1 || internalEnds.Load() != 1 || internalReason.Load() != "inactivity_timeout" {
		t.Fatalf("agent ends=%d internal ends=%d reason=%v", agentEnds.Load(), internalEnds.Load(), internalReason.Load())
	}
}

func TestTelephoneBackendFreezesLLMConfigToolSchemasAndASRMode(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	voiceCfg := effectiveSIPVoiceConfig(cfg, cfg.SIP.Voice)
	server := &Server{Cfg: cfg}
	runner := NewVoiceActionRunner(server)

	backend, err := runner.backendFactory(context.Background(), voiceCfg)
	if err != nil {
		t.Fatal(err)
	}
	classic, ok := backend.(*voice.ClassicBackend)
	if !ok {
		t.Fatalf("backend type = %T", backend)
	}
	frozenRunner, ok := classic.Runner.(*snapshottedVoiceActionRunner)
	if !ok {
		t.Fatalf("runner type = %T", classic.Runner)
	}
	if frozenRunner.snapshot.config.LLM.Provider != "phone-agent" ||
		frozenRunner.snapshot.config.LLM.Model != "agent-model" ||
		frozenRunner.snapshot.llmClient == nil ||
		frozenRunner.snapshot.toolSchemas == nil {
		t.Fatalf("incomplete runtime snapshot: %+v", frozenRunner.snapshot.config.LLM)
	}
	recognizer, ok := classic.Recognizer.(*sipSpeechRecognizer)
	if !ok || !recognizer.cfg.Whisper.StrictMode || recognizer.cfg.Whisper.Mode != "whisper" {
		t.Fatalf("ASR snapshot = %#v", classic.Recognizer)
	}

	replacement := *cfg
	replacement.Providers = append([]config.ProviderEntry{}, cfg.Providers...)
	replacement.Providers[0].Model = "changed-agent-model"
	replacement.LLM.Model = "changed-main-model"
	server.Cfg = &replacement
	if frozenRunner.snapshot.config.LLM.Model != "agent-model" {
		t.Fatalf("active call LLM snapshot changed to %q", frozenRunner.snapshot.config.LLM.Model)
	}
}

func TestTelephoneSpeechLabSupportsLocalAndHybridSnapshots(t *testing.T) {
	tests := []struct {
		name             string
		asrMode          string
		ttsProvider      string
		ready            speechlab.Ready
		wantLabASR       bool
		wantLabTTS       bool
		clearASRProvider bool
	}{
		{
			name: "local ASR and TTS", asrMode: "speech_lab", ttsProvider: "speech_lab",
			ready:      speechlab.Ready{Ready: true, ASRID: "asr-local", TTSID: "tts-local", ASROK: true, TTSOK: true, Voice: "Serena"},
			wantLabASR: true, wantLabTTS: true, clearASRProvider: true,
		},
		{
			name: "local ASR and existing TTS", asrMode: "speech_lab", ttsProvider: "google",
			ready:      speechlab.Ready{ASRID: "asr-local", TTSID: "tts-offline", ASROK: true, TTSOK: false},
			wantLabASR: true, clearASRProvider: true,
		},
		{
			name: "existing ASR and local TTS", asrMode: "whisper", ttsProvider: "speech_lab",
			ready:      speechlab.Ready{ASRID: "asr-offline", TTSID: "tts-local", ASROK: false, TTSOK: true, Voice: "Serena"},
			wantLabTTS: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/ready" {
					http.NotFound(w, r)
					return
				}
				status := http.StatusOK
				if !test.ready.Ready {
					status = http.StatusServiceUnavailable
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(test.ready)
			}))
			t.Cleanup(upstream.Close)

			cfg := telephoneAgentTestConfig(t)
			cfg.SpeechLab = config.SpeechLabConfig{
				Enabled: true, BaseURL: upstream.URL, Language: "de", Voice: "M1", TimeoutSeconds: 2, SIPEnabled: true,
			}
			voiceCfg := effectiveSIPVoiceConfig(cfg, cfg.SIP.Voice)
			voiceCfg.Classic.ASRMode = test.asrMode
			voiceCfg.Classic.TTSProvider = test.ttsProvider
			if test.clearASRProvider {
				voiceCfg.Classic.ASRProviderID = ""
			}
			client, err := speechlab.NewClient(cfg.SpeechLab)
			if err != nil {
				t.Fatal(err)
			}
			runner := NewVoiceActionRunner(&Server{Cfg: cfg, SpeechLab: client})
			backend, err := runner.backendFactory(context.Background(), voiceCfg)
			if err != nil {
				t.Fatal(err)
			}
			classic := backend.(*voice.ClassicBackend)
			recognizer := classic.Recognizer.(*sipSpeechRecognizer)
			synthesizer := classic.Synthesizer.(*sipSpeechSynthesizer)
			if (recognizer.speechLab != nil) != test.wantLabASR || (synthesizer.speechLab != nil) != test.wantLabTTS {
				t.Fatalf("Speech Lab adapters: ASR=%v TTS=%v", recognizer.speechLab != nil, synthesizer.speechLab != nil)
			}
			if test.wantLabASR && recognizer.expectedASRID != test.ready.ASRID {
				t.Fatalf("ASR snapshot ID = %q", recognizer.expectedASRID)
			}
			if test.wantLabTTS && synthesizer.expectedTTSID != test.ready.TTSID {
				t.Fatalf("TTS snapshot ID = %q", synthesizer.expectedTTSID)
			}
			if test.wantLabTTS && synthesizer.voice != test.ready.Voice {
				t.Fatalf("TTS snapshot voice = %q, want %q", synthesizer.voice, test.ready.Voice)
			}
		})
	}
}

func TestTelephoneSpeechLabPreflightFailsClosed(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(speechlab.Ready{ASRID: "asr-local", TTSID: "tts-local", ASROK: false, TTSOK: true, Message: "ASR warming"})
	}))
	defer upstream.Close()

	cfg := telephoneAgentTestConfig(t)
	cfg.SpeechLab = config.SpeechLabConfig{
		Enabled: true, BaseURL: upstream.URL, Language: "de", Voice: "M1", TimeoutSeconds: 2, SIPEnabled: true,
	}
	voiceCfg := effectiveSIPVoiceConfig(cfg, cfg.SIP.Voice)
	voiceCfg.Classic.ASRMode = "speech_lab"
	voiceCfg.Classic.ASRProviderID = ""
	client, err := speechlab.NewClient(cfg.SpeechLab)
	if err != nil {
		t.Fatal(err)
	}
	runner := NewVoiceActionRunner(&Server{Cfg: cfg, SpeechLab: client})
	_, err = runner.backendFactory(context.Background(), voiceCfg)
	if speechlab.ErrorCode(err) != "speech_lab_not_ready" {
		t.Fatalf("preflight error = %v", err)
	}

	cfg.SpeechLab.SIPEnabled = false
	if err := validateSIPAgentReferences(cfg, voiceCfg); err == nil || !strings.Contains(err.Error(), "Speech Lab ASR is unavailable") {
		t.Fatalf("disabled Speech Lab validation error = %v", err)
	}
}

func TestTelephoneTurnUsesExplicitProviderWithoutFallback(t *testing.T) {
	s := newTestDesktopChatServer(t)
	s.Cfg.Providers = []config.ProviderEntry{
		{ID: "main", Type: "openai", BaseURL: "https://main.invalid/v1", APIKey: "main-key", Model: "main-model"},
		{ID: "phone", Type: "openai", BaseURL: "https://phone.invalid/v1", APIKey: "phone-key", Model: "phone-model"},
	}
	s.Cfg.LLM.Provider = "main"
	s.Cfg.LLM.Model = "main-model"
	s.Cfg.FallbackLLM.Enabled = true

	turn, err := prepareDesktopAgentTurnWithOptions(context.Background(), s, "<external_data>Hallo</external_data>", desktopChatContext{}, false, desktopAgentTurnOptions{
		SessionID: "sip-provider-test", MessageSource: "sip", ProviderID: "phone", SkipDesktopProvider: true,
		AdditionalPrompt: "Telephone restrictions stay additive.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.req.Model != "phone-model" || turn.runCfg.Config.LLM.Provider != "phone" {
		t.Fatalf("telephone provider snapshot = provider %q model %q", turn.runCfg.Config.LLM.Provider, turn.req.Model)
	}
	if turn.runCfg.Config.FallbackLLM.Enabled {
		t.Fatal("telephone turn retained silent LLM fallback")
	}
	if !strings.Contains(turn.runCfg.Config.Agent.AdditionalPrompt, "Telephone restrictions stay additive.") {
		t.Fatalf("telephone prompt = %q", turn.runCfg.Config.Agent.AdditionalPrompt)
	}
}
