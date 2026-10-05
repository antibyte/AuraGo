package agent

import (
	"aurago/internal/security"
	"encoding/json"
	"strings"
)

// boundedToolResult never clips JSON in the middle of a token. A large opaque
// result becomes an explicit bounded envelope, preserving the execution status.
// Readable execution output keeps its head instead (truncateExecutionOutput).
func boundedToolResult(action, output string, limit int, status ToolResultStatus) string {
	if limit <= 0 || len(output) <= limit {
		return output
	}
	if bounded, ok := truncateExecutionOutput(action, output, limit, truncateToolOutput); ok {
		return bounded
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

// isolationBoundaryLen is what IsolateSourceData adds around a body.
const isolationBoundaryLen = len("<external_data>\n") + len("\n</external_data>")

// truncateExecutionOutput keeps the head of oversized execution output inside
// its boundary, as plain-text truncation did before execution output was
// always isolated. It applies to readable bodies only: the raw source form, or
// the escaped form whose text has no quote or angle character (the form
// IsolateSourceData itself picks for such text). An escaped body with those
// characters was escaped on purpose (scanner hit or boundary-like text) and,
// like JSON and the output of every other tool, keeps the never-clip envelope.
// The decoded text is truncated and re-isolated, so nothing is escaped twice;
// the presentation prefix and any trailing guidance are kept when they fit.
func truncateExecutionOutput(action, output string, limit int, truncate func(string, int) string) (string, bool) {
	if !security.IsExecutionToolOutput(action) {
		return "", false
	}
	payload, isolated, raw := toolResultPayloadForm(output)
	if !isolated || (!raw && strings.ContainsAny(payload, `"'<>`)) {
		return "", false
	}
	if value, _ := toolResultPayload(payload); json.Valid([]byte(value)) {
		return "", false
	}
	prefix := toolResultPresentationPrefix(output)
	for _, suffix := range []string{toolResultPresentationSuffix(output), ""} {
		budget := limit - len(prefix) - isolationBoundaryLen - len(suffix)
		// Escaping can lengthen the re-isolated text; shrink by the overflow.
		for attempt := 0; attempt < 8 && budget > 0; attempt++ {
			bounded := prefix + security.IsolateSourceData(truncate(payload, budget)) + suffix
			if len(bounded) <= limit {
				return bounded, true
			}
			budget -= len(bounded) - limit
		}
	}
	return "", false
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
