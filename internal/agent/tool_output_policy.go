package agent

import "fmt"

type toolOutputPolicyResult struct {
	Content      string
	Truncated    bool
	WasError     bool
	ErrorSummary string
}

func applyToolOutputPolicy(result string, limit int, scope AgentTelemetryScope) toolOutputPolicyResult {
	decision := toolOutputPolicyResult{
		Content:      result,
		WasError:     isToolError(result),
		ErrorSummary: extractErrorMessage(result),
	}
	if limit <= 0 || len(result) <= limit {
		return decision
	}

	decision.Truncated = true
	RecordToolRecoveryEventForScope(scope, "tool_output_truncated")

	if decision.WasError {
		RecordToolRecoveryEventForScope(scope, "error_output_truncated_preserved")
		decision.Content = truncateToolErrorPreserving(result, limit, decision.ErrorSummary)
		return decision
	}

	decision.Content = truncateToolOutput(result, limit)
	return decision
}

// truncateToolOutput trims a tool result that exceeds limit characters.
// It keeps the first portion of the output and appends a clear notice so the
// LLM knows the result was cut. limit=0 means no truncation.
func truncateToolOutput(result string, limit int) string {
	if limit <= 0 || len(result) <= limit {
		return result
	}
	notice := fmt.Sprintf("\n\n[Tool output truncated: %d of %d characters shown. Use a more specific command to get less output.]", limit, len(result))
	if len(notice) >= limit {
		return truncateUTF8ToLimit(notice, limit, "")
	}
	return truncateUTF8Prefix(result, limit-len(notice)) + notice
}

func truncateToolErrorPreserving(result string, limit int, errorSummary string) string {
	if limit <= 0 || len(result) <= limit {
		return result
	}
	if errorSummary == "" {
		errorSummary = "Tool reported an error, but no structured message was available."
	}

	summary := Truncate(errorSummary, 240)
	notice := fmt.Sprintf("\n\n[Tool output truncated: %d of %d characters shown. Error status preserved.]", limit, len(result))
	summaryHeader := "\n\n[Preserved error summary]\n"
	summaryBlock := summaryHeader + summary
	suffix := notice + summaryBlock
	if len(suffix) > limit {
		available := limit - len(notice) - len(summaryHeader)
		if available <= 0 {
			return truncateUTF8ToLimit(notice, limit, "")
		}
		summary = truncateUTF8Prefix(errorSummary, available)
		summaryBlock = summaryHeader + summary
		suffix = notice + summaryBlock
		if len(suffix) > limit {
			return truncateUTF8ToLimit(suffix, limit, "")
		}
	}

	headLimit := limit - len(suffix)
	if headLimit <= 0 {
		return suffix
	}
	return truncateUTF8Prefix(result, headLimit) + suffix
}
