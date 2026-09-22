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
func prepareTurnEmotion(ctx context.Context, runCfg RunConfig, flags prompts.ContextFlags, userMessage string, meta memory.PersonalityMeta, logger *slog.Logger) {
	cfg, stm := runCfg.Config, runCfg.ShortTermMem
	if cfg == nil || stm == nil || !cfg.Personality.Engine || strings.TrimSpace(userMessage) == "" ||
		!shouldRunTurnSideEffects(runCfg, runCfg.SessionID, flags) {
		return
	}
	if ctx.Err() != nil {
		return
	}
	mood, _ := memory.DetectMood(userMessage, "", meta)
	// Focused is also the heuristic's generic/positive-feedback fallback. It
	// must not flatten a more expressive affect state or a previous observation.
	if mood != memory.MoodFocused {
		if err := stm.ApplyMoodSuggestion(mood, time.Now()); err != nil {
			logger.Debug("[Personality] Current-turn mood unavailable", "error", err)
		}
	}
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
