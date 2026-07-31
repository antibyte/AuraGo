package agent

import (
	"strings"

	"aurago/internal/config"
)

// speechLabOwnsAutomaticWebChatOutput keeps the model-facing TTS tools out of
// automatic web-chat voice turns. The server emits the final text first, then
// performs the readiness-gated synthesis through its shared Speech Lab client.
func speechLabOwnsAutomaticWebChatOutput(cfg *config.Config, runCfg RunConfig) bool {
	return cfg != nil &&
		cfg.SpeechLab.Active() &&
		cfg.SpeechLab.ChatOutputEnabled &&
		runCfg.VoiceOutputActive &&
		strings.EqualFold(strings.TrimSpace(runCfg.MessageSource), "web_chat")
}
