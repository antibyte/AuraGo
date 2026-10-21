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
				DynamicsEpoch:            latest.DynamicsEpoch,
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
		snapshot, err := stm.GetPersonalitySnapshotAt(time.Now())
		if err != nil {
			return nil
		}
		if state != nil && state.DynamicsEpoch != snapshot.Epoch {
			state = nil
		}
		if affect := snapshot.Affect; affect.Active() {
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

	return emotionBehaviorPolicy{PromptHint: channelToneHint(messageSource)}
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
		return "Channel style: keep replies short and scannable while preserving the active persona's voice. Do not change tools or safety rules."
	case "virtual_desktop_chat":
		return "Channel style: use the active persona's natural register for desktop conversation. Do not change tools or safety rules."
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
		strings.TrimSpace(policy.RecoveryNudge) != "" {
		return true
	}
	if state == nil {
		return false
	}
	return !state.Timestamp.IsZero() && time.Since(state.Timestamp) < 24*time.Hour
}
