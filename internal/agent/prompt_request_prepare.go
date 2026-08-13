package agent

import (
	"fmt"
	"log/slog"

	"aurago/internal/config"
	loggerPkg "aurago/internal/logger"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

type promptRequestFinalization struct {
	Usage               []RequestTokenUsage
	DroppedToolMessages int
}

func applyPromptSecurityToRequest(req *openai.ChatCompletionRequest, cfg *config.Config, guardian *security.Guardian, systemPrompt string, logger *slog.Logger) {
	if req == nil || cfg == nil || guardian == nil {
		return
	}
	structureEnabled := cfg.Guardian.PromptSec.Structure.Enabled
	requestGuardian := guardian
	if structureEnabled {
		requestGuardian = guardian.WithSystemPrompt(systemPrompt)
	}
	if !cfg.Guardian.PromptSec.UseSanitizedOutput && !structureEnabled {
		return
	}
	if updatedMessages, applied := applyPromptSecToLatestUserMessage(req.Messages, requestGuardian); applied {
		req.Messages = updatedMessages
		if logger != nil {
			logger.Debug("[Guardian] Applied promptsec sanitized user message",
				"structure", structureEnabled,
				"use_sanitized_output", cfg.Guardian.PromptSec.UseSanitizedOutput)
		}
	}
}

func finalizePromptRequestForSend(req *openai.ChatCompletionRequest, budget *RequestBudget, cache *tokenCountCache, providerType string, logger *slog.Logger) (promptRequestFinalization, error) {
	if req == nil {
		return promptRequestFinalization{}, fmt.Errorf("chat completion request is required")
	}
	if budget == nil {
		return promptRequestFinalization{}, fmt.Errorf("request budget is required")
	}
	if cache == nil {
		cache = newTokenCountCache(128)
	}
	if logger == nil {
		logger = slog.Default()
	}
	if len(req.Tools) == 0 && (req.ToolChoice != nil || req.ParallelToolCalls != nil) {
		hadToolChoice := req.ToolChoice != nil
		hadParallelToolCalls := req.ParallelToolCalls != nil
		req.ToolChoice = nil
		req.ParallelToolCalls = nil
		logger.Debug("[PreSend] Removed tool request options because no tool schemas remain",
			"tool_choice_removed", hadToolChoice,
			"parallel_tool_calls_removed", hadParallelToolCalls)
	}

	before := len(req.Messages)
	sanitized, dropped := SanitizeToolMessages(req.Messages)
	req.Messages = sanitizeReasoningForRequestRoutes(sanitized, budget.Routes, providerType, req.Model)
	if dropped > 0 {
		logger.Warn("[PreSend] Sanitized orphaned tool messages before LLM call",
			"dropped", dropped, "before", before, "after", len(req.Messages))
	}
	usage, err := budget.validate(req.Messages, req.Tools, cache)
	if err != nil {
		return promptRequestFinalization{Usage: usage, DroppedToolMessages: dropped}, err
	}
	return promptRequestFinalization{Usage: usage, DroppedToolMessages: dropped}, nil
}

func appendPreparedPromptLog(cfg *config.Config, req openai.ChatCompletionRequest, providerType, builderRevision string, recovery toolRecoveryState, retry422Count, toolCallCount int, logger *slog.Logger) {
	if cfg == nil || !cfg.Logging.EnablePromptLog {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	entry := newPromptLogEntry(req, providerType, builderRevision, recovery, retry422Count, toolCallCount)
	if err := loggerPkg.AppendPromptLogEntry(cfg.Logging.LogDir, entry); err != nil {
		logger.Warn("[PromptLog] Failed to write entry", "error", err)
	}
}
