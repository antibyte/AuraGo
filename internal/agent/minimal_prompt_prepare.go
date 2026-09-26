package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

type minimalPromptPreparation struct {
	Budget          *RequestBudget
	BuilderRevision string
	ProviderType    string
	Usage           []RequestTokenUsage
}

func prepareMinimalLoopRequest(ctx context.Context, cfg *config.Config, client llm.ChatClient, req *openai.ChatCompletionRequest, baseSystemPrompt string, guardian *security.Guardian, logger *slog.Logger, tokenCache *tokenCountCache, toolCallCount int, addenda ...prompts.PromptAddendum) (minimalPromptPreparation, error) {
	return prepareMinimalLoopRequestWithReasoning(ctx, cfg, client, req, baseSystemPrompt, guardian, logger, tokenCache, toolCallCount, false, addenda...)
}

func prepareMinimalLoopRequestWithReasoning(ctx context.Context, cfg *config.Config, client llm.ChatClient, req *openai.ChatCompletionRequest, baseSystemPrompt string, guardian *security.Guardian, logger *slog.Logger, tokenCache *tokenCountCache, toolCallCount int, preserveReasoning bool, addenda ...prompts.PromptAddendum) (minimalPromptPreparation, error) {
	return prepareMinimalLoopRequestWithProfile(ctx, cfg, client, req, baseSystemPrompt, guardian, logger, tokenCache, toolCallCount, preserveReasoning, nil, addenda...)
}

func prepareMinimalLoopRequestWithProfile(ctx context.Context, cfg *config.Config, client llm.ChatClient, req *openai.ChatCompletionRequest, baseSystemPrompt string, guardian *security.Guardian, logger *slog.Logger, tokenCache *tokenCountCache, toolCallCount int, preserveReasoning bool, profile *PreparedPromptProfile, addenda ...prompts.PromptAddendum) (minimalPromptPreparation, error) {
	if req == nil {
		return minimalPromptPreparation{}, fmt.Errorf("chat completion request is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if tokenCache == nil {
		tokenCache = newTokenCountCache(256)
	}

	nonSystemMessages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for i, message := range req.Messages {
		if message.Role == openai.ChatMessageRoleSystem && (profile == nil || i == 0) {
			continue
		}
		nonSystemMessages = append(nonSystemMessages, message)
	}
	budgetReq := *req
	budgetReq.Messages = nonSystemMessages
	if profile != nil {
		budgetReq.Messages = minimumPreparedMessages(nonSystemMessages)
	}

	requiredTools := make(map[string]bool, len(req.Tools))
	for _, tool := range req.Tools {
		if tool.Function == nil || strings.TrimSpace(tool.Function.Name) == "" {
			return minimalPromptPreparation{}, fmt.Errorf("minimal loop tool schema has no function name")
		}
		requiredTools[strings.TrimSpace(tool.Function.Name)] = true
	}
	budget, fittedTools, droppedTools, err := prepareRequestBudgetAndTools(ctx, cfg, client, budgetReq, requiredTools, tokenCache, logger)
	if err != nil {
		return minimalPromptPreparation{}, err
	}
	if len(droppedTools) > 0 {
		return minimalPromptPreparation{}, fmt.Errorf("required minimal loop tool schemas were dropped")
	}
	req.Tools = fittedTools
	// Send the same output ceiling that was reserved for every eligible route.
	// Leaving it unset lets providers generate beyond the validated context budget.
	req.MaxTokens = budget.CompletionReserve

	buildStarted := time.Now()
	promptResult := prompts.PromptBuildResult{Revision: prompts.PromptRevision("")}
	if strings.TrimSpace(baseSystemPrompt) != "" {
		systemBudgetMessages := nonSystemMessages
		if profile != nil {
			systemBudgetMessages = minimumPreparedMessages(nonSystemMessages)
		}
		systemBudget, err := budget.systemPromptBudget(systemBudgetMessages, req.Tools, req.Model, tokenCache)
		if err != nil {
			return minimalPromptPreparation{}, err
		}
		if profile != nil {
			if len(addenda) != 0 {
				return minimalPromptPreparation{}, fmt.Errorf("prepared prompt cannot accept dynamic addenda")
			}
			promptResult = prompts.PromptBuildResult{Text: profile.system, Tokens: tokenCache.Count(profile.system, req.Model), Revision: profile.revision}
			if promptResult.Tokens > systemBudget {
				return minimalPromptPreparation{}, promptBudgetExceededForRoutes(budget, promptResult.Tokens)
			}
		} else {
			promptResult, err = prompts.FitSystemPromptToBudget(ctx, prompts.PromptFitRequest{
				Text: baseSystemPrompt, Tokens: -1, Model: req.Model, TokenBudget: systemBudget,
				Addenda: addenda,
			}, logger)
		}
		prompts.RecordPromptFit(prompts.PromptFitRecord{
			Timestamp:       time.Now(),
			InputChars:      promptResult.InputChars,
			OutputChars:     len(promptResult.Text),
			InputTokens:     promptResult.InputTokens,
			OutputTokens:    promptResult.Tokens,
			TokenBudget:     systemBudget,
			RemovedSections: promptResult.RemovedSections,
			BudgetExceeded:  promptResult.BudgetExceeded != nil,
		})
		if err != nil {
			return minimalPromptPreparation{}, fmt.Errorf("fit minimal system prompt: %w", err)
		}
		if promptResult.BudgetExceeded != nil {
			return minimalPromptPreparation{}, promptBudgetExceededForRoutes(budget, promptResult.BudgetExceeded.RequiredTokens)
		}
		req.Messages = append([]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: promptResult.Text}}, nonSystemMessages...)
	} else {
		req.Messages = nonSystemMessages
	}

	applyPromptSecurityToRequest(req, cfg, guardian, promptResult.Text, logger)
	before := len(req.Messages)
	sanitized, droppedOrphans := SanitizeToolMessages(req.Messages)
	req.Messages = sanitized
	if droppedOrphans > 0 {
		logger.Warn("[PreSend] Sanitized orphaned tool messages before context trimming",
			"dropped", droppedOrphans, "before", before, "after", len(sanitized))
	}
	currentUserText := ""
	if index := latestGenuineUserIndex(req.Messages); index >= 0 {
		currentUserText = messageText(req.Messages[index])
	}
	if profile != nil {
		req.Messages, err = trimPreparedHistory(budget, req.Messages, currentUserText, promptResult.Text, req.Tools, tokenCache)
		if err != nil {
			return minimalPromptPreparation{}, err
		}
	}
	workingMessages, workingDropped, workingStats := budget.trimHistoryWorkingSet(req.Messages, currentUserText, promptResult.Text, req.Tools, tokenCache)
	req.Messages = appendRecapWithinWorkingSet(budget, workingMessages, currentUserText, promptResult.Text, req.Tools, workingDropped, tokenCache)
	trimmed, droppedHistory, err := budget.trimHistory(req.Messages, req.Tools, false, logger, tokenCache)
	if err != nil {
		return minimalPromptPreparation{}, err
	}
	req.Messages = trimmed
	if len(droppedHistory) > 0 {
		logger.Info("[ContextGuard] Minimal loop history trimmed",
			"remaining_messages", len(req.Messages), "dropped_messages", len(droppedHistory))
	}

	providerType := ""
	if len(budget.Routes) > 0 {
		providerType = budget.Routes[0].Limits.Route.ProviderType
	}
	finalized, err := finalizePromptRequestForSend(req, budget, tokenCache, providerType, logger, preserveReasoning)
	if err != nil {
		return minimalPromptPreparation{}, err
	}
	for _, usage := range finalized.Usage {
		logger.Info("[PromptBudget] Minimal request ready",
			"prompt_build_ms", time.Since(buildStarted).Milliseconds(),
			"builder_prompt_revision", promptResult.Revision,
			"request_system_revision", promptMessagesRevision(req.Messages),
			"probe_cache_hit", usage.ProbeCacheHit,
			"provider", usage.ProviderType,
			"model", usage.Model,
			"context_window", usage.ContextWindow,
			"context_source", usage.ContextSource,
			"output_source", usage.OutputSource,
			"system_tokens", usage.SystemTokens,
			"schema_tokens", usage.SchemaTokens,
			"history_tokens", usage.HistoryTokens,
			"history_working_limit_tokens", workingStats.LimitTokens,
			"current_request_tokens", workingStats.CurrentTokens,
			"kept_history_tokens", budget.maxCarriedHistoryTokens(req.Messages, currentUserMessageIndex(req.Messages, currentUserText), tokenCache),
			"summary_tokens", budget.maxSummaryTokens(req.Messages, tokenCache),
			"output_tokens", usage.CompletionTokens,
			"safety_tokens", usage.SafetyTokens,
			"total_tokens", usage.TotalTokens)
	}
	appendPreparedPromptLog(cfg, *req, providerType, promptResult.Revision, toolRecoveryState{}, 0, toolCallCount, logger)
	return minimalPromptPreparation{
		Budget: budget, BuilderRevision: promptResult.Revision, ProviderType: providerType, Usage: finalized.Usage,
	}, nil
}

func promptBudgetExceededForRoutes(budget *RequestBudget, requiredTokens int) error {
	if budget == nil || len(budget.Routes) == 0 {
		return &prompts.PromptBudgetExceededError{RequiredTokens: requiredTokens}
	}
	limiting := budget.Routes[0].Limits
	for _, route := range budget.Routes[1:] {
		if budget.inputLimit(route.Limits) < budget.inputLimit(limiting) {
			limiting = route.Limits
		}
	}
	return budget.exceededError(limiting, requiredTokens)
}
