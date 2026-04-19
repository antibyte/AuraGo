package agent

import (
	"context"
	"fmt"
	"time"

	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

const vscodeDebugBridgeSessionID = "vscode-debug-bridge"

// AskAuraGoBridge executes a dedicated AuraGo debugging turn for MCP-based IDE agents.
// It uses an isolated session so the live debugging conversation does not pollute
// the main default chat history.
func AskAuraGoBridge(ctx context.Context, runCfg RunConfig, message string) (string, error) {
	cfg := runCfg.Config
	logger := runCfg.Logger
	shortTermMem := runCfg.ShortTermMem

	if cfg == nil || logger == nil || shortTermMem == nil || runCfg.LLMClient == nil {
		return "", fmt.Errorf("ask_aurago bridge is missing required dependencies")
	}

	sessionID := runCfg.SessionID
	if sessionID == "" {
		sessionID = vscodeDebugBridgeSessionID
	}
	runCfg.SessionID = sessionID
	runCfg.MessageSource = "mcp-vscode-bridge"

	if runCfg.Manifest == nil {
		runCfg.Manifest = tools.NewManifest(cfg.Directories.ToolsDir)
	}

	historyManager := memory.NewEphemeralHistoryManager()
	runCfg.HistoryManager = historyManager

	if recent, err := shortTermMem.GetRecentMessages(sessionID, 80); err == nil {
		for _, msg := range recent {
			_ = historyManager.Add(msg.Role, msg.Content, 0, false, false)
		}
	}

	msgID, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, message, false, false)
	if err != nil {
		return "", fmt.Errorf("failed to persist bridge message: %w", err)
	}
	_ = historyManager.Add(openai.ChatMessageRoleUser, message, msgID, false, false)

	policy := buildToolingPolicy(cfg, message)
	flags := buildPromptContextFlags(runCfg, policy, promptContextOptions{
		ActiveProcesses:       GetActiveProcessStatus(runCfg.Registry),
		IsMaintenanceMode:     tools.IsBusy(),
		SpecialistsAvailable:  specialistsAvailable(runCfg.Config),
		SpecialistsStatus:     buildSpecialistsStatus(runCfg.Config),
		SpecialistsSuggestion: buildSpecialistDelegationHint(runCfg.Config, message),
	})
	coreMem := shortTermMem.ReadCoreMemory()
	sysPrompt, _ := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, &flags, coreMem, logger)

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

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	resp, err := ExecuteAgentLoop(ctx, req, runCfg, false, NoopBroker{})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("ask_aurago bridge returned no choices")
	}
	answer := resp.Choices[0].Message.Content
	if answer == "" {
		return "", fmt.Errorf("ask_aurago bridge returned an empty response")
	}
	return answer, nil
}
