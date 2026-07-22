package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

func normalizedAllowedToolSet(tools []string) map[string]struct{} {
	if tools == nil {
		return nil
	}
	result := make(map[string]struct{}, len(tools))
	for _, name := range tools {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			result[name] = struct{}{}
		}
	}
	return result
}

func filterSchemasByAllowedTools(schemas []openai.Tool, allowed []string) []openai.Tool {
	if allowed == nil {
		return schemas
	}
	set := normalizedAllowedToolSet(allowed)
	filtered := make([]openai.Tool, 0, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil {
			continue
		}
		if _, ok := set[strings.ToLower(strings.TrimSpace(schema.Function.Name))]; ok {
			filtered = append(filtered, schema)
		}
	}
	return filtered
}

func dispatchToolAllowed(dc *DispatchContext, name string) bool {
	if dc == nil || !dc.ToolScopeRestricted {
		return true
	}
	_, ok := dc.AllowedTools[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func toolScopeDeniedOutput(name string) string {
	encoded, _ := json.Marshal(fmt.Sprintf("tool %q is not allowed in this agent run", name))
	return fmt.Sprintf(`Tool Output: {"status":"error","message":%s,"code":"tool_scope_denied"}`, encoded)
}
