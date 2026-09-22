package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

// prepareTurnEmotion runs once before the first chat completion. Local mood
// detection is immediate; semantic synthesis and slow trait/profile learning
// stay in the existing background helper batch without delaying the reply.
func prepareTurnEmotion(ctx context.Context, runCfg RunConfig, flags prompts.ContextFlags, userMessage string, meta memory.PersonalityMeta, logger *slog.Logger) *memory.PersonalitySnapshot {
	cfg, stm := runCfg.Config, runCfg.ShortTermMem
	if cfg == nil || stm == nil || !cfg.Personality.Engine || strings.TrimSpace(userMessage) == "" ||
		!shouldRunTurnSideEffects(runCfg, runCfg.SessionID, flags) {
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}
	persona := personalityContextID(cfg.Personality.CorePersonality)
	if cfg.AuthorizationSnapshots != nil {
		_, live := cfg.AuthorizationSnapshots()
		if live == nil || !live.Personality.Engine || personalityContextID(live.Personality.CorePersonality) != persona {
			return nil
		}
	} else {
		if err := stm.SetPersonalityContext(persona, meta); err != nil {
			logger.Debug("[Personality] Persona context unavailable", "error", err)
			return nil
		}
	}
	basis, err := stm.GetPersonalitySnapshotAt(time.Now())
	if err != nil || basis.Persona != persona {
		return nil
	}
	mood, deltas := memory.DetectMood(userMessage, "", meta)
	if mood == memory.MoodFocused {
		mood = ""
	}
	if cfg.Personality.EngineV2 {
		deltas = nil
	}
	trigger := memory.ClassifyConversationEmotionTrigger(userMessage)
	event, _ := memory.AffectEventForTrigger(trigger, "", "chat")
	appraisal := localPersonalityAppraisal(userMessage)
	affinityDelta := 0.0
	switch appraisal.Signal {
	case "praise", "repair":
		affinityDelta = 0.03 * meta.EmpathyBias
	case "criticism":
		affinityDelta = -0.015 * meta.EmpathyBias
	}
	snapshot, err := stm.ApplyPersonalityObservation(memory.PersonalityObservation{
		ID: runCfg.DiscoveryRunID, Source: "chat", Target: appraisal.Target,
		Confidence: appraisal.Confidence, Human: true, BeginTurn: true, At: time.Now(),
		Event: &event, Mood: mood, TraitDeltas: deltas, Signal: appraisal.Signal,
		AffinityDelta: affinityDelta, Explicit: appraisal.Signal == "criticism", Basis: &basis,
	})
	if err != nil {
		logger.Debug("[Personality] Current-turn observation unavailable", "error", err)
		return nil
	}
	return &snapshot
}

// Empty legacy prompt profiles use the neutral persistent context.
func personalityContextID(value string) string {
	id := effectivePersonalityID(value)
	if id == "" {
		return "neutral"
	}
	return id
}

// Only unambiguous, complete feedback phrases establish a local relationship
// signal. General frustration, quoted feedback and uncertain irony do not.
func localPersonalityAppraisal(message string) memory.PersonalityAppraisal {
	text := strings.Trim(strings.ToLower(strings.TrimSpace(message)), ".! ")
	signal := ""
	switch text {
	case "danke", "vielen dank", "danke dir", "gut gemacht", "das hast du gut gemacht", "thank you", "thanks", "well done", "good job":
		signal = "praise"
	case "deine antwort ist falsch", "du hast mich falsch verstanden", "your answer is wrong", "you misunderstood me":
		signal = "criticism"
	case "jetzt stimmt es, danke", "jetzt passt es, danke", "danke für die korrektur", "thanks for fixing it", "that fixes it, thanks":
		signal = "repair"
	}
	if signal == "" {
		return memory.PersonalityAppraisal{}
	}
	return memory.PersonalityAppraisal{Signal: signal, Target: "agent", Confidence: 0.95}
}

// emotionCompletionClient gives short emotion requests the same route/output
// limits and reasoning allowance as the other JSON helper requests.
type emotionCompletionClient struct {
	client memory.PersonalityAnalyzerClient
	cfg    *config.Config
}

func (c *emotionCompletionClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	helper := llm.ResolveHelperLLM(c.cfg)
	route := llm.ModelRoute{Model: req.Model, ProviderID: c.cfg.LLM.Provider, ProviderType: c.cfg.LLM.ProviderType, BaseURL: c.cfg.LLM.BaseURL}
	if helper.Enabled && helper.Model != "" {
		route.ProviderID, route.ProviderType, route.BaseURL = helper.ProviderID, helper.ProviderType, helper.BaseURL
	} else if c.cfg.Personality.V2Provider != "" || c.cfg.Personality.V2ResolvedURL != "" || c.cfg.Personality.V2URL != "" {
		route.ProviderID, route.ProviderType, route.BaseURL = c.cfg.Personality.V2Provider, c.cfg.Personality.V2ProviderType, c.cfg.Personality.V2ResolvedURL
		if route.BaseURL == "" {
			route.BaseURL = c.cfg.Personality.V2URL
		}
	}
	if provider := c.cfg.FindProvider(route.ProviderID); provider != nil {
		route.ContextWindowOverride, route.MaxOutputTokensOverride = provider.ContextWindow, provider.MaxOutputTokens
	}
	inputTokens := 32
	for _, message := range req.Messages {
		inputTokens += prompts.CountTokensForModel(message.Content, req.Model)
	}
	limits := llm.ResolveModelLimitsCached(route, c.cfg.Agent.ContextWindow)
	outputTokens, err := llm.JSONCompletionOutputBudget(limits, 512, inputTokens)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	req.MaxTokens = outputTokens
	if caps, ok := llm.CapabilitiesFromRegistry(route.ProviderType, req.Model); ok {
		req.ResponseFormat = llm.JSONResponseFormat(caps.StructuredOutputs)
	}
	response, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return response, err
	}
	content, err := llm.JSONContentFromResponse(response)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	response.Choices[0].Message.Content = content
	return response, nil
}
