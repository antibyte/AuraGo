package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestSpeechLabOwnsOnlyAutomaticWebChatOutput(t *testing.T) {
	cfg := &config.Config{}
	cfg.SpeechLab.Enabled = true
	cfg.SpeechLab.BaseURL = "http://127.0.0.1:8765"
	cfg.SpeechLab.ChatOutputEnabled = true

	if !speechLabOwnsAutomaticWebChatOutput(cfg, RunConfig{MessageSource: "web_chat", VoiceOutputActive: true}) {
		t.Fatal("enabled Speech Lab must own automatic web-chat voice output")
	}
	for _, runCfg := range []RunConfig{
		{MessageSource: "web_chat"},
		{MessageSource: "telegram", VoiceOutputActive: true},
		{MessageSource: "sip", VoiceOutputActive: true},
	} {
		if speechLabOwnsAutomaticWebChatOutput(cfg, runCfg) {
			t.Fatalf("Speech Lab unexpectedly took ownership for %+v", runCfg)
		}
	}
	cfg.SpeechLab.ChatOutputEnabled = false
	if speechLabOwnsAutomaticWebChatOutput(cfg, RunConfig{MessageSource: "web_chat", VoiceOutputActive: true}) {
		t.Fatal("disabled chat output must preserve the existing TTS path")
	}
}
