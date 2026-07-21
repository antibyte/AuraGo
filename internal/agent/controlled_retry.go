package agent

import (
	"encoding/json"
	"strings"

	"aurago/internal/security"
)

type controlledRetryReport struct {
	ToolName     string                 `json:"tool_name"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Context      string                 `json:"context"`
	RetryOutcome string                 `json:"retry_outcome"`
}

func appendControlledRetryReport(content string, route currentToolRoute, call ToolCall, userContext string, failed bool) string {
	outcome := "succeeded"
	errorText := ""
	if failed {
		outcome = "failed"
		errorText = sanitizeControlledRetryText(extractErrorMessage(content), 360)
		if errorText == "" {
			errorText = "The retried tool call failed without a safe structured error message."
		}
	}
	params := cloneSafeRetryParameters(route.Parameters)
	if len(params) == 0 {
		params = safeControlledRetryCallParameters(call)
	}
	report := controlledRetryReport{
		ToolName:     strings.TrimSpace(call.Action),
		Parameters:   params,
		Error:        errorText,
		Context:      sanitizeControlledRetryText(userContext, 240),
		RetryOutcome: outcome,
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return content
	}
	return strings.TrimSpace(content) + "\n\n[CONTROLLED RETRY REPORT]\n" + string(encoded)
}

func safeControlledRetryCallParameters(call ToolCall) map[string]interface{} {
	params := cloneSafeRetryParameters(call.Params)
	if params == nil {
		params = make(map[string]interface{})
	}
	for key, value := range map[string]string{
		"operation": call.Operation,
		"stream_id": toolArgString(call.Params, "stream_id"),
		"file_path": firstNonEmptyToolString(call.FilePath, call.Path),
		"prompt":    call.Prompt,
		"query":     call.Query,
	} {
		if strings.TrimSpace(value) != "" {
			params[key] = value
		}
	}
	return cloneSafeRetryParameters(params)
}

func sanitizeControlledRetryText(value string, maxChars int) string {
	value = security.RedactSensitiveInfo(security.Scrub(value))
	var safeLines []string
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" || strings.Contains(lower, "_guardian_justification") ||
			strings.Contains(lower, "suggested next step") || strings.Contains(lower, "secret environment") ||
			strings.HasPrefix(lower, "guardian instruction") || strings.HasPrefix(lower, "policy instruction") {
			continue
		}
		safeLines = append(safeLines, trimmed)
	}
	return Truncate(strings.Join(strings.Fields(strings.Join(safeLines, " ")), " "), maxChars)
}
