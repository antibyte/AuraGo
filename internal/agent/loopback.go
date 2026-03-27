package agent

import (
	"context"
	"time"

	"aurago/internal/prompts"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// Loopback injects an external message into the agent loop synchronously.
// Used by webhook-based integrations (e.g. Telnyx SMS) to relay incoming
// messages through the full agent pipeline including tool execution.
// The caller should invoke this in a goroutine for non-blocking operation.
func Loopback(runCfg RunConfig, message string, broker FeedbackBroker) {
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

	// Create manifest for tool resolution
	if runCfg.Manifest == nil {
		runCfg.Manifest = tools.NewManifest(cfg.Directories.ToolsDir)
	}

	// Insert external message into short-term memory
	mid, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, message, false, false)
	if err != nil {
		logger.Error("[Loopback] Failed to insert message", "error", err)
		return
	}
	historyManager.Add(openai.ChatMessageRoleUser, message, mid, false, false)

	policy := buildToolingPolicy(cfg, "")
	flags := buildPromptContextFlags(runCfg, policy, promptContextOptions{
		ActiveProcesses:       GetActiveProcessStatus(runCfg.Registry),
		IsMaintenanceMode:     tools.IsBusy(),
		SpecialistsAvailable:  specialistsAvailable(runCfg.Config),
		SpecialistsStatus:     buildSpecialistsStatus(runCfg.Config),
		SpecialistsSuggestion: buildSpecialistDelegationHint(runCfg.Config, message),
	})
	coreMem := shortTermMem.ReadCoreMemory()
	sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMem, logger)

	// Assemble messages
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}
	if summary := historyManager.GetSummary(); summary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + summary,
		})
	}
	finalMessages = append(finalMessages, historyManager.Get()...)

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resp, err := ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	if err != nil {
		logger.Error("[Loopback] Agent loop failed", "error", err)
		broker.Send("error_recovery", "Sorry, I encountered an error processing your message.")
		return
	}

	// Send final response via broker
	if len(resp.Choices) > 0 {
		answer := resp.Choices[0].Message.Content
		if answer != "" {
			broker.Send("final_response", answer)
		}
	}

	logger.Info("[Loopback] Completed", "session", sessionID)
}
