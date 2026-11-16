package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func completionConfigError(format string, args ...any) error {
	return &LLMError{Category: ErrCategoryNonRetryableConfig, Message: fmt.Sprintf(format, args...)}
}

func completionRoute(cfg *config.Config, route ModelRoute) ModelRoute {
	if cfg != nil && strings.EqualFold(route.ProviderType, "anthropic") {
		route.thinking = &anthropicThinkingConfig{
			Enabled: cfg.LLM.AnthropicThinking.Enabled, BudgetTokens: cfg.LLM.AnthropicThinking.BudgetTokens,
			ModelAllowlist: append([]string(nil), cfg.LLM.AnthropicThinking.ModelAllowlist...),
		}
	}
	return route
}

type completionRouteKey struct{}

// WithCompletionRoute binds single-provider callers; explicit physical routes win.
func WithCompletionRoute(ctx context.Context, route ModelRoute) context.Context {
	route.APIKey = ""
	return context.WithValue(ctx, completionRouteKey{}, route)
}

// ValidateCompletionLimits preserves invalid-input detection before budgeting.
func ValidateCompletionLimits(request openai.ChatCompletionRequest) error {
	if request.MaxTokens < 0 || request.MaxCompletionTokens < 0 ||
		request.MaxTokens > 0 && request.MaxCompletionTokens > 0 && request.MaxTokens != request.MaxCompletionTokens {
		return completionConfigError("invalid or conflicting completion token limits")
	}
	return nil
}

// prepareCompletionRequest runs after routing, before the SDK's local validator.
func prepareCompletionRequest(route ModelRoute, request openai.ChatCompletionRequest) (openai.ChatCompletionRequest, bool, error) {
	if err := ValidateCompletionLimits(request); err != nil {
		return request, false, err
	}
	provider := strings.ToLower(strings.TrimSpace(route.ProviderType))
	if provider == "copilot" {
		request.Model = strings.TrimPrefix(request.Model, "copilot/")
	}
	model := request.Model
	if provider == "" || provider == "openai" || provider == "copilot" || provider == "openrouter" {
		model = strings.TrimPrefix(model, "openai/")
	}
	check := request
	check.Model, check.MaxTokens = model, 1
	reasoningModel := errors.Is(openai.NewReasoningValidator().Validate(check), openai.ErrReasoningModelMaxTokensDeprecated)
	gpt6 := model == openai.GPT6Astra || model == openai.GPT6Dot1Sol || model == openai.GPT6Sol || model == openai.GPT6Luna || model == "gpt-6.1-terra"
	if reasoningModel || gpt6 {
		if request.MaxTokens > 0 {
			request.MaxCompletionTokens = request.MaxTokens
		}
		request.MaxTokens = 0
		if request.ReasoningEffort != openai.ReasoningEffortNone {
			request.Temperature, request.TopP = 0, 0
			request.LogProbs, request.TopLogProbs = false, 0
		}
	}
	check = request
	check.Model = model
	err := openai.NewReasoningValidator().Validate(check)
	hasTools := len(request.Tools) > 0 || len(request.Functions) > 0 || request.ToolChoice != nil && request.ToolChoice != "none" || request.FunctionCall != nil && request.FunctionCall != "none"
	responses := (provider == "copilot" || provider == "openai" || provider == "openrouter") && CopilotResponsesOnlyRe.MatchString(model) ||
		errors.Is(err, openai.ErrReasoningToolsRequireResponses) ||
		gpt6 && hasTools && (request.ReasoningEffort != openai.ReasoningEffortNone || model == "gpt-6.1-terra")
	if responses {
		if request.ReasoningEffort != openai.ReasoningEffortNone {
			request.Temperature, request.TopP = 0, 0
			request.LogProbs, request.TopLogProbs = false, 0
		}
		if provider != "" && provider != "openai" && provider != "copilot" && provider != "openrouter" {
			return request, false, completionConfigError("selected model requires the Responses API; provider %q has no supported Responses route", provider)
		}
		if err != nil && !errors.Is(err, openai.ErrReasoningToolsRequireResponses) {
			return request, false, completionConfigError("invalid reasoning request: %s", err)
		}
	} else if err != nil {
		return request, false, completionConfigError("invalid reasoning request: %s", err)
	}
	copied := false
	for i, message := range request.Messages {
		if !responses && strings.HasPrefix(message.ReasoningContent, responsesReasoningPrefix) ||
			provider != "" && provider != "anthropic" && strings.HasPrefix(message.ReasoningContent, anthropicReasoningPrefix) {
			if !copied {
				request.Messages = append([]openai.ChatCompletionMessage(nil), request.Messages...)
				copied = true
			}
			request.Messages[i].ReasoningContent = ""
		}
	}
	if route.thinking != nil {
		features := anthropicFeaturesForModel(request.Model)
		if features.defaultThinking && !features.disableThinking && !route.thinking.enabledForModel(request.Model) {
			return request, false, completionConfigError("selected Anthropic model requires enabled thinking and a matching model allowlist")
		}
		if request.ToolChoice != nil {
			choice, err := translateToolChoice(request.ToolChoice)
			if err != nil {
				return request, false, completionConfigError("invalid Anthropic tool choice: %s", err)
			}
			if choice != nil && (choice.Type == "any" || choice.Type == "tool") && features.forbidForcedTools {
				return request, false, completionConfigError("selected Anthropic model does not support forced tool_choice")
			}
		}
	}
	return request, responses, nil
}
