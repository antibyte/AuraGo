package server

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"aurago/internal/memory"
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
	return clampPreviewRunes(text, maxLen)
}

func clampPreviewRunes(text string, maxLen int) string {
	if maxLen <= 0 || utf8.RuneCountInString(text) <= maxLen {
		return text
	}
	return strings.TrimSpace(string([]rune(text)[:maxLen])) + "…"
}

func affectEventExposesUserText(event memory.AffectEventRecord) bool {
	switch strings.TrimSpace(event.CauseCode) {
	case memory.AffectCauseConversation, memory.AffectCausePositiveFeedback, memory.AffectCauseNegativeFeedback:
		return true
	}
	return strings.EqualFold(strings.TrimSpace(event.Source), "chat")
}

func sanitizeAffectEventRecord(event memory.AffectEventRecord) memory.AffectEventRecord {
	if affectEventExposesUserText(event) {
		event.Detail = ""
		return event
	}
	event.Detail = sanitizeEmotionPreview(event.Detail, 120)
	return event
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

func fallbackPersonalityTraits() memory.PersonalityTraits {
	return memory.PersonalityTraits{
		memory.TraitCuriosity:    0.5,
		memory.TraitThoroughness: 0.5,
		memory.TraitCreativity:   0.5,
		memory.TraitEmpathy:      0.5,
		memory.TraitConfidence:   0.5,
		memory.TraitAffinity:     0.5,
		memory.TraitLoneliness:   0.0,
	}
}

func (s *Server) buildPersonalityStatePayload() map[string]interface{} {
	if !s.Cfg.Personality.Engine {
		return map[string]interface{}{"enabled": false}
	}

	traits, err := s.ShortTermMem.GetTraits()
	if err != nil {
		s.Logger.Error("Failed to get personality traits", "error", err)
		traits = fallbackPersonalityTraits()
	}

	mood := s.ShortTermMem.GetCurrentMood()
	trigger := s.ShortTermMem.GetLastMoodTrigger()

	notes, _ := s.ShortTermMem.ListCharacterNotes()
	if notes == nil {
		notes = []memory.CharacterNote{}
	}
	milestones, _ := s.ShortTermMem.UnreviewedMilestones()
	if milestones == nil {
		milestones = []string{}
	}
	response := map[string]interface{}{
		"enabled":           true,
		"mood":              string(mood),
		"trigger":           trigger,
		"traits":            traits,
		"character_notes":   notes,
		"narrative_visible": s.ShortTermMem.NarrativeVisible(),
		"needs_discussion":  milestones,
	}
	if affect, err := s.ShortTermMem.GetAffectState(); err == nil && affect.Active() {
		response["affect_cause"] = affect.CauseCode
		response["affect_valence"] = affect.Valence
		response["affect_arousal"] = affect.Arousal
	}
	events, _ := s.ShortTermMem.ListAffectEvents(12)
	if events == nil {
		events = []memory.AffectEventRecord{}
	}
	for i := range events {
		events[i] = sanitizeAffectEventRecord(events[i])
	}
	response["affect_events"] = events

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
