package agent

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
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

// rerankWithRecency takes raw VectorDB results and re-ranks them by combining
// semantic similarity (extracted from the text prefix) with a recency decay factor.
// Recency decay: score = similarity * (1 + recencyBonus), where recencyBonus decays
// from 0.3 (accessed today) to 0.0 (accessed >30 days ago).
func rerankWithRecency(memories []string, docIDs []string, stm *memory.SQLiteMemory, logger *slog.Logger) []rankedMemory {
	metas, err := stm.GetAllMemoryMeta()
	metaMap := make(map[string]memory.MemoryMeta)
	if err == nil {
		for _, m := range metas {
			metaMap[m.DocID] = m
		}
	}

	now := time.Now()
	results := make([]rankedMemory, 0, len(memories))

	for i, mem := range memories {
		if i >= len(docIDs) {
			break
		}
		// Parse similarity from "[Similarity: 0.85] ..."
		sim := memory.ExtractSimilarityScore(mem)
		if sim == 0 {
			sim = 0.5 // fallback for malformed entries
		}

		// Calculate recency bonus:
		// - event freshness: up to +0.35 for new events (last_event_at)
		// - utility freshness: up to +0.15 for recently accessed items (last_accessed)
		// both decay to 0 at 30+ days.
		recencyBonus := 0.0
		if meta, ok := metaMap[docIDs[i]]; ok {
			if eventTime, err := time.Parse("2006-01-02 15:04:05", meta.LastEventAt); err == nil {
				daysSince := now.Sub(eventTime).Hours() / 24
				if daysSince < 30 {
					recencyBonus += 0.35 * (1.0 - daysSince/30.0)
				}
			}
			if lastAccessed, err := time.Parse("2006-01-02 15:04:05", meta.LastAccessed); err == nil {
				daysSince := now.Sub(lastAccessed).Hours() / 24
				if daysSince < 30 {
					recencyBonus += 0.15 * (1.0 - daysSince/30.0)
				}
			}
		}

		finalScore := sim * (1.0 + recencyBonus)
		results = append(results, rankedMemory{text: mem, docID: docIDs[i], score: finalScore})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	return results
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
		// Truncate very long messages
		if len(msg.Content) > 120 {
			return msg.Content[:120] + "…"
		}
		return msg.Content
	}
	return lastUserMsg // fallback
}

// processBehavioralEvents handles V2 mood milestone checks and loneliness calculations
func processBehavioralEvents(stm *memory.SQLiteMemory, messages *[]openai.ChatCompletionMessage, sessionID string, meta memory.PersonalityMeta, logger *slog.Logger) {
	// Loneliness Trait based on time elapsed since last user message
	hours, err := stm.GetHoursSinceLastUserMessage(sessionID)
	if err == nil {
		// Max loneliness after 72 hours (3 days), modified by susceptibility configuration
		loneliness := math.Min(1.0, (hours/72.0)*meta.LonelinessSusceptibility)
		_ = stm.SetTrait(memory.TraitLoneliness, loneliness)
		logger.Debug("[Behavioral Event] Evaluated loneliness", "hours_since_last_msg", hours, "new_loneliness_score", loneliness)
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
