package agent

import (
	"context"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

var loopbackLimiter = make(chan struct{}, 8)

func shouldPersistLoopbackHistory(sessionID string) bool {
	return sessionID == "default"
}

func isAutonomousLoopback(runCfg RunConfig, sessionID string) bool {
	switch runCfg.MessageSource {
	case "heartbeat":
		return true
	}
	return sessionID == "heartbeat"
}

func buildLoopbackConversationMessages(base []openai.ChatCompletionMessage, historyManager *memory.HistoryManager, safeMessage string, includeGlobalHistory bool) []openai.ChatCompletionMessage {
	finalMessages := append([]openai.ChatCompletionMessage(nil), base...)
	currentMsg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: safeMessage}
	if !includeGlobalHistory || historyManager == nil {
		return append(finalMessages, currentMsg)
	}

	if summary := historyManager.GetSummary(); summary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + summary,
		})
	}

	history := historyManager.Get()
	finalMessages = append(finalMessages, history...)
	for _, msg := range history {
		if msg.Role == currentMsg.Role && msg.Content == currentMsg.Content {
			return finalMessages
		}
	}
	return append(finalMessages, currentMsg)
}

// Loopback injects an external message into the agent loop synchronously.
// Used by webhook-based integrations (e.g. Telnyx SMS) to relay incoming
// messages through the full agent pipeline including tool execution.
// The caller should invoke this in a goroutine for non-blocking operation.
func Loopback(runCfg RunConfig, message string, broker FeedbackBroker) {
	loopbackLimiter <- struct{}{}
	defer func() { <-loopbackLimiter }()

	cfg := runCfg.Config
	logger := runCfg.Logger
	shortTermMem := runCfg.ShortTermMem
	historyManager := runCfg.HistoryManager

	if shortTermMem == nil || historyManager == nil || cfg == nil {
		if logger != nil {
			logger.Error("[Loopback] Missing required dependencies")
		}
		return
	}

	sessionID := runCfg.SessionID
	if sessionID == "" {
		sessionID = "default"
	}
	isInternalMessage := isAutonomousLoopback(runCfg, sessionID)

	// Create manifest for tool resolution
	if runCfg.Manifest == nil {
		runCfg.Manifest = tools.NewManifest(cfg.Directories.ToolsDir)
	}

	safeMessage := security.IsolateExternalData(message)

	// Insert external message into short-term memory
	mid, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, safeMessage, false, isInternalMessage)
	if err != nil {
		logger.Error("[Loopback] Failed to insert message", "error", err)
		return
	}
	NoteInnerVoiceUserTurn(sessionID)
	if shouldPersistLoopbackHistory(sessionID) && !isInternalMessage {
		historyManager.Add(openai.ChatMessageRoleUser, safeMessage, mid, false, false)
	}

	policy := buildToolingPolicy(cfg, "")
	flags := buildPromptContextFlags(runCfg, policy, promptContextOptions{
		ActiveProcesses:       GetActiveProcessStatus(runCfg.Registry),
		IsMaintenanceMode:     tools.IsBusy(),
		SpecialistsAvailable:  specialistsAvailable(runCfg.Config),
		SpecialistsStatus:     buildSpecialistsStatus(runCfg.Config),
		SpecialistsSuggestion: buildSpecialistDelegationHint(runCfg.Config, safeMessage),
	})
	coreMem := shortTermMem.ReadCoreMemory()
	sysPrompt, _ := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, &flags, coreMem, logger)

	// Assemble messages
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}
	finalMessages = buildLoopbackConversationMessages(finalMessages, historyManager, safeMessage, shouldPersistLoopbackHistory(sessionID) && !isInternalMessage)

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resp, err := ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	if err != nil {
		logger.Error("[Loopback] Agent loop failed", "error", err)
		recordOperationalIssue(runCfg, planner.OperationalIssue{
			Source:     operationalIssueSource(runCfg),
			Context:    operationalIssueContext(runCfg),
			Title:      "Background agent loop failed",
			Detail:     err.Error(),
			Severity:   "error",
			Reference:  "loopback",
			OccurredAt: time.Now(),
		}, logger)
		broker.Send("error_recovery", i18n.T(cfg.Server.UILanguage, "backend.stream_error_recovery_loopback"))
		return
	}

	// Send final response via broker
	if len(resp.Choices) > 0 {
		answer := security.StripThinkingTags(resp.Choices[0].Message.Content)
		if answer != "" {
			broker.Send("final_response", answer)
		}
	}

	logger.Info("[Loopback] Completed", "session", sessionID)
}
