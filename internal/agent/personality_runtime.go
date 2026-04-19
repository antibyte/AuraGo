package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

func resolvePersonalityAnalyzerClient(cfg *config.Config, fallback memory.PersonalityAnalyzerClient) memory.PersonalityAnalyzerClient {
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		client := llm.NewClientFromProvider(helperCfg.ProviderType, helperCfg.BaseURL, helperCfg.APIKey)
		if client != nil {
			return client
		}
	}

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
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		return helperCfg.Model
	}
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

// v2FailCount tracks consecutive V2 LLM failures for the circuit breaker.
var v2FailCount atomic.Int32

// v2PausedUntil stores the Unix nanosecond timestamp until which V2 is paused.
var v2PausedUntil atomic.Int64

// v2CircuitOpen reports whether the V2 circuit breaker is currently open (pausing calls).
func v2CircuitOpen(logger *slog.Logger) bool {
	pausedUntil := v2PausedUntil.Load()
	if pausedUntil == 0 {
		return false
	}
	if time.Now().UnixNano() < pausedUntil {
		return true
	}
	// Pause expired — reset state.
	v2PausedUntil.Store(0)
	v2FailCount.Store(0)
	logger.Info("[Personality V2] Circuit breaker reset after pause")
	return false
}

// v2RecordFailure increments the failure counter and opens the circuit breaker after 3 failures.
func v2RecordFailure(logger *slog.Logger) {
	n := v2FailCount.Add(1)
	if n >= 3 {
		pause := time.Now().Add(5 * time.Minute).UnixNano()
		v2PausedUntil.Store(pause)
		logger.Warn("[Personality V2] Circuit breaker opened after consecutive failures — pausing for 5 minutes",
			"fail_count", n)
	}
}

type personalityV2AnalysisResult struct {
	Mood               memory.Mood
	AffinityDelta      float64
	TraitDeltas        map[string]float64
	ProfileUpdates     []memory.ProfileUpdate
	SynthesizedEmotion *memory.EmotionState
	InnerThought       string // Inner voice thought (1-3 sentences, first person)
	NudgeCategory      string // Category of the inner voice nudge
}

func resolveHelperEmotionBatchState(cfg *config.Config, emotionSynthesizer *memory.EmotionSynthesizer) (bool, *memory.EmotionState) {
	helperEmotionBatchEligible := llm.IsHelperLLMAvailable(cfg) && emotionSynthesizer != nil
	var previousEmotion *memory.EmotionState
	if helperEmotionBatchEligible {
		previousEmotion = emotionSynthesizer.GetLastEmotion()
		helperEmotionBatchEligible = cfg.Personality.EmotionSynthesizer.TriggerAlways ||
			cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange ||
			previousEmotion == nil
	}
	return helperEmotionBatchEligible, previousEmotion
}

func applyPersonalityProfileUpdates(stm *memory.SQLiteMemory, logger *slog.Logger, profileUpdates []memory.ProfileUpdate) {
	if stm == nil || len(profileUpdates) == 0 {
		return
	}
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

func applyPersonalityV2AnalysisResult(
	cfg *config.Config,
	logger *slog.Logger,
	stm *memory.SQLiteMemory,
	emotionSynthesizer *memory.EmotionSynthesizer,
	previousEmotion *memory.EmotionState,
	triggerInfo string,
	triggerType memory.EmotionTriggerType,
	triggerDetail string,
	inactivityHours float64,
	profilingEnabled bool,
	errorCount int,
	successCount int,
	result personalityV2AnalysisResult,
) {
	if stm == nil {
		return
	}

	_ = stm.LogMood(result.Mood, triggerInfo)
	for trait, delta := range result.TraitDeltas {
		if err := stm.UpdateTrait(trait, delta); err != nil {
			logger.Warn("[Personality V2] Failed to update trait", "trait", trait, "delta", delta, "error", err)
		}
	}
	if err := stm.UpdateTrait(memory.TraitAffinity, result.AffinityDelta); err != nil {
		logger.Warn("[Personality V2] Failed to update affinity trait", "delta", result.AffinityDelta, "error", err)
	}

	if profilingEnabled && len(result.ProfileUpdates) > 0 {
		applyPersonalityProfileUpdates(stm, logger, result.ProfileUpdates)
	}

	logger.Debug("[Personality V2] Asynchronous mood analysis complete", "mood", result.Mood, "affinity_delta", result.AffinityDelta)

	// Apply inner voice result if present
	if result.InnerThought != "" && cfg.Personality.InnerVoice.Enabled {
		applyInnerVoiceResult(result.InnerThought, result.NudgeCategory)
		if err := stm.StoreInnerVoice(result.InnerThought, result.NudgeCategory); err != nil {
			logger.Warn("[InnerVoice] Failed to store inner voice", "error", err)
		}
		logger.Info("[InnerVoice] Inner voice generated", "category", result.NudgeCategory, "thought_len", len(result.InnerThought))
	}

	if emotionSynthesizer == nil {
		return
	}
	prevMood := ""
	if previousEmotion != nil {
		prevMood = string(previousEmotion.PrimaryMood)
	}
	moodChanged := prevMood != string(result.Mood)
	shouldSynthesize := cfg.Personality.EmotionSynthesizer.TriggerAlways ||
		(cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange && moodChanged) ||
		previousEmotion == nil
	if !shouldSynthesize {
		return
	}

	if result.SynthesizedEmotion != nil {
		if err := emotionSynthesizer.ApplyExternalState(stm, result.SynthesizedEmotion, triggerInfo); err == nil {
			return
		} else {
			logger.Warn("[EmotionSynthesizer] Failed to apply batched helper emotion", "error", err)
			return
		}
	}

	traits, _ := stm.GetTraits()
	esInput := memory.EmotionInput{
		UserMessage:     triggerInfo,
		CurrentMood:     result.Mood,
		Traits:          traits,
		LastEmotion:     previousEmotion,
		ErrorCount:      errorCount,
		SuccessCount:    successCount,
		TimeOfDay:       memory.TimeOfDay(),
		TriggerType:     triggerType,
		TriggerDetail:   triggerDetail,
		InactivityHours: inactivityHours,
		PersonaName:     cfg.Personality.CorePersonality,
		PersonaPrompt:   prompts.GetCorePersonalityPromptSummary(cfg.Directories.PromptsDir, cfg.Personality.CorePersonality, 300),
	}
	esCtx, esCancel := context.WithTimeout(context.Background(), 15*time.Second)
	_, _ = emotionSynthesizer.SynthesizeEmotion(esCtx, stm, esInput)
	esCancel()
}

func normalizeHelperTurnPersonalityResult(payload helperTurnPersonalityBlock, meta memory.PersonalityMeta) (personalityV2AnalysisResult, bool) {
	mood, affinityDelta, traitDeltas, profileUpdates, ok := memory.NormalizeHelperMoodAnalysis(
		payload.MoodAnalysis.UserSentiment,
		payload.MoodAnalysis.AgentMood,
		payload.MoodAnalysis.RelationshipDelta,
		payload.MoodAnalysis.TraitDeltas,
		payload.MoodAnalysis.ProfileUpdates,
		meta,
	)
	if !ok {
		return personalityV2AnalysisResult{}, false
	}

	var synthesizedEmotion *memory.EmotionState
	if emotionJSON, err := json.Marshal(payload.EmotionState); err == nil {
		if parsedEmotion, parseErr := memory.ParseStructuredEmotionState(string(emotionJSON), mood); parseErr == nil {
			synthesizedEmotion = parsedEmotion
		}
	}

	return personalityV2AnalysisResult{
		Mood:               mood,
		AffinityDelta:      affinityDelta,
		TraitDeltas:        traitDeltas,
		ProfileUpdates:     profileUpdates,
		SynthesizedEmotion: synthesizedEmotion,
		InnerThought:       extractHelperInnerThought(payload),
		NudgeCategory:      extractHelperNudgeCategory(payload),
	}, true
}

func extractHelperInnerThought(payload helperTurnPersonalityBlock) string {
	if payload.InnerVoice != nil {
		return payload.InnerVoice.InnerThought
	}
	return ""
}

func extractHelperNudgeCategory(payload helperTurnPersonalityBlock) string {
	if payload.InnerVoice != nil {
		return payload.InnerVoice.NudgeCategory
	}
	return ""
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
	consecutiveErrorCount int,
	totalErrorCount int,
	successCount int,
	isMission bool,
	isCoAgent bool,
) {
	if stm == nil {
		return
	}

	// P-10: Circuit breaker — skip V2 if too many recent failures.
	if v2CircuitOpen(logger) {
		logger.Debug("[Personality V2] Circuit breaker open, skipping analysis")
		return
	}

	contextHistory, userHistory := buildPersonalityHistories(recentMsgs, extraLabel, extraContent)
	modelName := resolvePersonalityModel(cfg)
	analyzerClient := resolvePersonalityAnalyzerClient(cfg, fallbackClient)
	helperEmotionBatchEligible, previousEmotion := resolveHelperEmotionBatchState(cfg, emotionSynthesizer)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[Personality V2] Panic in async analysis goroutine", "panic", r)
			}
		}()
		v2Ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Personality.V2TimeoutSecs)*time.Second)
		defer cancel()

		var (
			result personalityV2AnalysisResult
			err    error
		)

		taskCompleted := consecutiveErrorCount == 0 && successCount > 0
		if helperEmotionBatchEligible {
			traits, _ := stm.GetTraits()
			combinedInput := memory.EmotionInput{
				UserMessage:     triggerInfo,
				CurrentMood:     memory.MoodFocused,
				Traits:          traits,
				LastEmotion:     previousEmotion,
				ErrorCount:      consecutiveErrorCount,
				SuccessCount:    successCount,
				TimeOfDay:       memory.TimeOfDay(),
				TriggerType:     triggerType,
				TriggerDetail:   triggerDetail,
				InactivityHours: inactivityHours,
				PersonaName:     cfg.Personality.CorePersonality,
				PersonaPrompt:   prompts.GetCorePersonalityPromptSummary(cfg.Directories.PromptsDir, cfg.Personality.CorePersonality, 300),
			}
			// Enrich with inner voice context only when rate/session/trigger gates pass
			if shouldGenerateInnerVoice(cfg, consecutiveErrorCount, totalErrorCount, successCount, taskCompleted, isMission, isCoAgent) {
				// Gather lessons from error patterns
				var lessons []string
				if errPatterns, lErr := stm.GetRecentErrors(3); lErr == nil {
					for _, ep := range errPatterns {
						if ep.Resolution != "" {
							lessons = append(lessons, ep.Resolution)
						}
					}
				}
				// Estimate conversation turns from recent messages available to the analyzer
				conversationTurns := len(recentMsgs)
				// Recovery attempts = total errors that were eventually resolved
				recoveryAttempts := 0
				if totalErrorCount > 0 && consecutiveErrorCount == 0 && successCount > 0 {
					recoveryAttempts = totalErrorCount
				}
				enrichEmotionInputForInnerVoice(&combinedInput, conversationTurns, recoveryAttempts, consecutiveErrorCount, totalErrorCount, successCount, lessons)
				// Add inner voice history for narrative continuity
				if ivEntries, ivErr := stm.GetRecentInnerVoices(3); ivErr == nil && len(ivEntries) > 0 {
					combinedInput.InnerVoiceHistory = memory.FormatInnerVoiceHistory(ivEntries)
				}
			}
			result.Mood, result.AffinityDelta, result.TraitDeltas, result.ProfileUpdates, result.SynthesizedEmotion, result.InnerThought, result.NudgeCategory, err = stm.AnalyzeMoodV2WithEmotion(
				v2Ctx,
				analyzerClient,
				modelName,
				contextHistory,
				userHistory,
				meta,
				profilingEnabled,
				combinedInput,
				cfg.Agent.SystemLanguage,
			)
			if err != nil {
				logger.Debug("[Personality V2] Combined helper mood/emotion batch failed, falling back", "error", err)
			}
		}

		if !helperEmotionBatchEligible || err != nil {
			result.SynthesizedEmotion = nil
			result.Mood, result.AffinityDelta, result.TraitDeltas, result.ProfileUpdates, err = stm.AnalyzeMoodV2(v2Ctx, analyzerClient, modelName, contextHistory, userHistory, meta, profilingEnabled)
		}
		if err != nil {
			v2RecordFailure(logger)
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

		// Success — reset circuit breaker failure count.
		v2FailCount.Store(0)

		applyPersonalityV2AnalysisResult(
			cfg,
			logger,
			stm,
			emotionSynthesizer,
			previousEmotion,
			triggerInfo,
			triggerType,
			triggerDetail,
			inactivityHours,
			profilingEnabled,
			consecutiveErrorCount,
			successCount,
			result,
		)
	}()
}
