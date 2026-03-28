package agent

import (
	"strings"
	"time"

	"aurago/internal/memory"
)

type emotionBehaviorPolicy struct {
	PromptHint        string
	RecoveryNudge     string
	MaxToolCallsDelta int
}

func latestEmotionState(stm *memory.SQLiteMemory, synthesizer *memory.EmotionSynthesizer) *memory.EmotionState {
	if synthesizer != nil {
		if state := synthesizer.GetLastEmotion(); state != nil {
			clone := *state
			return &clone
		}
	}
	if stm != nil {
		if latest, err := stm.GetLatestEmotion(); err == nil && latest != nil {
			var ts time.Time
			if parsed, parseErr := time.Parse("2006-01-02 15:04:05", latest.Timestamp); parseErr == nil {
				ts = parsed
			}
			return &memory.EmotionState{
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
	return nil
}

func latestEmotionDescription(stm *memory.SQLiteMemory, synthesizer *memory.EmotionSynthesizer) string {
	if state := latestEmotionState(stm, synthesizer); state != nil {
		return strings.TrimSpace(state.Description)
	}
	return ""
}

func deriveEmotionBehaviorPolicy(stm *memory.SQLiteMemory, synthesizer *memory.EmotionSynthesizer) emotionBehaviorPolicy {
	if stm == nil {
		return emotionBehaviorPolicy{}
	}

	state := latestEmotionState(stm, synthesizer)
	traits, _ := stm.GetTraits()

	confidenceTrait := traits[memory.TraitConfidence]
	thoroughnessTrait := traits[memory.TraitThoroughness]
	empathyTrait := traits[memory.TraitEmpathy]

	lowConfidence := confidenceTrait > 0 && confidenceTrait < 0.35
	highThoroughness := thoroughnessTrait > 0.78
	highEmpathy := empathyTrait > 0.8
	tenseRecovery := false

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

	hints := make([]string, 0, 4)
	if lowConfidence {
		hints = append(hints, "When a step could modify or delete data, verify the target first and ask one brief confirmation question if the user intent is ambiguous.")
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

	policy := emotionBehaviorPolicy{}
	if len(hints) > 0 {
		policy.PromptHint = "Emotion-aware runtime guidance: " + strings.Join(hints, " ")
	}
	if tenseRecovery {
		policy.RecoveryNudge = "Inspect the exact last error and make one concrete correction. Avoid speculative retries."
		policy.MaxToolCallsDelta = -1
	}
	if lowConfidence && policy.RecoveryNudge == "" {
		policy.RecoveryNudge = "If the next step could modify or delete data and the request is ambiguous, ask one brief confirmation question instead of guessing."
	}

	return policy
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

func shouldUseEmotionBehavior(policy emotionBehaviorPolicy, state *memory.EmotionState) bool {
	if strings.TrimSpace(policy.PromptHint) != "" || strings.TrimSpace(policy.RecoveryNudge) != "" || policy.MaxToolCallsDelta != 0 {
		return true
	}
	if state == nil {
		return false
	}
	return !state.Timestamp.IsZero() && time.Since(state.Timestamp) < 24*time.Hour
}
