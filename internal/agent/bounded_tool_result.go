package agent

import (
	"aurago/internal/security"
	"encoding/json"
	"html"
	"strings"
)

// boundedToolResult never clips JSON in the middle of a token. A large opaque
// result becomes an explicit bounded envelope, preserving the execution status.
func boundedToolResult(output string, limit int, status ToolResultStatus) string {
	if limit <= 0 || len(output) <= limit {
		return output
	}
	value, isolated := toolResultPayload(output)
	wrap := func(data []byte) string {
		if isolated {
			return security.IsolateExternalData(string(data))
		}
		return string(data)
	}
	if json.Valid([]byte(value)) || isolated {
		payload := map[string]interface{}{"status": status, "truncated": true, "message": "Result exceeds the output budget. Request a smaller page or more specific operation."}
		var original map[string]interface{}
		if json.Unmarshal([]byte(value), &original) == nil {
			for _, key := range []string{"code", "error_code", "output_ref", "tool_call_id"} {
				if v, ok := original[key].(string); ok && len(v) <= 256 {
					payload[key] = v
				}
			}
		}
		if status.IsError() {
			if message := extractErrorMessage(value); message != "" {
				payload["message"] = truncateUTF8ToLimit(message, 180, "")
			}
		}
		for _, remove := range []string{"", "message", "tool_call_id", "output_ref", "error_code", "code"} {
			delete(payload, remove)
			data, _ := json.Marshal(payload)
			if bounded := wrap(data); len(bounded) <= limit {
				return bounded
			}
		}
		// No external content remains in this generated minimal envelope.
		data, _ := json.Marshal(map[string]interface{}{"status": status, "truncated": true})
		if len(data) <= limit {
			return string(data)
		}
		if limit == 1 {
			return "0"
		}
		return "{}"
	}
	return truncateToolOutput(output, limit)
}

func toolResultPayload(output string) (string, bool) {
	value := strings.TrimSpace(output)
	isolated := strings.HasPrefix(value, "<external_data>\n") && strings.HasSuffix(value, "\n</external_data>")
	if isolated {
		value = html.UnescapeString(strings.TrimSuffix(strings.TrimPrefix(value, "<external_data>\n"), "\n</external_data>"))
	}
	for _, prefix := range []string{"[Tool Output]", "Tool Output:"} {
		value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	return value, isolated
}
