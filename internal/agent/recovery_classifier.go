package agent

import (
	"aurago/internal/config"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// RecoveryCategory classifies LLM response problems into 3 actionable groups.
// This enables consolidated recovery handling instead of 8+ separate feedback loops.
//
// Categories:
//   - FormatError: Model produced text that looks like a tool call but isn't valid JSON/XML
//   - SchemaError: Model produced valid JSON/XML but it's structurally wrong (wrong keys, malformed args)
//   - EmptyResponse: Model produced no actionable output at all
type RecoveryCategory int

const (
	RecoveryCategoryNone RecoveryCategory = iota
	RecoveryCategoryFormatError
	RecoveryCategorySchemaError
	RecoveryCategoryEmptyResponse
)

func (c RecoveryCategory) String() string {
	switch c {
	case RecoveryCategoryFormatError:
		return "format_error"
	case RecoveryCategorySchemaError:
		return "schema_error"
	case RecoveryCategoryEmptyResponse:
		return "empty_response"
	default:
		return "none"
	}
}

// ToolCallProblem describes a detected problem with a tool call attempt.
type ToolCallProblem struct {
	Category   RecoveryCategory
	SubType    string // specific problem subtype for telemetry/logging
	RetryCount int    // current retry attempt for this category
	MaxRetries int    // maximum retries allowed for this category
	Retryable  bool   // whether this problem type is retryable
	Suggestion string // human-readable hint for debugging
}

// parseProblemRegistry tracks retry state for each problem category.
// This is a minimal state holder that enables future recovery consolidation.
type parseProblemRegistry struct {
	formatErrorCount   int
	schemaErrorCount   int
	emptyResponseCount int
}

func newParseProblemRegistry() parseProblemRegistry {
	return parseProblemRegistry{}
}

// RecordFormatError increments the format error counter and returns updated problem.
func (r *parseProblemRegistry) RecordFormatError(subType string) ToolCallProblem {
	r.formatErrorCount++
	return ToolCallProblem{
		Category:   RecoveryCategoryFormatError,
		SubType:    subType,
		RetryCount: r.formatErrorCount,
		MaxRetries: 3,
		Retryable:  true,
		Suggestion: "Model produced malformed tool call format",
	}
}

// RecordSchemaError increments the schema error counter and returns updated problem.
func (r *parseProblemRegistry) RecordSchemaError(subType string) ToolCallProblem {
	r.schemaErrorCount++
	return ToolCallProblem{
		Category:   RecoveryCategorySchemaError,
		SubType:    subType,
		RetryCount: r.schemaErrorCount,
		MaxRetries: 2,
		Retryable:  true,
		Suggestion: "Model produced structurally invalid tool call",
	}
}

// RecordEmptyResponse increments the empty response counter and returns updated problem.
func (r *parseProblemRegistry) RecordEmptyResponse() ToolCallProblem {
	r.emptyResponseCount++
	return ToolCallProblem{
		Category:   RecoveryCategoryEmptyResponse,
		SubType:    "no_output",
		RetryCount: r.emptyResponseCount,
		MaxRetries: 3,
		Retryable:  true,
		Suggestion: "Model produced no actionable output",
	}
}

// ShouldRetry returns true if the problem can be retried within the limit.
func (p ToolCallProblem) ShouldRetry() bool {
	return p.Retryable && p.RetryCount < p.MaxRetries
}

// ClassifyToolCallProblem analyzes a tool call attempt and classifies any detected problems.
// This function is the single entry point for categorizing LLM response issues,
// replacing the 8+ separate feedback loops in agent_loop.go.
//
// The classifier is conservative: it only marks a category as problematic when
// there's clear evidence, avoiding false positives that would trigger unnecessary retries.
func ClassifyToolCallProblem(
	tc ToolCall,
	content string,
	parsedToolResp ParsedToolResponse,
	useNativeFunctions bool,
) ToolCallProblem {
	lowerContent := strings.ToLower(content)

	// 1. Check for Raw Code Detection (Format Error)
	if tc.RawCodeDetected {
		return ToolCallProblem{
			Category:   RecoveryCategoryFormatError,
			SubType:    "raw_code",
			RetryCount: 0, // will be incremented by caller
			MaxRetries: 2,
			Retryable:  true,
			Suggestion: "Model sent raw Python/code instead of JSON tool call",
		}
	}

	// 2. Check for Incomplete Tool Call (Schema Error)
	// Model emitted bare <tool_call> or [TOOL_CALL] tag without valid JSON body
	if parsedToolResp.IncompleteToolCall && !tc.IsTool {
		return ToolCallProblem{
			Category:   RecoveryCategorySchemaError,
			SubType:    "incomplete_tool_call",
			RetryCount: 0,
			MaxRetries: 3,
			Retryable:  true,
			Suggestion: "Model emitted bare tool tag without JSON body",
		}
	}

	// 3. Check for Orphaned [TOOL_CALL] tag (Format Error)
	// Literal "[TOOL_CALL]" without closing "[/TOOL_CALL]"
	if !tc.IsTool && !useNativeFunctions {
		hasOpenTag := strings.Contains(lowerContent, "[tool_call]")
		hasCloseTag := strings.Contains(lowerContent, "[/tool_call]")
		if hasOpenTag && !hasCloseTag {
			return ToolCallProblem{
				Category:   RecoveryCategoryFormatError,
				SubType:    "orphaned_bracket_tag",
				RetryCount: 0,
				MaxRetries: 2,
				Retryable:  true,
				Suggestion: "Model sent [TOOL_CALL] without closing tag",
			}
		}
	}

	// 4. Check for Bare <tool_call> XML in Native Mode (Format Error)
	if !tc.IsTool && useNativeFunctions {
		if strings.Contains(lowerContent, "<tool_call") ||
			strings.Contains(lowerContent, "minimax:tool_call") {
			return ToolCallProblem{
				Category:   RecoveryCategoryFormatError,
				SubType:    "bare_xml_in_native_mode",
				RetryCount: 0,
				MaxRetries: 2,
				Retryable:  true,
				Suggestion: "Model sent XML tool tag in native function-calling mode",
			}
		}
	}

	// 5. Check for XML Fallback Format (Schema Error - not native API)
	// Model used proprietary XML format instead of native API
	if tc.XMLFallbackDetected {
		return ToolCallProblem{
			Category:   RecoveryCategorySchemaError,
			SubType:    "xml_fallback_format",
			RetryCount: 0,
			MaxRetries: 2,
			Retryable:  true,
			Suggestion: "Model used XML format instead of native function-calling API",
		}
	}

	// 6. Check for Invalid Native Tool Args (Schema Error)
	if useNativeFunctions && tc.NativeArgsMalformed {
		return ToolCallProblem{
			Category:   RecoveryCategorySchemaError,
			SubType:    "invalid_native_args",
			RetryCount: 0,
			MaxRetries: 2,
			Retryable:  true,
			Suggestion: "Native function call had malformed arguments JSON",
		}
	}

	// 7. Check for Missed Tool Call in Fence (Format Error)
	// Tool call wrapped in markdown fence instead of bare JSON
	if !tc.IsTool && !tc.RawCodeDetected {
		if (strings.Contains(content, "```") || strings.Contains(content, "{")) &&
			(strings.Contains(content, `"action"`) || strings.Contains(content, `'action'`)) {
			return ToolCallProblem{
				Category:   RecoveryCategoryFormatError,
				SubType:    "tool_in_fence",
				RetryCount: 0,
				MaxRetries: 2,
				Retryable:  true,
				Suggestion: "Model wrapped tool call in markdown fence instead of bare JSON",
			}
		}
	}

	// 8. Check for Announcement-only Response (Empty Response variant)
	// Model announced what it would do but didn't actually call a tool.
	// Uses the existing isAnnouncementOnlyResponse function from agent_loop_announcements.go.
	announcementContent := parsedToolResp.SanitizedContent
	if announcementContent != "" && !parsedToolResp.IsFinished && !tc.IsTool {
		if isAnnouncementOnlyResponse(announcementContent, tc, useNativeFunctions, false, "") {
			return ToolCallProblem{
				Category:   RecoveryCategoryEmptyResponse,
				SubType:    "announcement_only",
				RetryCount: 0,
				MaxRetries: 4,
				Retryable:  true,
				Suggestion: "Model announced action without producing tool call",
			}
		}
	}

	// No problem detected
	return ToolCallProblem{}
}

// ValidateToolCall performs post-parsing schema validation.
// Returns a ToolCallProblem if validation fails, or an empty problem if valid.
//
// Validation rules:
//   - If IsTool is true, Action must be non-empty
//   - If NativeArgsMalformed is true, it's a schema error
//   - If RawJSON is non-empty but parsing failed (IsTool false), it's likely a schema issue
func ValidateToolCall(tc ToolCall) ToolCallProblem {
	// If IsTool is set, Action MUST be present
	if tc.IsTool && tc.Action == "" {
		return ToolCallProblem{
			Category:   RecoveryCategorySchemaError,
			SubType:    "missing_action_field",
			RetryCount: 0,
			MaxRetries: 2,
			Retryable:  true,
			Suggestion: "Parsed tool call is missing the 'action' field",
		}
	}

	// If native args are malformed, it's a schema error
	if tc.NativeArgsMalformed {
		return ToolCallProblem{
			Category:   RecoveryCategorySchemaError,
			SubType:    "malformed_native_args",
			RetryCount: 0,
			MaxRetries: 2,
			Retryable:  true,
			Suggestion: "Native function arguments JSON is malformed",
		}
	}

	// If we have raw JSON but didn't successfully parse as a tool, that's suspicious
	// This could indicate the model sent valid JSON but with wrong field names
	if tc.RawJSON != "" && !tc.IsTool {
		// Check if RawJSON looks like a tool call but with non-standard field names
		normalized := normalizeTagsInJSON(tc.RawJSON)
		var check ToolCall
		if json.Unmarshal([]byte(normalized), &check) == nil {
			// JSON parsed but didn't have required fields
			if check.Action == "" && check.Name == "" && check.ToolCallAction == "" {
				return ToolCallProblem{
					Category:   RecoveryCategorySchemaError,
					SubType:    "unrecognized_json_structure",
					RetryCount: 0,
					MaxRetries: 2,
					Retryable:  true,
					Suggestion: "JSON parsed but no recognized tool call fields found",
				}
			}
		}
	}

	// Valid tool call
	return ToolCallProblem{}
}

// ConsolidatedRecoveryHandler replaces the 7+ separate feedback loops in agent_loop.go
// with a single unified handler. It uses the RecoveryClassifier to categorize issues
// and applies consistent retry logic.
//
// This handler MUST produce the same behavior as the original feedback loops to avoid
// regressions. The feedback messages are preserved verbatim from the original code.
type ConsolidatedRecoveryHandler struct {
	registry parseProblemRegistry
	cfg      *config.Config
	broker   FeedbackBroker
	logger   *slog.Logger
}

func newConsolidatedRecoveryHandler(cfg *config.Config, broker FeedbackBroker, logger *slog.Logger) *ConsolidatedRecoveryHandler {
	return &ConsolidatedRecoveryHandler{
		cfg:    cfg,
		broker: broker,
		logger: logger,
	}
}

// RecoveryResult contains the outcome of HandleRecovery.
type RecoveryResult struct {
	ShouldRecover bool            // true if a recovery was triggered
	Recovered     bool            // true if recovery was successful (retry happened)
	Problem       ToolCallProblem // the problem that was handled
	ContinueLoop  bool            // true if the agent loop should continue to next iteration
}

// HandleRecovery analyzes the current state and decides if a recovery should be triggered.
// It replaces the 7+ separate feedback loops in agent_loop.go:
//
// Original loops replaced:
// 1. rawCodeCount (max 2) - Raw Code Detection
// 2. xmlFallbackCount (max 2) - XML Fallback Detection
// 3. invalidNativeToolCount (max 2) - Invalid Native Tool Args
// 4. incompleteToolCallCount (max 3) - Incomplete Tool Call
// 5. announcementCount (max configurable) - Announcement-only Response
// 6. missedToolCount (max 2) - Missed Tool Call in Fence
// 7. orphanedToolCallCount (max 2) - Orphaned [TOOL_CALL] tag + Bare XML in Native Mode
func (h *ConsolidatedRecoveryHandler) HandleRecovery(
	tc ToolCall,
	content string,
	parsedToolResp ParsedToolResponse,
	useNativeFunctions bool,
	announcementContent string,
	useNativePath bool,
	lastResponseWasTool bool,
	lastUserMsg string,
	recentTools []string,
) RecoveryResult {
	result := RecoveryResult{}

	// First, classify the problem
	problem := ClassifyToolCallProblem(tc, content, parsedToolResp, useNativeFunctions)
	if problem.Category == RecoveryCategoryNone {
		return result
	}

	// Record the problem in the registry and get updated retry count
	switch problem.Category {
	case RecoveryCategoryFormatError:
		problem = h.registry.RecordFormatError(problem.SubType)
	case RecoveryCategorySchemaError:
		problem = h.registry.RecordSchemaError(problem.SubType)
	case RecoveryCategoryEmptyResponse:
		problem = h.registry.RecordEmptyResponse()
	}

	result.Problem = problem

	// Check if we've exceeded max retries for this category
	if !problem.ShouldRetry() {
		h.logger.Warn("[ConsolidatedRecovery] Max retries exceeded for category",
			"category", problem.Category.String(),
			"subtype", problem.SubType,
			"retry_count", problem.RetryCount)
		return result
	}

	result.ShouldRecover = true
	result.ContinueLoop = true

	// Generate and send the appropriate feedback message
	feedbackMsg := h.buildFeedbackMessage(problem, tc, useNativeFunctions, useNativePath, lastResponseWasTool, recentTools)
	if feedbackMsg != "" {
		h.broker.Send("error_recovery", feedbackMsg)
	}

	h.logger.Warn("[ConsolidatedRecovery] Recovery triggered",
		"category", problem.Category.String(),
		"subtype", problem.SubType,
		"retry_count", problem.RetryCount,
		"max_retries", problem.MaxRetries)

	result.Recovered = true
	return result
}

// buildFeedbackMessage generates the feedback message for a given problem.
// Messages are preserved verbatim from the original feedback loops to maintain compatibility.
func (h *ConsolidatedRecoveryHandler) buildFeedbackMessage(
	problem ToolCallProblem,
	tc ToolCall,
	useNativeFunctions bool,
	useNativePath bool,
	lastResponseWasTool bool,
	recentTools []string,
) string {
	switch problem.SubType {
	case "raw_code":
		return "ERROR: You sent raw Python code instead of a JSON tool call. My supervisor only understands JSON tool calls. Please wrap your code in a valid JSON object: {\"action\": \"save_tool\", \"name\": \"script.py\", \"description\": \"...\", \"code\": \"<your python code with \\n escaped>\"}."

	case "incomplete_tool_call":
		if useNativeFunctions {
			return "ERROR: You emitted a bare <tool_call> or <minimax:tool_call> tag but did not produce an actual tool call. You MUST use the native function-calling mechanism to invoke tools. Do NOT output any XML tags in text — use the structured function call API instead."
		}
		if problem.RetryCount >= 2 {
			return "CRITICAL ERROR: You sent '<tool_call>' as raw text again. This is not a valid tool call format. Do NOT output any XML tags at all. Output a raw JSON object starting with '{'."
		}
		return "ERROR: You emitted a bare <tool_call> tag but did not include the JSON body. Do NOT output XML tags. Output ONLY the raw JSON tool call object - no XML tags, no explanation, no preamble."

	case "orphaned_bracket_tag":
		if useNativeFunctions {
			return "ERROR: Your response contained the literal text \"[TOOL_CALL]\" but no actual function call was made. You MUST use the native function-calling mechanism to invoke a tool. Do NOT write [TOOL_CALL] as text — call the function directly using the tool call interface."
		}
		return "ERROR: Your response contained the literal text \"[TOOL_CALL]\" but no valid tool call JSON. Do NOT write [TOOL_CALL] as text. Your ENTIRE response must be ONLY the raw JSON tool call — no explanation, no tags. Output the JSON tool call NOW."

	case "bare_xml_in_native_mode":
		return "ERROR: Your response contained a literal <tool_call> XML tag but no actual function call was made. You MUST use the native function-calling mechanism — do not write XML tags. Call the function directly using the tool call interface now."

	case "xml_fallback_format":
		// This is informational - the tool will still execute
		return fmt.Sprintf(
			"NOTE: You called '%s' using a proprietary XML format (minimax:tool_call). "+
				"The tool has already been executed and the action is COMPLETE — do NOT repeat it. "+
				"Continue with the next step of the task. "+
				"For future calls, always use the native function-calling API instead. "+
				"If a tool is not in your current tool list, use discover_tools first so it can be re-added "+
				"to your active tool list on the next turn.",
			tc.Action)

	case "invalid_native_args":
		recoveryTool := tc.Action
		if strings.TrimSpace(recoveryTool) == "" {
			recoveryTool = "the requested tool"
		} else {
			recoveryTool = Truncate(strings.ReplaceAll(strings.ReplaceAll(recoveryTool, "\n", " "), "\r", " "), 80)
		}
		return fmt.Sprintf(
			"ERROR: Your last native function call for %q had invalid function arguments JSON and was discarded. Emit the function call again with valid JSON arguments only. Do not include source code, XML/HTML, or prose inside the function name or outside the JSON arguments.",
			recoveryTool)

	case "tool_in_fence":
		return "ERROR: Your response contained explanation text and/or markdown fences (```json). Tool calls MUST be a raw JSON object ONLY - no explanation before or after, no markdown, no fences. Output ONLY the JSON object, starting with { and ending with }. Example: {\"action\": \"co_agent\", \"operation\": \"spawn\", \"task\": \"...\"}"

	case "announcement_only":
		msg := "ERROR: You announced what you were going to do but did not output a tool call. When executing a task, your ENTIRE response must be ONLY the raw JSON tool call — no explanation before it. Output the JSON tool call NOW."
		if lastResponseWasTool && len(recentTools) > 0 {
			lastTool := recentTools[len(recentTools)-1]
			msg += fmt.Sprintf(" IMPORTANT: '%s' already completed successfully in this turn. Do NOT call it again. Your next action must be a DIFFERENT tool that continues your plan.", lastTool)
		}
		msg += " If the task is genuinely complete and no more tool calls are needed, state the final result to the user and append <done/> at the very end."
		return msg
	}

	return ""
}
