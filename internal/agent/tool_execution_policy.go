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

func toolCallForExecutionTracking(tc ToolCall) ToolCall {
	if tc.Action != "invoke_tool" {
		return tc
	}
	toolName := stringValueFromMap(tc.Params, "tool_name", "name")
	if toolName == "" {
		return tc
	}
	tracking := tc
	tracking.Action = toolName
	if args := mapValueFromMap(tc.Params, "arguments", "params", "skill_args"); args != nil {
		tracking.Params = args
		if op := stringValueFromMap(args, "operation"); op != "" {
			tracking.Operation = op
		}
	}
	return tracking
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
	trackingTC := toolCallForExecutionTracking(tc)
	limit := 0
	if cfg != nil {
		limit = cfg.Agent.ToolOutputLimit
	}

	// Compress tool output before applying truncation policy.
	// This reduces token consumption by filtering, deduplicating, and
	// summarising verbose outputs while preserving semantic content.
	// Config defaults are applied in config.go via yamlHasPath, so the
	// zero-value heuristic is no longer needed here.
	if !guardianBlocked && cfg != nil {
		compCfg := outputcompress.Config{
			Enabled:           cfg.Agent.OutputCompression.Enabled,
			MinChars:          cfg.Agent.OutputCompression.MinChars,
			PreserveErrors:    cfg.Agent.OutputCompression.PreserveErrors,
			ShellCompression:  cfg.Agent.OutputCompression.ShellCompression,
			PythonCompression: cfg.Agent.OutputCompression.PythonCompression,
			APICompression:    cfg.Agent.OutputCompression.APICompression,
		}
		var compStats outputcompress.CompressionStats
		rawContent, compStats = outputcompress.Compress(trackingTC.Action, trackingTC.Command, rawContent, compCfg)
		if compStats.Ratio < 1.0 {
			logger.Debug("output compressed",
				"tool", trackingTC.Action,
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
		resultContent = augmentToolFailureContent(trackingTC, resultContent, policyResult.ErrorSummary)
	}

	prompts.RecordToolUsage(trackingTC.Action, trackingTC.Operation, !toolFailed)
	prompts.RecordAdaptiveToolUsage(trackingTC.Action, !toolFailed)
	RecordScopedToolResultForTool(scope, trackingTC.Action, !toolFailed)

	if shortTermMem != nil {
		if err := shortTermMem.UpsertToolUsage(trackingTC.Action, !toolFailed); err != nil {
			logToolMemoryWarning(logger, "Failed to persist tool usage stats", trackingTC.Action, err)
		}
	}

	if toolFailed && shortTermMem != nil {
		errMsg := policyResult.ErrorSummary
		if errMsg != "" {
			if err := shortTermMem.RecordError(trackingTC.Action, errMsg); err != nil {
				logToolMemoryWarning(logger, "Failed to persist tool error pattern", trackingTC.Action, err)
			}
			if recoveryState != nil && recoveryState.shouldRecordFirstErrorInChain() &&
				cfg != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries {
				if _, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
					EntryType:     "error_learned",
					Title:         fmt.Sprintf("Error in %s", trackingTC.Action),
					Content:       errMsg,
					Tags:          []string{trackingTC.Action},
					Importance:    2,
					SessionID:     sessionID,
					AutoGenerated: true,
				}); err != nil {
					logToolMemoryWarning(logger, "Failed to persist error journal entry", trackingTC.Action, err)
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
		title := fmt.Sprintf("Tool blocked: %s", trackingTC.Action)
		if strings.Contains(resultContent, "Guardian") {
			title = fmt.Sprintf("Guardian blocked: %s", trackingTC.Action)
		}
		if _, err := shortTermMem.InsertJournalEntry(memory.JournalEntry{
			EntryType:     "security_event",
			Title:         title,
			Content:       reason,
			Tags:          []string{trackingTC.Action, "security"},
			Importance:    4,
			SessionID:     sessionID,
			AutoGenerated: true,
		}); err != nil {
			logToolMemoryWarning(logger, "Failed to persist security journal entry", trackingTC.Action, err)
		}
	}

	if !toolFailed && recoveryState != nil && recoveryState.shouldRecordResolution() && shortTermMem != nil {
		resolutionErr := extractErrorMessage(recoveryState.LastToolError)
		if resolutionErr == "" {
			resolutionErr = recoveryState.LastToolError
		}
		if err := shortTermMem.RecordResolution(trackingTC.Action, resolutionErr, "Succeeded with adjusted parameters"); err != nil {
			logToolMemoryWarning(logger, "Failed to persist tool resolution", trackingTC.Action, err)
		}
	}

	if recoveryState != nil && req != nil {
		_ = recoveryState.updateToolErrorState(trackingTC, resultContent, req, logger, scope, promptVersion, execTimeMs)
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
