package agent

import (
	"context"
	"encoding/json"
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
	Content      string
	EventContent string
	OutputRef    string
	Failed       bool
	Outcome      ExecutionOutcome
}

type outputVaultPayload struct {
	Status        string `json:"status"`
	OutputRef     string `json:"output_ref"`
	ToolName      string `json:"tool_name"`
	Summary       string `json:"summary"`
	View          string `json:"view"`
	OriginalChars int    `json:"original_chars"`
	Message       string `json:"message"`
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
	toolName := stringValueFromMap(tc.Params, "tool_name", "name", "tool")
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

func shouldPersistToolErrorJournal(runCfg RunConfig, sessionID, toolAction, errMsg string, shortTermMem *memory.SQLiteMemory) bool {
	if errMsg == "" {
		return false
	}
	if isAutonomousAgentRun(runCfg, sessionID) {
		return false
	}
	if shortTermMem != nil {
		title := fmt.Sprintf("Error in %s", toolAction)
		if logged, err := shortTermMem.JournalErrorRecentlyLogged("error_learned", title, errMsg, 24); err == nil && logged {
			return false
		}
	}
	return true
}

func finalizeToolExecution(
	ctx context.Context,
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
	runCfg RunConfig,
) toolExecutionResult {
	trackingTC := toolCallForExecutionTracking(tc)
	eventContent := rawContent
	limit := 0
	if cfg != nil {
		limit = cfg.Agent.ToolOutputLimit
	}

	policyResult := applyToolOutputPolicy(rawContent, limit, scope)
	rawContent = policyResult.Content

	// Apply compression after truncation so expensive filters only process the
	// retained content that can actually be added to the model context.
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
			RepetitiveSubstitution: outputcompress.RepetitiveSubstitutionConfig{
				Enabled:              cfg.Agent.OutputCompression.RepetitiveSubstitution.Enabled,
				LZWEnabled:           cfg.Agent.OutputCompression.RepetitiveSubstitution.LZWEnabled,
				LTSCLiteEnabled:      cfg.Agent.OutputCompression.RepetitiveSubstitution.LTSCLiteEnabled,
				MinPhraseChars:       cfg.Agent.OutputCompression.RepetitiveSubstitution.MinPhraseChars,
				MinOccurrences:       cfg.Agent.OutputCompression.RepetitiveSubstitution.MinOccurrences,
				MinSavingsPercent:    cfg.Agent.OutputCompression.RepetitiveSubstitution.MinSavingsPercent,
				MaxInputChars:        cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxInputChars,
				MaxDictionaryEntries: cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxDictionaryEntries,
			},
			TOONJSON: outputcompress.TOONJSONConfig{
				Enabled:           cfg.Agent.OutputCompression.TOONJSON.Enabled,
				MinSavingsPercent: cfg.Agent.OutputCompression.TOONJSON.MinSavingsPercent,
				MaxRows:           cfg.Agent.OutputCompression.TOONJSON.MaxRows,
			},
			SmartCrusher: outputcompress.SmartCrusherConfig{
				Enabled:  cfg.Agent.OutputCompression.SmartCrusher.Enabled,
				MaxRows:  cfg.Agent.OutputCompression.SmartCrusher.MaxRows,
				TailRows: 5,
				MaxCols:  20,
			},
		}
		originalContent := rawContent
		var compStats outputcompress.CompressionStats
		rawContent, compStats = outputcompress.Compress(trackingTC.Action, trackingTC.Command, rawContent, compCfg)

		// CCR: archive original output when reversible compression is enabled,
		// we are on the native tool path, and meaningful compression occurred.
		if cfg.Agent.OutputCompression.Reversible.Enabled &&
			tc.NativeCallID != "" &&
			compStats.Ratio < 0.95 &&
			shortTermMem != nil {
			_ = shortTermMem.StoreCompressedOutput(ctx, &memory.CompressedToolOutput{
				SessionID:         sessionID,
				ToolCallID:        tc.NativeCallID,
				ToolName:          trackingTC.Action,
				OriginalContent:   originalContent,
				CompressedContent: rawContent,
				CompressionRatio:  compStats.Ratio,
				FilterUsed:        compStats.FilterUsed,
			})
		}

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
		} else if compStats.FilterUsed == "none" || strings.HasPrefix(compStats.FilterUsed, "skipped-") {
			outputcompress.RecordCompressionSkipped()
		}
	}

	if limit > 0 && len(rawContent) > limit {
		postCompressionPolicy := applyToolOutputPolicy(rawContent, limit, scope)
		postCompressionPolicy.Truncated = postCompressionPolicy.Truncated || policyResult.Truncated
		if postCompressionPolicy.ErrorSummary == "" {
			postCompressionPolicy.ErrorSummary = policyResult.ErrorSummary
		}
		if policyResult.WasError {
			postCompressionPolicy.WasError = true
		}
		policyResult = postCompressionPolicy
	} else {
		policyResult.Content = rawContent
		policyResult.WasError = isToolError(rawContent)
		if summary := extractErrorMessage(rawContent); summary != "" {
			policyResult.ErrorSummary = summary
		}
	}
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
	outputRef := ""
	if !guardianBlocked {
		if compactContent, ref, ok := maybeStorePrimaryToolOutputVault(ctx, tc, trackingTC, eventContent, resultContent, toolFailed, cfg, shortTermMem, sessionID, logger); ok {
			resultContent = compactContent
			outputRef = ref
			if outcome == ExecutionOutcomeSuccess {
				outcome = ExecutionOutcomeSanitized
			}
		}
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
				cfg != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries &&
				shouldPersistToolErrorJournal(runCfg, sessionID, trackingTC.Action, errMsg, shortTermMem) {
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
			reason = truncateUTF8ToLimit(reason, 150, "...")
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

		// Trigger A: Recovery success → generate learned rule if this error has
		// recurred at least twice (indicating a pattern worth remembering).
		if cfg != nil && cfg.Agent.AutoLearning.Enabled {
			count, _ := shortTermMem.GetErrorCountInSession(trackingTC.Action, resolutionErr)
			if count >= 2 {
				go GenerateLearnedRule(ctx, shortTermMem, trackingTC.Action, resolutionErr, "Succeeded with adjusted parameters", logger)
			}
		}
	}

	circuitBroken := false
	if recoveryState != nil && req != nil {
		circuitBroken = recoveryState.updateToolErrorState(trackingTC, resultContent, req, logger, scope, promptVersion, execTimeMs)
	}

	// Trigger B: Circuit breaker triggered (≥3 identical consecutive errors) →
	// generate a learned rule so the agent remembers how to avoid this loop.
	if circuitBroken && cfg != nil && cfg.Agent.AutoLearning.Enabled && shortTermMem != nil {
		errMsg := extractErrorMessage(recoveryState.LastToolError)
		if errMsg == "" {
			errMsg = recoveryState.LastToolError
		}
		go GenerateLearnedRule(ctx, shortTermMem, trackingTC.Action, errMsg, "", logger)
	}

	return toolExecutionResult{
		Content:      resultContent,
		EventContent: eventContent,
		OutputRef:    outputRef,
		Failed:       toolFailed,
		Outcome:      outcome,
	}
}

func maybeStorePrimaryToolOutputVault(
	ctx context.Context,
	tc ToolCall,
	trackingTC ToolCall,
	originalContent string,
	contextContent string,
	toolFailed bool,
	cfg *config.Config,
	shortTermMem *memory.SQLiteMemory,
	sessionID string,
	logger *slog.Logger,
) (string, string, bool) {
	if cfg == nil || shortTermMem == nil || toolFailed || tc.NativeCallID == "" {
		return "", "", false
	}
	if !cfg.Agent.OutputCompression.Reversible.Enabled ||
		!cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault {
		return "", "", false
	}
	if tc.Action == "read_tool_output" || tc.Action == "retrieve_original_output" {
		return "", "", false
	}
	maxInline := cfg.Agent.OutputCompression.Reversible.MaxInlineChars
	if maxInline <= 0 {
		maxInline = 6000
	}
	if len(originalContent) <= maxInline {
		return "", "", false
	}

	outputRef := memory.StableToolOutputRef(sessionID, tc.NativeCallID)
	summary := summarizeToolOutputForVault(originalContent)
	view := buildToolOutputVaultView(originalContent, maxInline)
	ratio := 1.0
	if len(originalContent) > 0 {
		ratio = float64(len(view)) / float64(len(originalContent))
	}
	out := &memory.CompressedToolOutput{
		SessionID:         sessionID,
		ToolCallID:        tc.NativeCallID,
		OutputRef:         outputRef,
		ToolName:          trackingTC.Action,
		OriginalContent:   originalContent,
		CompressedContent: contextContent,
		SummaryContent:    summary,
		ViewContent:       view,
		CompressionRatio:  ratio,
		FilterUsed:        "output-vault",
	}
	if err := shortTermMem.StoreCompressedOutput(ctx, out); err != nil {
		logToolMemoryWarning(logger, "Failed to store primary output vault entry", trackingTC.Action, err)
		return "", "", false
	}
	payload := outputVaultPayload{
		Status:        "success",
		OutputRef:     out.OutputRef,
		ToolName:      trackingTC.Action,
		Summary:       summary,
		View:          view,
		OriginalChars: len(originalContent),
		Message:       "Full output archived. Use read_tool_output with output_ref for more views.",
	}
	b, err := json.Marshal(payload)
	if err != nil {
		logToolMemoryWarning(logger, "Failed to encode primary output vault view", trackingTC.Action, err)
		return "", "", false
	}
	return "Tool Output: " + string(b), out.OutputRef, true
}

func summarizeToolOutputForVault(content string) string {
	lines := splitOutputLines(content)
	var nonEmpty []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty = append(nonEmpty, line)
		if len(nonEmpty) >= 5 {
			break
		}
	}
	prefix := fmt.Sprintf("%d chars", len(content))
	if len(lines) > 0 {
		prefix = fmt.Sprintf("%s, %d lines", prefix, len(lines))
	}
	if len(nonEmpty) == 0 {
		return prefix
	}
	summary := prefix + ". First lines: " + strings.Join(nonEmpty, " | ")
	if len(summary) > 800 {
		summary = truncateUTF8ToLimit(summary, 800, "...")
	}
	return summary
}

func buildToolOutputVaultView(content string, maxInline int) string {
	if maxInline <= 0 {
		maxInline = 6000
	}
	viewLimit := maxInline
	if viewLimit > 4000 {
		viewLimit = 4000
	}
	view := selectHeadLines(content, 80)
	if len(view) > viewLimit {
		view = truncateUTF8ToLimit(view, viewLimit, "...")
	}
	if view == "" {
		view = truncateUTF8ToLimit(content, viewLimit, "...")
	}
	return view
}

func logToolMemoryWarning(logger *slog.Logger, message string, action string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Warn(message, "action", action, "error", err)
}
