package agent

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/memory"
)

type emotionBehaviorPolicy struct {
	PromptHint          string
	CuriosityPromptHint string
	RecoveryNudge       string
	MaxToolCallsDelta   int
}

func latestEmotionState(stm *memory.SQLiteMemory, synthesizer *memory.EmotionSynthesizer) *memory.EmotionState {
	var state *memory.EmotionState
	if synthesizer != nil {
		if last := synthesizer.GetLastEmotion(); last != nil {
			clone := *last
			state = &clone
		}
	}
	if state == nil && stm != nil {
		if latest, err := stm.GetLatestEmotion(); err == nil && latest != nil {
			var ts time.Time
			if parsed, parseErr := time.Parse("2006-01-02 15:04:05", latest.Timestamp); parseErr == nil {
				ts = parsed
			}
			state = &memory.EmotionState{
				Description:              strings.TrimSpace(latest.Description),
				PrimaryMood:              memory.Mood(latest.PrimaryMood),
				SecondaryMood:            strings.TrimSpace(latest.SecondaryMood),
				Valence:                  latest.Valence,
				Arousal:                  latest.Arousal,
				Confidence:               latest.Confidence,
				Cause:                    strings.TrimSpace(latest.Cause),
				Source:                   strings.TrimSpace(latest.Source),
				RecommendedResponseStyle: strings.TrimSpace(latest.RecommendedResponseStyle),
				Timestamp:                ts,
			}
		}
	}
	if stm != nil {
		if affect, err := stm.GetAffectState(); err == nil && affect.Active() {
			if state == nil {
				state = &memory.EmotionState{
					Description: "Current affect is driven by recent environment events.",
					PrimaryMood: affect.Mood,
					Valence:     affect.Valence,
					Arousal:     affect.Arousal,
					Confidence:  0.7,
					Cause:       affect.CauseCode,
					Source:      "affect_integrator",
					Timestamp:   affect.UpdatedAt,
				}
			} else {
				memory.OverlayAffectOnEmotion(state, affect)
			}
		}
	}
	return state
}

func latestEmotionDescription(stm *memory.SQLiteMemory, synthesizer *memory.EmotionSynthesizer) string {
	if state := latestEmotionState(stm, synthesizer); state != nil {
		return strings.TrimSpace(state.Description)
	}
	return ""
}

func deriveEmotionBehaviorPolicy(stm *memory.SQLiteMemory, synthesizer *memory.EmotionSynthesizer, meta memory.PersonalityMeta, messageSource, userMessage string) emotionBehaviorPolicy {
	if stm == nil {
		return emotionBehaviorPolicy{}
	}

	meta = meta.Normalized()
	state := latestEmotionState(stm, synthesizer)
	traits, _ := stm.GetTraits()

	confidenceTrait := traits[memory.TraitConfidence]
	curiosityTrait := traits[memory.TraitCuriosity]
	thoroughnessTrait := traits[memory.TraitThoroughness]
	empathyTrait := traits[memory.TraitEmpathy]

	lowConfidence := confidenceTrait > 0 && confidenceTrait < 0.35
	highThoroughness := thoroughnessTrait > meta.Thresholds.HighThoroughness
	highEmpathy := empathyTrait > meta.Thresholds.HighEmpathy
	tenseRecovery := false
	ambiguousIntent := intentLooksAmbiguous(userMessage)

	if state != nil {
		if state.Confidence > 0 && state.Confidence < 0.45 {
			lowConfidence = true
		}
		style := strings.ToLower(state.RecommendedResponseStyle)
		if strings.Contains(style, "precise") || strings.Contains(style, "focused") || strings.Contains(style, "careful") {
			highThoroughness = true
		}
		if strings.Contains(style, "warm") || strings.Contains(style, "reassuring") || strings.Contains(style, "support") {
			highEmpathy = true
		}
		if state.Confidence >= 0.45 && state.Valence <= -0.25 && state.Arousal >= 0.65 {
			tenseRecovery = true
		}
	}

	hints := make([]string, 0, 5)
	if lowConfidence && ambiguousIntent {
		hints = append(hints, "When a step could modify or delete data, verify the target first and ask one brief confirmation question if the user intent is ambiguous.")
	} else if lowConfidence {
		hints = append(hints, "When a step could modify or delete data, verify the exact target first. Do not ask a confirmation question when the user already named a concrete target.")
	}
	if highThoroughness {
		hints = append(hints, "After making changes, prefer one lightweight verification step such as a focused test, diff, stat, or read-back before declaring success.")
	}
	if tenseRecovery {
		hints = append(hints, "During error recovery, inspect the exact last error and make one concrete correction at a time instead of trying multiple speculative alternatives.")
	}
	if highEmpathy {
		hints = append(hints, "Keep explanations warm and supportive, but stay concise and practical.")
	}
	if channelHint := channelToneHint(messageSource); channelHint != "" {
		hints = append(hints, channelHint)
	}

	policy := emotionBehaviorPolicy{}
	if len(hints) > 0 {
		policy.PromptHint = "Emotion-aware runtime guidance: " + strings.Join(hints, " ")
	}
	shortChannel := isShortChannel(messageSource)
	if curiosityTrait > meta.Thresholds.HighCuriosity && !shortChannel {
		policy.CuriosityPromptHint = "Curiosity-aware runtime guidance: Be more curious and gather a little more context when it naturally fits. Ask casual, optional follow-up questions only when they would help the conversation, and do not interrogate the user. For example, if the user asks for the weather in a place, answer the request first and, if it feels natural, casually ask in the user's language whether they live there. Keep it relaxed and easy to ignore."
	}
	if tenseRecovery {
		policy.RecoveryNudge = "Inspect the exact last error and make one concrete correction. Avoid speculative retries."
		policy.MaxToolCallsDelta = -1
	}
	if highThoroughness && policy.MaxToolCallsDelta == 0 {
		policy.MaxToolCallsDelta = 1
	}
	if lowConfidence && ambiguousIntent && policy.RecoveryNudge == "" {
		policy.RecoveryNudge = "If the next step could modify or delete data and the request is ambiguous, ask one brief confirmation question instead of guessing."
	}

	return policy
}

var filenameLikeToken = regexp.MustCompile(`\S+\.\w{1,8}\b`)

func intentLooksAmbiguous(userMsg string) bool {
	msg := strings.ToLower(strings.TrimSpace(userMsg))
	if msg == "" {
		return true
	}
	destructive := []string{"delete", "remove", "rm ", "lösch", "losch", "entfernen", "drop ", "wipe", "format ", "truncate"}
	hasDestructive := false
	for _, marker := range destructive {
		if strings.Contains(msg, marker) {
			hasDestructive = true
			break
		}
	}
	if !hasDestructive {
		return false
	}
	if strings.ContainsAny(msg, `/\\`) || filenameLikeToken.MatchString(msg) {
		return false
	}
	return utf8.RuneCountInString(msg) < 80
}

func isShortChannel(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "telegram", "sms", "discord", "rocketchat", "telnyx":
		return true
	default:
		return false
	}
}

func channelToneHint(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "telegram", "sms", "discord", "rocketchat", "telnyx":
		return "Channel style: keep replies short and scannable. Do not change tools or safety rules."
	case "virtual_desktop_chat":
		return "Channel style: a slightly informal desktop tone is fine. Do not change tools or safety rules."
	default:
		return ""
	}
}

func temperamentSnapshot(stm *memory.SQLiteMemory, meta memory.PersonalityMeta) string {
	if stm == nil {
		return ""
	}
	meta = meta.Normalized()
	traits, err := stm.GetTraits()
	if err != nil || traits == nil {
		return ""
	}
	switch {
	case traits[memory.TraitThoroughness] > meta.Thresholds.HighThoroughness:
		return "Temperament: thorough. Prefer one verification step. Do not change safety or tool policy."
	case traits[memory.TraitConfidence] > 0 && traits[memory.TraitConfidence] < meta.Thresholds.LowConfidence:
		return "Temperament: cautious. Stay conservative and on-task. Do not ask the parent-chat user for confirmation."
	case traits[memory.TraitCreativity] > meta.Thresholds.HighCreativity:
		return "Temperament: creative. Offer at most one unconventional option if it stays on task. Do not change safety or tool policy."
	default:
		return ""
	}
}

func applyEmotionRecoveryNudge(base string, policy emotionBehaviorPolicy) string {
	base = strings.TrimSpace(base)
	if strings.TrimSpace(policy.RecoveryNudge) == "" {
		return base
	}
	if base == "" {
		return policy.RecoveryNudge
	}
	return base + "\n\n" + strings.TrimSpace(policy.RecoveryNudge)
}

func mergeAdditionalPrompt(base, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	switch {
	case base == "":
		return extra
	case extra == "":
		return base
	default:
		return base + "\n\n" + extra
	}
}

func mergeEmotionBehaviorPrompt(base string, policy emotionBehaviorPolicy) string {
	return mergeAdditionalPrompt(mergeAdditionalPrompt(base, policy.PromptHint), policy.CuriosityPromptHint)
}

func shouldUseEmotionBehavior(policy emotionBehaviorPolicy, state *memory.EmotionState) bool {
	if strings.TrimSpace(policy.PromptHint) != "" ||
		strings.TrimSpace(policy.CuriosityPromptHint) != "" ||
		strings.TrimSpace(policy.RecoveryNudge) != "" ||
		policy.MaxToolCallsDelta != 0 {
		return true
	}
	if state == nil {
		return false
	}
	return !state.Timestamp.IsZero() && time.Since(state.Timestamp) < 24*time.Hour
}
