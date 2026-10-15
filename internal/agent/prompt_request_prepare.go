package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	loggerPkg "aurago/internal/logger"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

type promptRequestFinalization struct {
	Usage               []RequestTokenUsage
	DroppedToolMessages int
}

func applyPromptSecurityToRequest(req *openai.ChatCompletionRequest, cfg *config.Config, guardian *security.Guardian, _ string, logger *slog.Logger) {
	if req == nil || cfg == nil || guardian == nil || !cfg.Guardian.PromptSec.UseSanitizedOutput {
		return
	}
	if updatedMessages, applied := applyPromptSecToLatestUserMessage(req.Messages, guardian); applied {
		req.Messages = updatedMessages
		if logger != nil {
			logger.Debug("[Guardian] Applied promptsec sanitized user message",
				"use_sanitized_output", true)
		}
	}
}

func finalizePromptRequestForSend(req *openai.ChatCompletionRequest, budget *RequestBudget, cache *tokenCountCache, providerType string, logger *slog.Logger, preserveReasoning ...bool) (promptRequestFinalization, error) {
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
	// The sanitizer copies matched calls, so wire normalization preserves the archive.
	for i := range sanitized {
		if sanitized[i].Role != openai.ChatMessageRoleAssistant {
			continue
		}
		for j := range sanitized[i].ToolCalls {
			call := &sanitized[i].ToolCalls[j]
			call.Index = nil // Stream positions are not request fields.
			arguments := strings.TrimSpace(call.Function.Arguments)
			// Providers parse historical arguments too; rejected calls must not poison recovery.
			if !json.Valid([]byte(arguments)) || arguments[0] != '{' {
				call.Function.Arguments = "{}"
			}
		}
	}
	req.Messages = sanitized
	if len(preserveReasoning) == 0 || !preserveReasoning[0] {
		req.Messages = sanitizeReasoningForRequestRoutes(sanitized, budget.Routes, providerType, req.Model)
	}
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
