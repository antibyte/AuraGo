package agent

import (
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/tools/outputcompress"

	"github.com/sashabaranov/go-openai"
)

type ExecutionOutcome int

const (
	ExecutionOutcomeSuccess         ExecutionOutcome = iota
	ExecutionOutcomeFailed                           // tool returned an error status
	ExecutionOutcomeGuardianBlocked                  // LLM Guardian blocked the tool
	ExecutionOutcomeSanitized                        // output was truncated/sanitized by policy
)

func (e ExecutionOutcome) String() string {
	switch e {
	case ExecutionOutcomeSuccess:
		return "success"
	case ExecutionOutcomeFailed:
		return "failed"
	case ExecutionOutcomeGuardianBlocked:
		return "guardian_blocked"
	case ExecutionOutcomeSanitized:
		return "sanitized"
	default:
		return "unknown"
	}
}

type toolExecutionResult struct {
	Content string
	Failed  bool
	Outcome ExecutionOutcome
}

func augmentToolFailureContent(tc ToolCall, content string, errorSummary string) string {
	if errorSummary == "" {
		errorSummary = extractErrorMessage(content)
	}
	hint := recoveryHintForToolFailure(tc, errorSummary)
	if hint == "" {
		return content
	}
	if strings.Contains(content, "[Suggested next step]") {
		return content
	}
	return content + "\n\n[Suggested next step]\n" + hint
}

func blockedToolOutputFromRequest(req *openai.ChatCompletionRequest) string {
	if req != nil && len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == openai.ChatMessageRoleSystem && strings.Contains(last.Content, "CIRCUIT BREAKER") {
			return "[TOOL BLOCKED]\n" + last.Content
		}
	}
	return "[TOOL BLOCKED]\nCIRCUIT BREAKER: This tool call was blocked because it would repeat the same failing action."
}

func finalizeToolExecution(
	tc ToolCall,
	rawContent string,
	guardianBlocked bool,
	cfg *config.Config,
	shortTermMem *memory.SQLiteMemory,
	sessionID string,
	recoveryState *toolRecoveryState,
	req *openai.ChatCompletionRequest,
	logger *slog.Logger,
	scope AgentTelemetryScope,
	promptVersion string,
	execTimeMs int64,
) toolExecutionResult {
	limit := 0
	if cfg != nil {
		limit = cfg.Agent.ToolOutputLimit
	}

	// Compress tool output before applying truncation policy.
	// This reduces token consumption by filtering, deduplicating, and
	// summarising verbose outputs while preserving semantic content.
	if !guardianBlocked && cfg != nil {
		compCfg := outputcompress.Config{
			Enabled:        cfg.Agent.OutputCompression.Enabled,
			MinChars:       cfg.Agent.OutputCompression.MinChars,
			PreserveErrors: cfg.Agent.OutputCompression.PreserveErrors,
		}
		// For zero-value config (user didn't set anything), enable with defaults
		if !compCfg.Enabled && compCfg.MinChars == 0 {
			compCfg = outputcompress.DefaultConfig()
		}
		var compStats outputcompress.CompressionStats
		rawContent, compStats = outputcompress.Compress(tc.Action, tc.Command, rawContent, compCfg)
		if compStats.Ratio < 1.0 {
			logger.Debug("output compressed",
				"tool", tc.Action,
				"filter", compStats.FilterUsed,
				"raw_chars", compStats.RawChars,
				"compressed_chars", compStats.CompressedChars,
				"ratio", fmt.Sprintf("%.2f", compStats.Ratio),
			)
			outputcompress.RecordCompressionStats(compStats)
			RecordScopedToolResultForTool(scope, "output_compression_used", true)
		} else if compStats.FilterUsed == "none" || compStats.FilterUsed == "skipped-error" {
			outputcompress.RecordCompressionSkipped()
		}
	}

	policyResult := applyToolOutputPolicy(rawContent, limit, scope)
	resultContent := policyResult.Content
	toolFailed := policyResult.WasError
	outcome := ExecutionOutcomeSuccess
	if guardianBlocked {
		outcome = ExecutionOutcomeGuardianBlocked
		toolFailed = true
	} else if toolFailed {
		outcome = ExecutionOutcomeFailed
	} else if policyResult.Truncated {
		outcome = ExecutionOutcomeSanitized
	}
	if toolFailed {
		resultContent = augmentToolFailureContent(tc, resultContent, policyResult.ErrorSummary)
	}

	prompts.RecordToolUsage(tc.Action, tc.Operation, !toolFailed)
	prompts.RecordAdaptiveToolUsage(tc.Action, !toolFailed)
	RecordScopedToolResultForTool(scope, tc.Action, !toolFailed)

	if shortTermMem != nil {
		if err := shortTermMem.UpsertToolUsage(tc.Action, !toolFailed); err != nil {
			logToolMemoryWarning(logger, "Failed to persist tool usage stats", tc.Action, err)
		}
	}

	if toolFailed && shortTermMem != nil {
		errMsg := policyResult.ErrorSummary
		if errMsg != "" {
			if err := shortTermMem.RecordError(tc.Action, errMsg); err != nil {
				logToolMemoryWarning(logger, "Failed to persist tool error pattern", tc.Action, err)
			}
			if recoveryState != nil && recoveryState.shouldRecordFirstErrorInChain() &&
				cfg != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries {
				if _, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
					EntryType:     "error_learned",
					Title:         fmt.Sprintf("Error in %s", tc.Action),
					Content:       errMsg,
					Tags:          []string{tc.Action},
					Importance:    2,
					SessionID:     sessionID,
					AutoGenerated: true,
				}); err != nil {
					logToolMemoryWarning(logger, "Failed to persist error journal entry", tc.Action, err)
				}
			}
		}
	}

	if strings.Contains(resultContent, "[TOOL BLOCKED]") && shortTermMem != nil &&
		cfg != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries {
		reason := resultContent
		if len(reason) > 150 {
			reason = truncateUTF8ToLimit(reason, 153, "...")
		}
		title := fmt.Sprintf("Tool blocked: %s", tc.Action)
		if strings.Contains(resultContent, "Guardian") {
			title = fmt.Sprintf("Guardian blocked: %s", tc.Action)
		}
		if _, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
			EntryType:     "security_event",
			Title:         title,
			Content:       reason,
			Tags:          []string{tc.Action, "security"},
			Importance:    4,
			SessionID:     sessionID,
			AutoGenerated: true,
		}); err != nil {
			logToolMemoryWarning(logger, "Failed to persist security journal entry", tc.Action, err)
		}
	}

	if !toolFailed && recoveryState != nil && recoveryState.shouldRecordResolution() && shortTermMem != nil {
		resolutionErr := extractErrorMessage(recoveryState.LastToolError)
		if resolutionErr == "" {
			resolutionErr = recoveryState.LastToolError
		}
		if err := shortTermMem.RecordResolution(tc.Action, resolutionErr, "Succeeded with adjusted parameters"); err != nil {
			logToolMemoryWarning(logger, "Failed to persist tool resolution", tc.Action, err)
		}
	}

	if recoveryState != nil && req != nil {
		_ = recoveryState.updateToolErrorState(tc, resultContent, req, logger, scope, promptVersion, execTimeMs)
	}

	return toolExecutionResult{
		Content: resultContent,
		Failed:  toolFailed,
		Outcome: outcome,
	}
}

func logToolMemoryWarning(logger *slog.Logger, message string, action string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Warn(message, "action", action, "error", err)
}
