package agent

import (
	"context"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

var loopbackLimiter = make(chan struct{}, 8)

const (
	loopbackSessionMaxMessages = 24
	loopbackSessionMaxChars    = 80000
)

func shouldPersistLoopbackHistory(sessionID string) bool {
	return sessionID == "default"
}

func isAutonomousLoopback(runCfg RunConfig, sessionID string) bool {
	return isAutonomousAgentRun(runCfg, sessionID)
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

func buildLoopbackSessionConversationMessages(base []openai.ChatCompletionMessage, sessionMessages []memory.HistoryMessage, safeMessage string) []openai.ChatCompletionMessage {
	finalMessages := append([]openai.ChatCompletionMessage(nil), base...)
	currentMsg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: safeMessage}
	currentIncluded := false
	visibleMessages := make([]openai.ChatCompletionMessage, 0, len(sessionMessages)+1)
	for _, stored := range sessionMessages {
		if stored.IsInternal {
			continue
		}
		msg := stored.ChatCompletionMessage
		if msg.Role == "" || msg.Content == "" {
			continue
		}
		visibleMessages = append(visibleMessages, msg)
		if msg.Role == currentMsg.Role && msg.Content == currentMsg.Content {
			currentIncluded = true
		}
	}
	if !currentIncluded {
		visibleMessages = append(visibleMessages, currentMsg)
	}
	return append(finalMessages, trimLoopbackSessionMessages(visibleMessages)...)
}

func trimLoopbackSessionMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	start := 0
	if len(messages) > loopbackSessionMaxMessages {
		start = len(messages) - loopbackSessionMaxMessages
	}
	messages = messages[start:]

	if loopbackSessionMaxChars <= 0 {
		return messages
	}
	kept := make([]openai.ChatCompletionMessage, 0, len(messages))
	chars := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msgChars := len(messageText(messages[i])) + 4
		if len(kept) > 0 && chars+msgChars > loopbackSessionMaxChars {
			break
		}
		kept = append(kept, messages[i])
		chars += msgChars
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept
}

// Loopback injects an external message into the agent loop synchronously.
// Used by webhook-based integrations (e.g. Telnyx SMS) to relay incoming
// messages through the full agent pipeline including tool execution.
// The caller should invoke this in a goroutine for non-blocking operation.
func Loopback(runCfg RunConfig, message string, broker FeedbackBroker) {
	LoopbackContext(context.Background(), runCfg, message, broker)
}

// LoopbackContext injects an external message into the agent loop and cancels
// the work when ctx is canceled.
func LoopbackContext(ctx context.Context, runCfg RunConfig, message string, broker FeedbackBroker) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case loopbackLimiter <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-loopbackLimiter }()

	cfg := runCfg.Config
	logger := runCfg.Logger
	shortTermMem := runCfg.ShortTermMem
	historyManager := runCfg.HistoryManager

	if err := ctx.Err(); err != nil {
		if logger != nil {
			logger.Info("[Loopback] Canceled before start", "error", err)
		}
		return
	}

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

	// Assemble messages
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem},
	}
	includeGlobalHistory := shouldPersistLoopbackHistory(sessionID) && !isInternalMessage
	if includeGlobalHistory || isInternalMessage {
		finalMessages = buildLoopbackConversationMessages(finalMessages, historyManager, safeMessage, includeGlobalHistory)
	} else {
		sessionMessages, err := shortTermMem.GetSessionMessages(sessionID)
		if err != nil {
			if logger != nil {
				logger.Warn("[Loopback] Failed to load session history, continuing with current message only", "session", sessionID, "error", err)
			}
			finalMessages = append(finalMessages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: safeMessage})
		} else {
			finalMessages = buildLoopbackSessionConversationMessages(finalMessages, sessionMessages, safeMessage)
			sanitizedMessages, droppedToolMessages := SanitizeToolMessages(finalMessages)
			if droppedToolMessages > 0 {
				if logger != nil {
					logger.Warn("[Loopback] Sanitized orphaned tool messages in session history", "session", sessionID, "dropped", droppedToolMessages, "before", len(finalMessages), "after", len(sanitizedMessages))
				}
			}
			finalMessages = sanitizedMessages
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
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
