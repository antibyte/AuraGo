package agent

import (
	"encoding/json"
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
