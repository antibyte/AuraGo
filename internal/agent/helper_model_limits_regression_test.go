package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestHelperModelOverrideDoesNotInheritAnotherModelsLimits(t *testing.T) {
	ResetGlobalHelperLLMManager()
	t.Cleanup(ResetGlobalHelperLLMManager)
	cfg := &config.Config{Providers: []config.ProviderEntry{{ID: "helper", Type: "openai", BaseURL: "https://fixture.invalid/v1", Model: "default-model", ContextWindow: 64000, MaxOutputTokens: 8000}}}
	cfg.Agent.ContextWindow = 12000
	cfg.LLM.HelperEnabled, cfg.LLM.HelperProvider, cfg.LLM.HelperProviderType = true, "helper", "openai"
	cfg.LLM.HelperBaseURL, cfg.LLM.HelperResolvedModel = "https://fixture.invalid/v1", "default-model"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	matching := getOrCreateHelperLLMManager(cfg, logger)
	if matching == nil || matching.route.ContextWindowOverride != 64000 || matching.route.MaxOutputTokensOverride != 8000 {
		t.Fatal("matching default model lost its explicit limits")
	}
	cfg.LLM.HelperResolvedModel = "different-model"
	overridden := getOrCreateHelperLLMManager(cfg, logger)
	if overridden == nil || overridden == matching || overridden.route.ContextWindowOverride != 0 || overridden.route.MaxOutputTokensOverride != 0 || overridden.contextWindow != 12000 {
		t.Fatal("model override retained stale model limits or lost the global cap")
	}
	if getOrCreateHelperLLMManager(cfg, logger) != overridden {
		t.Fatal("unchanged helper configuration invalidated its cache")
	}
}

func TestEmotionHelperModelOverrideUsesItsOwnOutputLimit(t *testing.T) {
	for _, model := range []string{"default-model", "override-model"} {
		t.Run(model, func(t *testing.T) {
			cfg := &config.Config{Providers: []config.ProviderEntry{{ID: "helper", Type: "openai", BaseURL: "https://fixture.invalid/v1", Model: "default-model", ContextWindow: 64000, MaxOutputTokens: 320}}}
			cfg.LLM.HelperEnabled, cfg.LLM.HelperProvider, cfg.LLM.HelperProviderType = true, "helper", "openai"
			cfg.LLM.HelperBaseURL, cfg.LLM.HelperResolvedModel = "https://fixture.invalid/v1", model
			client := &turnEmotionClient{complete: func(context.Context) (openai.ChatCompletionResponse, error) {
				return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{}`}, FinishReason: openai.FinishReasonStop}}}, nil
			}}
			adapter := &emotionCompletionClient{cfg: cfg, client: client}
			_, err := adapter.CreateChatCompletion(t.Context(), openai.ChatCompletionRequest{Model: model, MaxTokens: 1200, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Return JSON."}}})
			want := 1200
			if model == "default-model" {
				want = 320
			}
			if err != nil || client.request.MaxTokens != want {
				t.Fatalf("output=%d want=%d err=%v", client.request.MaxTokens, want, err)
			}
		})
	}
}
