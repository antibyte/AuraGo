package agent

import (
	"encoding/json"
	"strings"
)

// A manual is a documentation family, not an executed action. Dynamic bridges
// additionally need their effective target to keep cohorts comparable.
func toolTraceActionIdentity(tc ToolCall) string {
	action := strings.ToLower(strings.TrimSpace(tc.Action))
	parts := []string{action}
	switch action {
	case "mcp_call":
		req := decodeMCPCallArgs(tc)
		parts = append(parts, req.Server, req.ToolName)
	case "execute_skill":
		parts = append(parts, firstNonEmptyToolString(tc.Skill, stringValueFromMap(tc.Params, "skill", "name")))
	case "run_tool":
		parts = append(parts, firstNonEmptyToolString(tc.Name, stringValueFromMap(tc.Params, "name", "tool_name")))
	case "composio_call":
		parts = append(parts, decodeComposioCallArgs(tc).ToolSlug)
	default:
		return action
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return "" // Unresolved target is never comparison evidence.
		}
	}
	encoded, _ := json.Marshal(parts)
	return string(encoded)
}
