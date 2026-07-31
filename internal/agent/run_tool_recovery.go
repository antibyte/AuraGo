package agent

import (
	"encoding/json"
	"sort"
	"strings"

	"aurago/internal/tools"
)

const maxRunToolRecoveryNames = 10

type runToolRecoverySuggestion struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
}

type runToolNotFoundResult struct {
	Status               string                     `json:"status"`
	ErrorCode            string                     `json:"error_code"`
	Message              string                     `json:"message"`
	AvailableCustomTools []string                   `json:"available_custom_tools"`
	Suggestion           *runToolRecoverySuggestion `json:"suggestion,omitempty"`
}

func runToolNotFoundOutput(manifest *tools.Manifest, requestedName string) string {
	names := availableCustomToolNames(manifest, requestedName, maxRunToolRecoveryNames)
	result := runToolNotFoundResult{
		Status:               "error",
		ErrorCode:            "CUSTOM_TOOL_NOT_FOUND",
		Message:              "Saved custom tool not found. Use discover_tools or list_tools, then call the exact returned name. Built-in AuraGo tools must not be called through run_tool.",
		AvailableCustomTools: names,
		Suggestion:           legacyRunToolSuggestion(requestedName),
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return `Tool Output: {"status":"error","error_code":"CUSTOM_TOOL_NOT_FOUND","message":"Saved custom tool not found. Use discover_tools before retrying."}`
	}
	return "Tool Output: " + string(payload)
}

func availableCustomToolNames(manifest *tools.Manifest, excludedName string, limit int) []string {
	if manifest == nil || limit <= 0 {
		return []string{}
	}
	excludedName = strings.TrimSpace(excludedName)
	loaded, err := manifest.Load()
	if err != nil {
		return []string{}
	}
	names := make([]string, 0, len(loaded))
	for name := range loaded {
		name = strings.TrimSpace(name)
		if name != "" && name != excludedName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > limit {
		names = names[:limit]
	}
	return names
}

func legacyRunToolSuggestion(requestedName string) *runToolRecoverySuggestion {
	normalized := strings.ToLower(strings.TrimSpace(requestedName))
	normalized = strings.TrimSuffix(normalized, ".py")
	normalized = strings.NewReplacer("-", "_", " ", "_").Replace(normalized)
	switch normalized {
	case "save_note", "create_note", "add_note":
		return &runToolRecoverySuggestion{
			ToolName:  "manage_notes",
			Arguments: map[string]any{"operation": "add"},
		}
	case "list_open_tasks", "open_tasks", "list_tasks":
		return &runToolRecoverySuggestion{
			ToolName:  "manage_todos",
			Arguments: map[string]any{"operation": "list", "status": "open"},
		}
	default:
		return nil
	}
}
