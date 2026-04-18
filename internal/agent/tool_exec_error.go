package agent

import (
	"encoding/json"
	"fmt"
)

// ToolExecError is a structured execution error for tool-dispatch paths that
// need stable machine-readable error output without a full error-system rewrite.
type ToolExecError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Retryable bool                   `json:"retryable"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func (e ToolExecError) Error() string {
	return e.Message
}

func formatToolExecError(err ToolExecError) string {
	payload := map[string]interface{}{
		"status":    "error",
		"code":      err.Code,
		"message":   err.Message,
		"retryable": err.Retryable,
	}
	if len(err.Details) > 0 {
		payload["details"] = err.Details
	}
	encoded, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","code":"%s","message":"%s","retryable":%t}`,
			err.Code, err.Message, err.Retryable)
	}
	return "Tool Output: " + string(encoded)
}

func newUnexpectedBuiltinActionToolExecError(action string) ToolExecError {
	return ToolExecError{
		Code:      "builtin_handler_missing",
		Message:   fmt.Sprintf("builtin tool handler missing implementation for %q", action),
		Retryable: false,
		Details: map[string]interface{}{
			"action": action,
		},
	}
}
