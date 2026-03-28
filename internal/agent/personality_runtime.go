package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func resolvePersonalityAnalyzerClient(cfg *config.Config, fallback memory.PersonalityAnalyzerClient) memory.PersonalityAnalyzerClient {
	v2URL := cfg.Personality.V2ResolvedURL
	if v2URL == "" {
		v2URL = cfg.Personality.V2URL
	}
	v2Key := cfg.Personality.V2ResolvedKey
	if v2Key == "" {
		v2Key = cfg.Personality.V2APIKey
	}
	if v2URL == "" {
		return fallback
	}
	if v2Key == "" {
		v2Key = "dummy"
	}
	v2Cfg := openai.DefaultConfig(v2Key)
	v2Cfg.BaseURL = v2URL
	return openai.NewClientWithConfig(v2Cfg)
}

func resolvePersonalityModel(cfg *config.Config) string {
	if m := cfg.Personality.V2ResolvedModel; m != "" {
		return m
	}
	if m := cfg.Personality.V2Model; m != "" {
		return m
	}
	return cfg.LLM.Model
}

func buildPersonalityHistories(recentMsgs []openai.ChatCompletionMessage, extraLabel, extraContent string) (string, string) {
	if len(recentMsgs) > 5 {
		recentMsgs = recentMsgs[len(recentMsgs)-5:]
	}

	var historyBuilder strings.Builder
	var userHistoryBuilder strings.Builder
	for _, m := range recentMsgs {
		if m.Role == openai.ChatMessageRoleSystem {
			continue
		}
		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
		if m.Role == openai.ChatMessageRoleUser {
			userHistoryBuilder.WriteString(fmt.Sprintf("user: %s\n", m.Content))
		}
	}
	if strings.TrimSpace(extraContent) != "" {
		historyBuilder.WriteString(fmt.Sprintf("%s: %s\n", extraLabel, extraContent))
	}
	return historyBuilder.String(), userHistoryBuilder.String()
}

func applyPersonalityMilestones(stm *memory.SQLiteMemory) {
	traits, err := stm.GetTraits()
	if err != nil {
		return
	}
	for _, milestone := range memory.CheckMilestones(traits) {
		has, err := stm.HasMilestone(milestone.Label)
		if err != nil || has {
			continue
		}
		trigger := stm.GetLastMoodTrigger()
		details := fmt.Sprintf("%s %s %.2f", milestone.Trait, milestone.Direction, milestone.Threshold)
		if trigger != "" {
			details = fmt.Sprintf("%s (Trigger: %q)", details, trigger)
		}
		_ = stm.AddMilestone(milestone.Label, details)
	}
}

func launchAsyncPersonalityV2Analysis(
	cfg *config.Config,
	logger *slog.Logger,
	fallbackClient memory.PersonalityAnalyzerClient,
	stm *memory.SQLiteMemory,
	emotionSynthesizer *memory.EmotionSynthesizer,
	recentMsgs []openai.ChatCompletionMessage,
	triggerInfo string,
	triggerType memory.EmotionTriggerType,
	triggerDetail string,
	inactivityHours float64,
	extraLabel string,
	extraContent string,
	meta memory.PersonalityMeta,
	profilingEnabled bool,
	errorCount int,
	successCount int,
) {
	if stm == nil {
		return
	}

	contextHistory, userHistory := buildPersonalityHistories(recentMsgs, extraLabel, extraContent)
	modelName := resolvePersonalityModel(cfg)
	analyzerClient := resolvePersonalityAnalyzerClient(cfg, fallbackClient)

	go func() {
		v2Ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Personality.V2TimeoutSecs)*time.Second)
		defer cancel()

		mood, affDelta, traitDeltas, profileUpdates, err := stm.AnalyzeMoodV2(v2Ctx, analyzerClient, modelName, contextHistory, userHistory, meta, profilingEnabled)
		if err != nil {
			v2URL := cfg.Personality.V2ResolvedURL
			if v2URL == "" {
				v2URL = cfg.Personality.V2URL
			}
			if v2URL == "" {
				v2URL = "(main LLM endpoint)"
			}
			logger.Warn("[Personality V2] Failed to analyze mood",
				"error", err,
				"model", modelName,
				"url", v2URL,
				"timeout_secs", cfg.Personality.V2TimeoutSecs,
				"hint", "check personality engine config or use a dedicated v2_provider")
			return
		}

		_ = stm.LogMood(mood, triggerInfo)
		for trait, delta := range traitDeltas {
			_ = stm.UpdateTrait(trait, delta)
		}
		_ = stm.UpdateTrait(memory.TraitAffinity, affDelta)
		applyPersonalityMilestones(stm)

		if profilingEnabled && len(profileUpdates) > 0 {
			validCategories := map[string]bool{"tech": true, "prefs": true, "interests": true, "context": true, "comm": true}
			count := 0
			for _, pu := range profileUpdates {
				if count >= 1 {
					break
				}
				trimVal := strings.TrimSpace(pu.Value)
				if validCategories[pu.Category] && pu.Key != "" && pu.Value != "" &&
					!strings.EqualFold(pu.Key, trimVal) && len(trimVal) >= 2 &&
					!strings.ContainsAny(pu.Key, " \t") && pu.Key == strings.ToLower(pu.Key) {
					if err := stm.UpsertProfileEntry(pu.Category, pu.Key, pu.Value, "v2"); err != nil {
						logger.Warn("[User Profiling] Failed to upsert profile entry", "key", pu.Key, "error", err)
					}
					count++
				}
			}
			_ = stm.EnforceProfileSizeLimit(50)
			if err := stm.DeduplicateProfileEntries(); err != nil {
				logger.Warn("[User Profiling] Deduplication failed", "error", err)
			}
			if del, down, err := stm.PruneStaleProfileEntries(); err == nil && (del > 0 || down > 0) {
				logger.Debug("[User Profiling] Pruned stale entries", "deleted", del, "downgraded", down)
			}
			logger.Debug("[User Profiling] Profile updates applied", "count", count)
		}

		logger.Debug("[Personality V2] Asynchronous mood analysis complete", "mood", mood, "affinity_delta", affDelta)

		if emotionSynthesizer != nil {
			prevMood := ""
			if prev := emotionSynthesizer.GetLastEmotion(); prev != nil {
				prevMood = string(prev.PrimaryMood)
			}
			moodChanged := prevMood != string(mood)
			shouldSynthesize := cfg.Personality.EmotionSynthesizer.TriggerAlways ||
				(cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange && moodChanged) ||
				emotionSynthesizer.GetLastEmotion() == nil

			if shouldSynthesize {
				traits, _ := stm.GetTraits()
				esInput := memory.EmotionInput{
					UserMessage:     triggerInfo,
					CurrentMood:     mood,
					Traits:          traits,
					LastEmotion:     emotionSynthesizer.GetLastEmotion(),
					ErrorCount:      errorCount,
					SuccessCount:    successCount,
					TimeOfDay:       memory.TimeOfDay(),
					TriggerType:     triggerType,
					TriggerDetail:   triggerDetail,
					InactivityHours: inactivityHours,
				}
				esCtx, esCancel := context.WithTimeout(context.Background(), 15*time.Second)
				_, _ = emotionSynthesizer.SynthesizeEmotion(esCtx, stm, esInput)
				esCancel()
			}
		}
	}()
}
