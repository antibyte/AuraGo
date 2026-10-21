package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
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
		client := llm.NewClientFromProviderWithConfig(cfg, helperCfg.ProviderType, helperCfg.BaseURL, helperCfg.APIKey, helperCfg.AccountID, helperCfg.GatewayID)
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
	accountID := ""
	gatewayID := ""
	providerType := cfg.Personality.V2ProviderType
	if provider := cfg.FindProvider(cfg.Personality.V2Provider); provider != nil {
		accountID = provider.AccountID
		gatewayID = provider.GatewayID
		providerType = provider.Type
	}
	return llm.NewClientFromProviderWithConfig(cfg, providerType, v2URL, v2Key, accountID, gatewayID)
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

func buildPersonalityHistories(_ []openai.ChatCompletionMessage, extraLabel, extraContent string) (string, string) {
	// Conversation history is never new human evidence. Only a compact, labelled
	// assistant response may accompany the separately captured current input.
	runes := []rune(extraContent)
	if len(runes) > 1200 {
		extraContent = string(runes[:1200])
	}
	if strings.TrimSpace(extraContent) == "" {
		return "", ""
	}
	return extraLabel + " (assistant context only): " + extraContent, ""
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

func personalityFailureCounts(parent context.Context, err error, calls int) bool {
	return calls > 0 && parent.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, memory.ErrSessionDeleted) && !errors.Is(err, errHelperBudgetUnavailable)
}

type personalityV2AnalysisResult struct {
	Input              *memory.HumanInput
	Basis              *memory.PersonalitySnapshot
	ObservationID      string
	Appraisal          *memory.PersonalityAppraisal
	Mood               memory.Mood
	AffinityDelta      float64
	TraitDeltas        map[string]float64
	ProfileUpdates     []memory.ProfileUpdate
	SynthesizedEmotion *memory.EmotionState
	InnerThought       string // Inner voice thought (1-3 sentences, first person)
	NudgeCategory      string // Category of the inner voice nudge
	NudgeConfidence    float64
}

// A malformed optional narration must not reject valid mood or profile fields.
func (p *helperTurnPersonalityBlock) UnmarshalJSON(data []byte) error {
	type plain helperTurnPersonalityBlock
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	emotion := fields["emotion_state"]
	delete(fields, "emotion_state")
	rest, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var result plain
	if err = json.Unmarshal(rest, &result); err != nil {
		return err
	}
	if err = json.Unmarshal(emotion, &result.EmotionState); err != nil {
		result.EmotionState = helperTurnEmotionPayload{}
	}
	*p = helperTurnPersonalityBlock(result)
	return nil
}

func resolveHelperEmotionBatchState(cfg *config.Config, emotionSynthesizer *memory.EmotionSynthesizer, bases ...*memory.PersonalitySnapshot) (bool, *memory.EmotionState) {
	helperEmotionBatchEligible := cfg.Personality.EmotionSynthesizer.Enabled && emotionSynthesizer != nil
	var previousEmotion *memory.EmotionState
	if helperEmotionBatchEligible {
		canSynthesize, lastEmotion := emotionSynthesizer.CanSynthesizeNow(time.Now())
		previousEmotion = lastEmotion
		helperEmotionBatchEligible = cfg.Personality.EmotionSynthesizer.TriggerAlways ||
			(cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange && len(bases) > 0 && bases[0] != nil && previousEmotion != nil && bases[0].Affect.Mood != previousEmotion.PrimaryMood) ||
			previousEmotion == nil
		helperEmotionBatchEligible = helperEmotionBatchEligible && canSynthesize
	}
	return helperEmotionBatchEligible, previousEmotion
}

// dampenTraitDelta reduces only updates that push a trait further toward an
// extreme. Center-bound updates are left intact so saturated traits can recover
// quickly while repeated positive helper deltas converge instead of clamping to 1.0.
func dampenTraitDelta(current, delta float64) float64 {
	if delta == 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return 0
	}
	current = math.Max(0, math.Min(1, current))
	if delta > 0 {
		if current < 0.5 {
			return delta
		}
		return delta * math.Max(0, math.Min(1, (1-current)/0.5))
	}
	if current > 0.5 {
		return delta
	}
	return delta * math.Max(0, math.Min(1, current/0.5))
}

func applyPersonalityV2AnalysisResult(
	sessionID string,
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
	contexts ...context.Context,
) {
	if stm == nil || cfg == nil || !result.Input.Valid() || result.Input.SessionID != sessionID {
		return
	}
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	apply := func() {
		if cfg.AuthorizationSnapshots != nil {
			_, current := cfg.AuthorizationSnapshots()
			if current == nil || !current.Personality.Engine || !current.Personality.EngineV2 || personalityContextID(current.Personality.CorePersonality) != personalityContextID(cfg.Personality.CorePersonality) {
				return
			}
		}
		// Runtime callers capture the snapshot before dispatching any background
		// work. The fallback is for immediate, synchronous callers only.
		basis := result.Basis
		if basis == nil {
			snapshot, err := stm.GetPersonalitySnapshotAt(time.Now())
			if err != nil {
				return
			}
			basis = &snapshot
		}
		observation := memory.PersonalityObservation{
			ID: result.ObservationID, Source: "helper", Target: "task", At: time.Now(),
			Basis: basis, Input: result.Input, Semantic: true, Human: true,
			Mood: result.Mood, TraitDeltas: result.TraitDeltas,
		}
		if appraisal := result.Appraisal; appraisal != nil && appraisal.Reference == "current_user_message" {
			observation.Signal, observation.Target, observation.Confidence = appraisal.Signal, appraisal.Target, appraisal.Confidence
			observation.AffinityDelta = result.AffinityDelta
		}
		moodChanged := previousEmotion == nil || previousEmotion.PrimaryMood != basis.Affect.Mood
		if cfg.Personality.EmotionSynthesizer.Enabled && (cfg.Personality.EmotionSynthesizer.TriggerAlways ||
			(cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange && moodChanged) || previousEmotion == nil) {
			observation.Emotion = result.SynthesizedEmotion
		}
		var acceptedThought, acceptedCategory string
		var acceptedConfidence float64
		if observation.Emotion != nil && cfg.Personality.InnerVoice.Enabled && result.InnerThought != "" {
			thought, category, confidence, accepted, _ := normalizeInnerVoice(sessionID, result.InnerThought, result.NudgeCategory, result.NudgeConfidence)
			if accepted {
				acceptedThought, acceptedCategory, acceptedConfidence = thought, category, confidence
				observation.InnerThought, observation.NudgeCategory = thought, category
			}
		}
		if profilingEnabled {
			observation.ProfileUpdates = result.ProfileUpdates
		}
		_, narrated, err := emotionSynthesizer.ApplyObservation(stm, observation)
		if err != nil {
			logger.Debug("[Personality V2] Observation not applied", "error", err)
			return
		}
		if narrated && acceptedThought != "" {
			applyInnerVoiceResult(sessionID, acceptedThought, acceptedCategory, acceptedConfidence)
		}
	}
	_ = stm.WithSessionWrite(ctx, sessionID, func() error { apply(); return nil })

}

func normalizeHelperTurnPersonalityResult(payload helperTurnPersonalityBlock, meta memory.PersonalityMeta) (personalityV2AnalysisResult, bool) {
	if strings.TrimSpace(payload.MoodAnalysis.UserSentiment) == "" {
		payload.MoodAnalysis.UserSentiment = "neutral"
	}
	if strings.TrimSpace(payload.MoodAnalysis.AgentMood) == "" {
		payload.MoodAnalysis.AgentMood = string(memory.MoodFocused)
	}

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
		Appraisal:          payload.Appraisal,
		AffinityDelta:      affinityDelta,
		TraitDeltas:        traitDeltas,
		ProfileUpdates:     profileUpdates,
		SynthesizedEmotion: synthesizedEmotion,
		InnerThought: func() string {
			if synthesizedEmotion == nil {
				return ""
			}
			return extractHelperInnerThought(payload)
		}(),
		NudgeCategory:   extractHelperNudgeCategory(payload),
		NudgeConfidence: extractHelperNudgeConfidence(payload),
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

func extractHelperNudgeConfidence(payload helperTurnPersonalityBlock) float64 {
	if payload.InnerVoice != nil {
		return payload.InnerVoice.Confidence
	}
	return 0
}

func launchAsyncPersonalityV2Analysis(
	sessionID string,
	cfg *config.Config,
	logger *slog.Logger,
	fallbackClient memory.PersonalityAnalyzerClient,
	runCfg RunConfig,
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
	basisSnapshots ...*memory.PersonalitySnapshot,
) {
	if stm == nil || cfg == nil || !runCfg.HumanInput.Valid() || runCfg.HumanInput.SessionID != sessionID || strings.TrimSpace(runCfg.HumanInput.Text) == "" || isMission || isCoAgent || runCfg.BudgetTracker == nil || runCfg.BudgetTracker.IsBlocked("personality") {
		return
	}

	// P-10: Circuit breaker — skip V2 if too many recent failures.
	if v2CircuitOpen(logger) {
		logger.Debug("[Personality V2] Circuit breaker open, skipping analysis")
		return
	}

	var basis *memory.PersonalitySnapshot
	if len(basisSnapshots) > 0 {
		basis = basisSnapshots[0]
	} else {
		snapshot, err := stm.GetPersonalitySnapshotAt(time.Now())
		if err == nil {
			basis = &snapshot
		}
	}
	if basis == nil {
		return
	}
	contextHistory, userHistory := buildPersonalityHistories(recentMsgs, extraLabel, extraContent)
	triggerInfo = runCfg.HumanInput.Text
	userHistory = triggerInfo
	modelName := resolvePersonalityModel(cfg)
	helper := llm.ResolveHelperLLM(cfg)
	mainFallback := !(helper.Enabled && helper.Model != "") && cfg.Personality.V2ResolvedURL == "" && cfg.Personality.V2URL == ""
	if mainFallback {
		modelName = cfg.LLM.Model
	}
	analyzerClient := &emotionCompletionClient{client: resolvePersonalityAnalyzerClient(cfg, fallbackClient), cfg: cfg, tracker: runCfg.BudgetTracker, mainFallback: mainFallback}
	timeout := time.Duration(cfg.Personality.V2TimeoutSecs) * time.Second
	if timeout <= 0 || timeout > 60*time.Second {
		timeout = 60 * time.Second
	}
	if mainFallback {
		timeout = min(timeout, 20*time.Second)
		logger.Warn("[Personality V2] Using bounded main-model fallback", "model", modelName, "max_requests", 1, "max_output_tokens", llm.ReasoningOutputTokens, "timeout", timeout, "budget_category", "personality")
	}
	helperEmotionBatchEligible, previousEmotion := resolveHelperEmotionBatchState(cfg, emotionSynthesizer, basis)

	sideEffectsFromRunConfig(runCfg).Go(func(parentCtx context.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[Personality V2] Panic in async analysis goroutine", "panic", r)
			}
		}()
		sessionCtx, release, sessionErr := stm.RegisterSessionRun(parentCtx, sessionID)
		if sessionErr != nil {
			return
		}
		defer release()
		v2Ctx, cancel := context.WithTimeout(sessionCtx, timeout)
		defer cancel()

		var (
			result personalityV2AnalysisResult
			err    error
		)

		taskCompleted := consecutiveErrorCount == 0 && successCount > 0
		if helperEmotionBatchEligible {
			traits := basis.Traits
			combinedInput := memory.EmotionInput{
				Snapshot: basis, ObservationID: basis.TurnID,
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
				PersonaName:     effectivePersonalityID(cfg.Personality.CorePersonality),
				PersonaPrompt:   prompts.GetCorePersonalityPromptSummary(cfg.Directories.PromptsDir, effectivePersonalityID(cfg.Personality.CorePersonality), 300),
			}
			// Enrich with inner voice context only when rate/session/trigger gates pass
			if shouldGenerateInnerVoice(sessionID, cfg, consecutiveErrorCount, totalErrorCount, successCount, taskCompleted, isMission, isCoAgent) {
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
				enrichEmotionInputForInnerVoice(&combinedInput, conversationTurns, recoveryAttempts, consecutiveErrorCount, totalErrorCount, successCount, lessons, nil)
				// Add inner voice history for narrative continuity
				if ivEntries, ivErr := stm.GetRecentInnerVoices(3); ivErr == nil && len(ivEntries) > 0 {
					combinedInput.InnerVoiceHistory = memory.FormatInnerVoiceHistory(ivEntries)
				}
			}
			result.Mood, result.AffinityDelta, result.TraitDeltas, result.ProfileUpdates, result.SynthesizedEmotion, result.InnerThought, result.NudgeCategory, result.NudgeConfidence, err = stm.AnalyzeMoodV2WithEmotion(
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

		if !helperEmotionBatchEligible || (err != nil && !mainFallback && v2Ctx.Err() == nil) {
			result.SynthesizedEmotion = nil
			observationContext := &memory.MoodAnalysisContext{Snapshot: basis, CurrentUserMessage: triggerInfo}
			result.Mood, result.AffinityDelta, result.TraitDeltas, result.ProfileUpdates, err = stm.AnalyzeMoodV2(v2Ctx, analyzerClient, modelName, contextHistory, userHistory, meta, profilingEnabled, observationContext)
			result.Appraisal = observationContext.Appraisal
		}
		if err != nil {
			if !personalityFailureCounts(sessionCtx, err, analyzerClient.calls) {
				return
			}
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
		result.Basis, result.ObservationID, result.Input = basis, runCfg.HumanInput.TurnID, runCfg.HumanInput
		if result.SynthesizedEmotion != nil {
			result.Appraisal = result.SynthesizedEmotion.Appraisal
		}
		v2FailCount.Store(0)

		applyPersonalityV2AnalysisResult(
			sessionID,
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
			result, v2Ctx,
		)
	})
}
