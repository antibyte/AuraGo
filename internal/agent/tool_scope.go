package agent

import (
	"encoding/json"
	"fmt"
	"slices"
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
		name = canonicalNativeAlias(name)
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
		if _, ok := set[canonicalNativeAlias(strings.ToLower(strings.TrimSpace(schema.Function.Name)))]; ok {
			filtered = append(filtered, schema)
		}
	}
	return filtered
}

func filterSchemasByName(schemas []openai.Tool, excluded string) []openai.Tool {
	excluded = strings.ToLower(strings.TrimSpace(excluded))
	filtered := make([]openai.Tool, 0, len(schemas))
	for _, schema := range schemas {
		if schema.Function != nil && strings.EqualFold(strings.TrimSpace(schema.Function.Name), excluded) {
			continue
		}
		filtered = append(filtered, schema)
	}
	return filtered
}

func dispatchToolAllowed(dc *DispatchContext, name string) bool {
	if dc == nil || !dc.ToolScopeRestricted {
		return true
	}
	name = canonicalNativeAlias(strings.ToLower(strings.TrimSpace(name)))
	for allowed := range dc.AllowedTools {
		if canonicalNativeAlias(strings.ToLower(strings.TrimSpace(allowed))) == name {
			return true
		}
	}
	return false
}

func toolScopeDeniedOutput(name string) string {
	encoded, _ := json.Marshal(fmt.Sprintf("tool %q is not allowed in this agent run", name))
	return fmt.Sprintf(`Tool Output: {"status":"error","message":%s,"code":"tool_scope_denied"}`, encoded)
}

// scopeRecoveryMaxTools bounds allowed_tools in a scope refusal.
const scopeRecoveryMaxTools = 12

// scopeRecoveryFields tells a model refused by a restricted tool scope what the
// run may call instead: the sorted, bounded allow-list and a one-line hint. A
// Game Maker operation name whose owning tool is outside the scope (validate
// while planning) is traced to that owner. Both scope refusals read the run's
// dc.AllowedTools, the set dispatchToolAllowed decides on. The list is nil
// without a restricted scope, which leaves the refusal untouched.
func scopeRecoveryFields(dc *DispatchContext, denied string) ([]string, string) {
	if dc == nil || !dc.ToolScopeRestricted {
		return nil, ""
	}
	names := []string{}
	studio := false
	for name := range dc.AllowedTools {
		if name = canonicalNativeAlias(strings.ToLower(strings.TrimSpace(name))); name != "" && !slices.Contains(names, name) {
			names = append(names, name)
			studio = studio || strings.HasPrefix(name, "game_maker_")
		}
	}
	slices.Sort(names)
	if len(names) > scopeRecoveryMaxTools {
		names = append(names[:scopeRecoveryMaxTools:scopeRecoveryMaxTools], "…")
	}
	owner := gameMakerOperationOwners()[denied]
	switch {
	case len(names) == 0:
		return names, "No tool is available in this run; answer without a tool call."
	case studio && owner != "" && !dispatchToolAllowed(dc, owner):
		return names, fmt.Sprintf("%s is an operation of %s, which is not available in this phase. Call one of allowed_tools by its exact name.", echoedToolName(denied), owner)
	case studio:
		return names, "Call one of allowed_tools by its exact name; operations such as set_design go in the operation argument."
	}
	return names, "Call one of allowed_tools by its exact name; operations go in the operation argument."
}

// toolScopeDeniedFor is toolScopeDeniedOutput for a run: under a restricted
// scope the refusal also carries allowed_tools and a hint. The existing fields
// and an unrestricted run's output are unchanged.
func toolScopeDeniedFor(dc *DispatchContext, name string) string {
	output := toolScopeDeniedOutput(name)
	allowed, hint := scopeRecoveryFields(dc, name)
	if allowed == nil {
		return output
	}
	list, _ := json.Marshal(allowed)
	note, _ := json.Marshal(hint)
	return strings.TrimSuffix(output, "}") + `,"allowed_tools":` + string(list) + `,"hint":` + string(note) + "}"
}

// scopeDeniedRefusal is the action-policy form of toolScopeDeniedFor.
func scopeDeniedRefusal(dc *DispatchContext, name string) *ToolDispatchResult {
	allowed, hint := scopeRecoveryFields(dc, name)
	if allowed == nil {
		return actionPolicyRefusal("tool_scope_denied", "The tool is outside this run's scope.", ToolResultDenied)
	}
	encoded, _ := json.Marshal(map[string]interface{}{
		"status": string(ToolResultDenied), "code": "tool_scope_denied", "message": "The tool is outside this run's scope.",
		"allowed_tools": allowed, "hint": hint,
	})
	return &ToolDispatchResult{Output: "Tool Output: " + string(encoded), Status: ToolResultDenied, IsError: true}
}
