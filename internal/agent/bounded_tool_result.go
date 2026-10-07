package agent

import (
	"aurago/internal/security"
	"encoding/json"
	"strings"
)

// boundedToolResult never clips JSON in the middle of a token. A large opaque
// result becomes an explicit bounded envelope, preserving the execution status.
func boundedToolResult(output string, limit int, status ToolResultStatus) string {
	if limit <= 0 || len(output) <= limit {
		return output
	}
	value, isolated, raw := toolResultPayloadForm(output)
	prefix := toolResultPresentationPrefix(output)
	wrap := func(data []byte) string {
		if isolated {
			return prefix + isolateToolPayload(string(data), raw)
		}
		return prefix + string(data)
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
		if len(prefix)+len(data) <= limit {
			return prefix + string(data)
		}
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

func toolResultPresentationPrefix(output string) string {
	value := strings.TrimSpace(output)
	if strings.HasPrefix(value, "[Tool Output]") {
		return "[Tool Output]\n"
	}
	if strings.HasPrefix(value, "Tool Output:") {
		return "Tool Output: "
	}
	return ""
}

func toolResultPresentationSuffix(output string) string {
	if end := strings.Index(output, "\n</external_data>"); end >= 0 {
		return output[end+len("\n</external_data>"):]
	}
	return ""
}

func toolResultPayload(output string) (string, bool) {
	value, isolated, _ := toolResultPayloadForm(output)
	return value, isolated
}

// isolateToolPayload re-wraps an extracted payload in the form it arrived in:
// readable source isolation stays readable, escaped isolation stays escaped.
func isolateToolPayload(payload string, raw bool) string {
	if raw {
		return security.IsolateSourceData(payload)
	}
	return security.IsolateExternalData(payload)
}

// toolResultPayloadForm also reports whether an isolated body was readable
// source isolation (raw) rather than the fully escaped form.
func toolResultPayloadForm(output string) (string, bool, bool) {
	value := strings.TrimSpace(output)
	for {
		before := value
		for _, prefix := range []string{"[Tool Output]", "Tool Output:"} {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
		if before == value {
			break
		}
	}
	if strings.HasPrefix(value, "<external_data>\n") {
		if end := strings.Index(value, "\n</external_data>"); end >= 0 {
			// Recovery guidance may follow the trusted presentation envelope.
			// It is replaceable when bounding; never clip the isolated body.
			payload, raw := security.IsolatedPayload(value[len("<external_data>\n"):end])
			return payload, true, raw
		}
	}
	return value, false, false
}
