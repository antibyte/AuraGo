package server

import (
	"strings"

	"aurago/internal/security"
)

func sanitizeEmotionPreview(text string, maxLen int) string {
	text = strings.TrimSpace(security.StripThinkingTags(text))
	text = strings.Join(strings.Fields(text), " ")
	if maxLen > 0 && len(text) > maxLen {
		text = strings.TrimSpace(text[:maxLen]) + "…"
	}
	return text
}

func (s *Server) buildPersonalityStatePayload() map[string]interface{} {
	if !s.Cfg.Personality.Engine {
		return map[string]interface{}{"enabled": false}
	}

	traits, err := s.ShortTermMem.GetTraits()
	if err != nil {
		s.Logger.Error("Failed to get personality traits", "error", err)
		return map[string]interface{}{"enabled": false}
	}

	mood := s.ShortTermMem.GetCurrentMood()
	trigger := s.ShortTermMem.GetLastMoodTrigger()

	response := map[string]interface{}{
		"enabled": true,
		"mood":    string(mood),
		"trigger": trigger,
		"traits":  traits,
	}

	if s.Cfg.Personality.EmotionSynthesizer.Enabled {
		if latest, err := s.ShortTermMem.GetLatestEmotion(); err == nil && latest != nil {
			sanitized := *latest
			sanitized.Description = sanitizeEmotionPreview(latest.Description, 220)
			sanitized.Cause = sanitizeEmotionPreview(latest.Cause, 120)
			response["current_emotion"] = sanitized.Description
			response["emotion_timestamp"] = latest.Timestamp
			response["current_emotion_state"] = &sanitized
		}
	}

	return response
}
