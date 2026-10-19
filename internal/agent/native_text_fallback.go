package agent

import (
	"encoding/json"
	"strings"
)

// exactJSONToolAnswer accepts only an entire structured tool answer. Examples,
// prose, fences, reasoning and XML wrappers cannot become native-mode calls.
func exactJSONToolAnswer(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" || !json.Valid([]byte(content)) {
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(content), &object) == nil && object != nil {
		for _, key := range []string{"action", "tool", "tool_name", "tool_calls", "tool_call"} {
			if _, exists := object[key]; exists {
				return true
			}
		}
		return object["name"] != nil && object["arguments"] != nil
	}
	var calls []json.RawMessage
	if json.Unmarshal([]byte(content), &calls) != nil || len(calls) == 0 {
		return false
	}
	for _, call := range calls {
		if !exactJSONToolAnswer(string(call)) {
			return false
		}
	}
	return true
}
