package server

import (
	"fmt"
	"strings"

	"aurago/internal/security"
)

func sanitizeEmotionPreview(text string, maxLen int) string {
	text = strings.TrimSpace(security.StripThinkingTags(text))
	for _, openTag := range []string{"<think>", "<thinking>"} {
		if idx := strings.Index(strings.ToLower(text), openTag); idx >= 0 {
			text = strings.TrimSpace(text[:idx])
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if maxLen > 0 && len(text) > maxLen {
		text = strings.TrimSpace(text[:maxLen]) + "…"
	}
	return text
}

func fallbackEmotionPreview(mood, cause, style string) string {
	mood = strings.TrimSpace(mood)
	if mood != "" {
		mood = strings.ToUpper(mood[:1]) + mood[1:]
	}
	cause = sanitizeEmotionPreview(cause, 100)
	style = sanitizeEmotionPreview(style, 80)
	switch {
	case cause != "" && mood != "":
		return fmt.Sprintf("%s because %s.", mood, cause)
	case cause != "":
		return cause
	case style != "" && mood != "":
		return fmt.Sprintf("%s, with a %s tone.", mood, strings.ReplaceAll(style, "_", " "))
	case mood != "":
		return mood
	default:
		return ""
	}
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
			currentEmotion := sanitized.Description
			if currentEmotion == "" {
				currentEmotion = fallbackEmotionPreview(sanitized.PrimaryMood, sanitized.Cause, sanitized.RecommendedResponseStyle)
			}
			response["current_emotion"] = currentEmotion
			response["emotion_timestamp"] = latest.Timestamp
			response["current_emotion_state"] = &sanitized
		}
	}

	return response
}
