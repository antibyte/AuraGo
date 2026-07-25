package server

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
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

	backend, err := runner.backendFactory(voiceCfg)
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
