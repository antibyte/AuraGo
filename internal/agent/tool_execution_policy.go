package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

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
	ExecutionOutcomeUnknown
	ExecutionOutcomeDeferred
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
	case ExecutionOutcomeDeferred:
		return "deferred"
	default:
		return "unknown"
	}
}

type toolExecutionResult struct {
	Status       ToolResultStatus
	Content      string
	EventContent string
	OutputRef    string
	Failed       bool
	Outcome      ExecutionOutcome
}

// suggestedNextStepHeader introduces the recovery hint that
// augmentToolFailureContent appends to a failed plain-text result.
const suggestedNextStepHeader = "\n\n[Suggested next step]\n"

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
	var payload map[string]interface{}
	value := strings.TrimSpace(strings.TrimPrefix(content, "Tool Output: "))
	if json.Unmarshal([]byte(value), &payload) == nil && payload != nil {
		payload["suggested_next_step"] = hint
		encoded, _ := json.Marshal(payload)
		return string(encoded)
	}
	return content + suggestedNextStepHeader + hint
}

// failureHintLen is what augmentToolFailureContent appends to a failed
// plain-text result with this error summary.
func failureHintLen(tc ToolCall, errorSummary string) int {
	if hint := recoveryHintForToolFailure(tc, errorSummary); hint != "" {
		return len(suggestedNextStepHeader) + len(hint)
	}
	return 0
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
	args := mapValueFromMap(tc.Params, "arguments", "params", "skill_args")
	if args == nil {
		args = flattenedInvokeArgs(tc.Params)
	}
	if args != nil {
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
	inline inlineBudgetBasis,
) toolExecutionResult {
	trackingTC := toolCallForExecutionTracking(tc)
	status := tc.DispatchStatus
	if status == "" {
		status = classifyLegacyToolResult(rawContent)
	}
	if guardianBlocked {
		status = ToolResultDenied
	}
	eventContent := rawContent
	limit := 0
	if cfg != nil {
		limit = cfg.Agent.ToolOutputLimit
	}
	// A failure gets its recovery hint appended after the policy. Reserve it so
	// the final bound does not cut the preserved error summary again.
	policyLimit := limit
	if status.IsError() {
		effective := effectiveToolOutputLimit(cfg)
		if reserve := failureHintLen(trackingTC, extractErrorMessage(rawContent)); reserve > 0 && reserve < effective/2 {
			policyLimit = effective - reserve
		}
	}

	policyResult := applyToolOutputPolicy(rawContent, policyLimit, scope, status)
	rawContent = policyResult.Content

	// Apply compression after truncation so expensive filters only process the
	// retained content that can actually be added to the model context.
	// Config defaults are applied in config.go via yamlHasPath, so the
	// zero-value heuristic is no longer needed here.
	if !guardianBlocked && cfg != nil && trackingTC.Action != "discover_tools" {
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
		payload, isolated, raw := toolResultPayloadForm(rawContent)
		if isolated {
			payload, compStats = outputcompress.Compress(trackingTC.Action, trackingTC.Command, payload, compCfg)
			rawContent = toolResultPresentationPrefix(rawContent) + isolateToolPayload(payload, raw) + toolResultPresentationSuffix(rawContent)
		} else {
			rawContent, compStats = outputcompress.Compress(trackingTC.Action, trackingTC.Command, rawContent, compCfg)
		}

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

	if limit > 0 && len(rawContent) > policyLimit {
		postCompressionPolicy := applyToolOutputPolicy(rawContent, policyLimit, scope, status)
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
		policyResult.WasError = status.IsError()
		if summary := extractErrorMessage(rawContent); summary != "" {
			policyResult.ErrorSummary = summary
		}
	}
	resultContent := policyResult.Content
	toolFailed := status.IsError()
	outcome := ExecutionOutcomeSuccess
	if status == ToolResultUnknown {
		outcome = ExecutionOutcomeUnknown
	} else if status == ToolResultDeferred {
		outcome = ExecutionOutcomeDeferred
	}
	if guardianBlocked {
		outcome = ExecutionOutcomeGuardianBlocked
		toolFailed = true
	} else if toolFailed {
		outcome = ExecutionOutcomeFailed
	} else if policyResult.Truncated && status == ToolResultSuccess {
		outcome = ExecutionOutcomeSanitized
	}
	if toolFailed {
		resultContent = augmentToolFailureContent(trackingTC, resultContent, policyResult.ErrorSummary)
	}
	outputRef := ""
	// Scoped builders cannot follow archive references when retrieval is excluded.
	// Keep their bounded inline response rather than advertise an unusable tool.
	_, canReadOutput := normalizedAllowedToolSet(runCfg.AllowedTools)["read_tool_output"]
	if !guardianBlocked && (runCfg.AllowedTools == nil || canReadOutput) {
		if compactContent, ref, ok := maybeStorePrimaryToolOutputVault(ctx, tc, trackingTC, eventContent, resultContent, toolFailed, cfg, shortTermMem, sessionID, logger, inline); ok {
			resultContent = compactContent
			outputRef = ref
			if outcome == ExecutionOutcomeSuccess {
				outcome = ExecutionOutcomeSanitized
			}

		}
	}

	if status == ToolResultSuccess || status == ToolResultFailed {
		emitConfirmedToolPersonality(shortTermMem, cfg, runCfg, tc, trackingTC, status, logger)
		prompts.RecordToolUsage(trackingTC.Action, trackingTC.Operation, !toolFailed)
		prompts.RecordAdaptiveToolUsage(trackingTC.Action, !toolFailed)
		RecordScopedToolResultForTool(scope, trackingTC.Action, !toolFailed)

		if shortTermMem != nil {
			if err := shortTermMem.UpsertToolUsage(trackingTC.Action, !toolFailed); err != nil {
				logToolMemoryWarning(logger, "Failed to persist tool usage stats", trackingTC.Action, err)
			}
		}

	}

	if toolFailed && shortTermMem != nil {
		diagnosticTC := trackingTC
		diagnosticTC.DispatchStatus = status
		errMsg := correctionDiagnostic(diagnosticTC, policyResult.ErrorSummary)
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

	if status == ToolResultSuccess && recoveryState != nil && recoveryState.shouldRecordResolution() && shortTermMem != nil {
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
				_ = sideEffectsFromRunConfig(runCfg).Go(func(taskCtx context.Context) {
					learnCtx, cancel := context.WithTimeout(taskCtx, 60*time.Second)
					defer cancel()
					GenerateLearnedRule(learnCtx, shortTermMem, trackingTC.Action, resolutionErr, "Succeeded with adjusted parameters", logger)
				})
			}
		}
	}

	circuitBroken := false
	if (status == ToolResultSuccess || status.IsError()) && recoveryState != nil && req != nil {
		trackingTC.DispatchStatus = status
		circuitBroken = recoveryState.updateToolErrorState(trackingTC, correctionDiagnostic(trackingTC, resultContent), req, logger, scope, promptVersion, execTimeMs)
	}

	// Trigger B: Circuit breaker triggered (≥3 identical consecutive errors) →
	// generate a learned rule so the agent remembers how to avoid this loop.
	if circuitBroken && cfg != nil && cfg.Agent.AutoLearning.Enabled && shortTermMem != nil {
		errMsg := extractErrorMessage(recoveryState.LastToolError)
		if errMsg == "" {
			errMsg = recoveryState.LastToolError
		}
		_ = sideEffectsFromRunConfig(runCfg).Go(func(taskCtx context.Context) {
			learnCtx, cancel := context.WithTimeout(taskCtx, 60*time.Second)
			defer cancel()
			GenerateLearnedRule(learnCtx, shortTermMem, trackingTC.Action, errMsg, "", logger)
		})
	}

	resultContent = boundedToolResult(resultContent, effectiveToolOutputLimit(cfg), status)

	return toolExecutionResult{
		Status:       status,
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
	inline inlineBudgetBasis,
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
	maxInline := primaryOutputVaultMaxInline(cfg, inline)
	if len(originalContent) <= maxInline {
		return "", "", false
	}

	outputRef := memory.StableToolOutputRef(sessionID, tc.NativeCallID)
	status := tc.DispatchStatus
	if status == "" {
		status = classifyLegacyToolResult(originalContent)
	}
	payload, isolated := vaultReadablePayload(originalContent)
	content, view := fitOutputVaultReply(outputVaultPayload{
		Status:        string(status),
		OutputRef:     outputRef,
		ToolName:      trackingTC.Action,
		Summary:       vaultOutputStats(payload),
		OriginalChars: len(payload),
	}, payload, isolated, toolResultPresentationSuffix(originalContent), min(maxInline, effectiveToolOutputLimit(cfg)))
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
		SummaryContent:    summarizeToolOutputForVault(payload),
		ViewContent:       view,
		CompressionRatio:  ratio,
		FilterUsed:        "output-vault",
	}
	if err := shortTermMem.StoreCompressedOutput(ctx, out); err != nil {
		logToolMemoryWarning(logger, "Failed to store primary output vault entry", trackingTC.Action, err)
		return "", "", false
	}
	return content, out.OutputRef, true
}

const (
	// toolResultInlineFloor is the inline budget in bytes for an unknown or
	// small model context (the former fixed default).
	toolResultInlineFloor = 6000
	// toolResultInlineCeiling caps the automatic budget of a single call; a
	// larger result goes to the output vault, whose view fills the budget.
	toolResultInlineCeiling = 32000
	// One automatic inline result may use this share (percent) of the tightest
	// route's input capacity, counted at four bytes per token. Escaped
	// external output measures about 2-2.5 bytes per token, so such a result
	// takes up to about 5% of the input.
	toolResultInlinePercent       = 3
	toolResultInlineBytesPerToken = 4
)

// inlineBudgetBasis is what the automatic inline budget scales with.
type inlineBudgetBasis struct {
	// InputTokens is the input capacity of the tightest route of the current
	// request; 0 when unknown (standalone tool bridges).
	InputTokens int
	// BatchCalls is the number of tool calls the model declared in the
	// response being executed. Their results share the next request, and
	// the newest two tool rounds always stay complete.
	BatchCalls int
}

// primaryOutputVaultMaxInline is the size in bytes (len of the dispatch
// output) above which maybeStorePrimaryToolOutputVault archives a successful
// native tool result. A positive
// agent.output_compression.reversible.max_inline_chars fixes it; 0 (the
// default) scales it with the current request (autoToolResultInlineBudget).
func primaryOutputVaultMaxInline(cfg *config.Config, basis inlineBudgetBasis) int {
	if cfg != nil && cfg.Agent.OutputCompression.Reversible.MaxInlineChars > 0 {
		return cfg.Agent.OutputCompression.Reversible.MaxInlineChars
	}
	return autoToolResultInlineBudget(basis)
}

// autoToolResultInlineBudget is 3% of the input capacity at four bytes per
// token, at most 32000 bytes, divided by the square root of the calls in the
// batch, and at least 6000 bytes (also for an unknown capacity). A single
// call gets 6000 bytes for a 32k context, about 14.7 KB for 128k and about
// 29.7 KB for 256k; three parallel calls on a 256k model get about 17 KB each.
func autoToolResultInlineBudget(basis inlineBudgetBasis) int {
	if basis.InputTokens <= 0 {
		return toolResultInlineFloor
	}
	budget := min(basis.InputTokens*toolResultInlineBytesPerToken*toolResultInlinePercent/100, toolResultInlineCeiling)
	if basis.BatchCalls > 1 {
		budget = int(float64(budget) / math.Sqrt(float64(basis.BatchCalls)))
	}
	return max(budget, toolResultInlineFloor)
}

// toolResultInlineBudget is the largest dispatch output in bytes that reaches
// the model whole in this dispatch: at most the tool output limit and, while
// the primary output vault is on, at most its archive threshold for the
// current request (dc.inlineBasis). Tools that page their answers
// (local_wikipedia) size them to it.
func toolResultInlineBudget(dc *DispatchContext) int {
	var cfg *config.Config
	var basis inlineBudgetBasis
	if dc != nil {
		cfg, basis = dc.Cfg, dc.inlineBasis
	}
	budget := effectiveToolOutputLimit(cfg)
	if cfg == nil || (cfg.Agent.OutputCompression.Reversible.Enabled && cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault) {
		budget = min(budget, primaryOutputVaultMaxInline(cfg, basis))
	}
	return budget
}

func logToolMemoryWarning(logger *slog.Logger, message string, action string, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.Warn(message, "action", action, "error", err)
}

func effectiveToolOutputLimit(cfg *config.Config) int {
	if cfg != nil && cfg.Agent.ToolOutputLimit > 0 {
		return cfg.Agent.ToolOutputLimit
	}
	return 50000
}
