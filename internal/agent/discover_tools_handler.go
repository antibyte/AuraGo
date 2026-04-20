package agent

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/prompts"
)

// handleDiscoverTools dispatches discover_tools operations (list_categories, search, get_tool_info).
func handleDiscoverTools(tc ToolCall, cfg *config.Config, logger *slog.Logger, sessionID string) string {
	op := strings.TrimSpace(stringValueFromMap(tc.Params, "operation"))
	if op == "" {
		return "Tool Output: ERROR 'operation' is required. Use list_categories, search, or get_tool_info."
	}

	allSchemas, activeNames, enabledNames, _ := GetDiscoverToolsState()
	if len(allSchemas) == 0 {
		return "Tool Output: Tool state not available yet. Try again after the first agent turn."
	}

	switch op {
	case "list_categories":
		category := strings.TrimSpace(stringValueFromMap(tc.Params, "category"))
		logger.Info("[DiscoverTools] list_categories", "category", category)
		return "Tool Output:\n" + FormatToolCategories(category, activeNames, enabledNames)

	case "search":
		query := strings.TrimSpace(stringValueFromMap(tc.Params, "query"))
		if query == "" {
			return "Tool Output: ERROR 'query' is required for search."
		}
		resolvedQuery := resolveDiscoverToolName(query)
		logger.Info("[DiscoverTools] search", "query", query)
		results := SearchToolsInCategories(resolvedQuery)
		if len(results) == 0 {
			return fmt.Sprintf("Tool Output: No tools found matching '%s'. Use list_categories to browse all tools.", query)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Tool Output: %d tools matching '%s':\n", len(results), query))
		for _, r := range results {
			status := "○"
			if activeNames[r.Entry.Name] {
				status = "●"
			}
			if !enabledNames[r.Entry.Name] {
				status = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s %s [%s] — %s\n", status, r.Entry.Name, r.Category, r.Entry.ShortDesc))
		}
		sb.WriteString("\n● = active   ○ = available (hidden)   ✗ = disabled in config")
		sb.WriteString("\nUse get_tool_info to see full parameters for any available tool, then call it directly.")
		return sb.String()

	case "get_tool_info":
		toolName := strings.TrimSpace(stringValueFromMap(tc.Params, "tool_name"))
		if toolName == "" {
			return "Tool Output: ERROR 'tool_name' is required for get_tool_info."
		}
		resolvedToolName := resolveDiscoverToolName(toolName)
		logger.Info("[DiscoverTools] get_tool_info", "tool", toolName)

		// Load tool guide
		toolsDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")
		guidePath := filepath.Join(toolsDir, resolvedToolName+".md")
		guide, _ := prompts.ReadToolGuide(guidePath)

		info := FormatToolInfo(resolvedToolName, allSchemas, guide)
		active := activeNames[resolvedToolName]
		var hint string
		if active {
			hint = "\n\n[STATUS] This tool is currently active in your tool list. You can call it directly."
		} else if enabledNames[resolvedToolName] {
			MarkDiscoverRequestedTool(sessionID, resolvedToolName)
			hint = "\n\n[STATUS] This tool is currently hidden by adaptive filtering but enabled. You can call it directly by name using the parameters shown above."
		} else {
			hint = "\n\n[STATUS] This tool is disabled in config. It cannot be used until enabled by the user."
		}
		if resolvedToolName != toolName {
			hint = fmt.Sprintf("\n\n[ALIAS] '%s' maps to '%s'.", toolName, resolvedToolName) + hint
		}
		return "Tool Output:\n" + info + hint

	default:
		return fmt.Sprintf("Tool Output: ERROR Unknown operation '%s'. Use list_categories, search, or get_tool_info.", op)
	}
}
