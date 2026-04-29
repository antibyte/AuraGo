package agent

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

// rankedMemory holds a memory result annotated with its final score after recency boost.
type rankedMemory struct {
	text  string
	docID string
	score float64
}

// rerankWithRecency preserves the historic helper name but now delegates to the
// central memory ranking policy, which combines similarity, recency, and
// confidence/provenance signals into one score.
func rerankWithRecency(memories []string, docIDs []string, stm *memory.SQLiteMemory, logger *slog.Logger) []rankedMemory {
	return rankMemoryCandidates(memories, docIDs, stm, nil, time.Now())
}

// moodTrigger returns the last real human message from the conversation,
// skipping auto-injected nudge messages inserted by the agent loop.
func getMoodTrigger(messages []openai.ChatCompletionMessage, lastUserMsg string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != openai.ChatMessageRoleUser {
			continue
		}
		// Skip internal injected nudge messages
		if strings.HasPrefix(msg.Content, "Tool manuals loaded for:") ||
			strings.HasPrefix(msg.Content, "[INTERNAL]") ||
			strings.HasPrefix(msg.Content, "[Tool Output]") ||
			strings.HasPrefix(msg.Content, "Tool Output:") {
			continue
		}
		// Truncate very long messages — use rune-safe slicing to avoid splitting multi-byte UTF-8 characters.
		runes := []rune(msg.Content)
		if len(runes) > 120 {
			return string(runes[:120]) + "…"
		}
		return msg.Content
	}
	return lastUserMsg // fallback
}

// processBehavioralEvents handles V2 mood milestone checks and loneliness calculations
func processBehavioralEvents(stm *memory.SQLiteMemory, messages *[]openai.ChatCompletionMessage, sessionID string, meta memory.PersonalityMeta, logger *slog.Logger) {
	// Loneliness Trait based on time elapsed since last user message.
	// Incremental update: nudge toward the time-based target instead of hard-setting
	// the absolute value, so that other dynamics (e.g. positive interactions) are preserved.
	hours, err := stm.GetHoursSinceLastUserMessage(sessionID)
	if err == nil {
		traits, _ := stm.GetTraits()
		current := 0.0
		if traits != nil {
			current = traits[memory.TraitLoneliness]
		}
		target := math.Min(1.0, (hours/72.0)*meta.LonelinessSusceptibility)
		// Adjust convergence rate based on recency:
		// - If the user was active recently (< 6h), converge faster toward the low target
		//   so loneliness drops quickly during active use.
		// - If the user has been away longer, use the gentler 20% rate to let loneliness
		//   build up more gradually.
		convergenceRate := 0.2
		if hours < 6 {
			convergenceRate = 0.6
		}
		delta := (target - current) * convergenceRate
		if delta != 0 {
			_ = stm.UpdateTrait(memory.TraitLoneliness, delta)
		}
		logger.Debug("[Behavioral Event] Evaluated loneliness", "hours_since_last_msg", hours, "target", target, "current", current, "delta", delta, "convergence_rate", convergenceRate)
	}

	// Narrative Events based on Milestones
	if traits, err := stm.GetTraits(); err == nil {
		triggered := memory.CheckMilestones(traits)
		for _, m := range triggered {
			has, err := stm.HasMilestone(m.Label)
			if err != nil {
				logger.Error("[Behavioral Event] Skipping milestone check due to DB error", "error", err)
				continue
			}
			if !has {
				_ = stm.AddMilestone(m.Label, fmt.Sprintf("Triggered by %s %s %f", m.Trait, m.Direction, m.Threshold))

				// Apply persistent milestone effects (trait floors, decay resistance)
				if err := memory.ApplyMilestoneEffect(stm, m.Label); err != nil {
					logger.Error("[Behavioral Event] Failed to apply milestone effect", "milestone", m.Label, "error", err)
				} else {
					logger.Info("[Behavioral Event] Applied persistent milestone effect", "milestone", m.Label)
				}

				// Inject a proactive system message to prompt the agent to adapt its behavior
				eventMsg := fmt.Sprintf("Note: You have just reached a psychological state: '%s'. Do NOT announce this state or mention the milestone to the user. Instead, simply let this state profoundly, yet subtly, shift your tone, reasoning, and response style according to your core personality.", m.Label)
				*messages = append(*messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: eventMsg})
				logger.Info("[Behavioral Event] Injected milestone event into context", "milestone", m.Label)
			}
		}
	}
}
