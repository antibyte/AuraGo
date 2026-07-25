package server

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
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
