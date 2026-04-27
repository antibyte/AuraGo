package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/prompts"

	openai "github.com/sashabaranov/go-openai"
)

type DiscoverToolsResponse struct {
	Status   string               `json:"status"`
	Summary  string               `json:"summary,omitempty"`
	Results  []DiscoverToolResult `json:"results,omitempty"`
	Tool     *DiscoverToolResult  `json:"tool,omitempty"`
	Manual   string               `json:"manual,omitempty"`
	Error    string               `json:"error,omitempty"`
	Category string               `json:"category,omitempty"`
}

type DiscoverToolResult struct {
	Name            string                 `json:"name"`
	Kind            string                 `json:"kind"`
	ToolStatus      string                 `json:"status"`
	Description     string                 `json:"description,omitempty"`
	CallMethod      string                 `json:"call_method"`
	CallableNow     bool                   `json:"callable_now"`
	SchemaAvailable bool                   `json:"schema_available"`
	HiddenReason    string                 `json:"hidden_reason,omitempty"`
	Category        string                 `json:"category,omitempty"`
	Instruction     string                 `json:"instruction"`
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
}

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
	catalog := GetToolCatalogState()
	if catalog == nil {
		catalog = BuildToolCatalog(allSchemas, schemasFromActiveNames(allSchemas, activeNames), cfg.Directories.PromptsDir)
	}

	switch op {
	case "list_categories":
		category := strings.TrimSpace(stringValueFromMap(tc.Params, "category"))
		logger.Info("[DiscoverTools] list_categories", "category", category)
		return discoverToolsJSON(DiscoverToolsResponse{
			Status:   "success",
			Category: category,
			Summary:  FormatToolCategories(category, activeNames, enabledNames),
			Results:  discoverResultsFromEntries(catalog.ByCategory(category), sessionID, false),
		})

	case "search":
		query := strings.TrimSpace(stringValueFromMap(tc.Params, "query"))
		if query == "" {
			return "Tool Output: ERROR 'query' is required for search."
		}
		resolvedQuery := resolveDiscoverToolName(query)
		logger.Info("[DiscoverTools] search", "query", query)
		results := catalog.Search(resolvedQuery)
		if len(results) == 0 {
			return discoverToolsJSON(DiscoverToolsResponse{
				Status: "error",
				Error:  fmt.Sprintf("No tools found matching '%s'. Use list_categories to browse all tools.", query),
			})
		}
		return discoverToolsJSON(DiscoverToolsResponse{
			Status:  "success",
			Summary: fmt.Sprintf("%d tools matching '%s'", len(results), query),
			Results: discoverResultsFromEntries(results, sessionID, true),
		})

	case "get_tool_info":
		toolName := strings.TrimSpace(stringValueFromMap(tc.Params, "tool_name"))
		if toolName == "" {
			return "Tool Output: ERROR 'tool_name' is required for get_tool_info."
		}
		resolvedToolName := resolveDiscoverToolName(toolName)
		logger.Info("[DiscoverTools] get_tool_info", "tool", toolName)
		if entry, ok := catalog.Get(resolvedToolName); ok {
			guidePath := entry.ManualPath
			guide, _ := prompts.ReadToolGuide(guidePath)
			result := discoverResultFromEntry(entry, sessionID, true)
			return discoverToolsJSON(DiscoverToolsResponse{
				Status: "success",
				Tool:   &result,
				Manual: guide,
			})
		}
		groupResults := catalog.Search(resolvedToolName)
		enabledGroupResults := make([]*ToolCatalogEntry, 0, len(groupResults))
		for _, entry := range groupResults {
			if entry.Enabled {
				enabledGroupResults = append(enabledGroupResults, entry)
			}
		}
		if len(enabledGroupResults) > 0 {
			return discoverToolsJSON(DiscoverToolsResponse{
				Status:  "success",
				Summary: fmt.Sprintf("Tool family '%s' has %d enabled tools", resolvedToolName, len(enabledGroupResults)),
				Results: discoverResultsFromEntries(enabledGroupResults, sessionID, true),
			})
		}

		// Load tool guide
		toolsDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")
		guidePath := filepath.Join(toolsDir, resolvedToolName+".md")
		guide, _ := prompts.ReadToolGuide(guidePath)

		info := FormatToolInfo(resolvedToolName, allSchemas, guide)
		if !toolSchemaExists(resolvedToolName, allSchemas) {
			if groupInfo := formatDiscoverToolGroupInfo(resolvedToolName, activeNames, enabledNames, sessionID); groupInfo != "" {
				return groupInfo
			}
		}
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

func discoverToolsJSON(resp DiscoverToolsResponse) string {
	b, _ := json.Marshal(resp)
	return "Tool Output: " + string(b)
}

func discoverResultsFromEntries(entries []*ToolCatalogEntry, sessionID string, markHidden bool) []DiscoverToolResult {
	results := make([]DiscoverToolResult, 0, len(entries))
	for _, entry := range entries {
		results = append(results, discoverResultFromEntry(entry, sessionID, markHidden))
	}
	return results
}

func discoverResultFromEntry(entry *ToolCatalogEntry, sessionID string, markHidden bool) DiscoverToolResult {
	if entry == nil {
		return DiscoverToolResult{}
	}
	if markHidden && entry.Enabled && entry.Status == ToolStatusHidden && entry.Kind == ToolKindNative {
		MarkDiscoverRequestedTool(sessionID, entry.Name)
	}
	method := callMethodForEntry(entry)
	params := schemaParameters(entry.Schema)
	return DiscoverToolResult{
		Name:            entry.Name,
		Kind:            string(entry.Kind),
		ToolStatus:      string(entry.Status),
		Description:     entry.Description,
		CallMethod:      method,
		CallableNow:     entry.Enabled && (entry.Status == ToolStatusActive || method == "invoke_tool" || method == "execute_skill" || method == "run_tool"),
		SchemaAvailable: entry.Schema.Function != nil,
		HiddenReason:    entry.HiddenReason,
		Category:        entry.Category,
		Instruction:     callInstructionForEntry(entry, method),
		Parameters:      params,
	}
}

func callInstructionForEntry(entry *ToolCatalogEntry, method string) string {
	if entry == nil {
		return ""
	}
	switch method {
	case "invoke_tool":
		return fmt.Sprintf("Use invoke_tool with tool_name=%q and arguments matching the parameters shown here. Do not use execute_skill for native tools.", entry.Name)
	case "execute_skill":
		return fmt.Sprintf("Use execute_skill with skill=%q and skill_args matching the parameters shown here.", entry.Routing.SkillName)
	case "run_tool":
		return fmt.Sprintf("Use run_tool with name=%q and params matching the parameters shown here.", entry.Routing.CustomName)
	case "direct":
		return fmt.Sprintf("Call the native tool %q directly.", entry.Name)
	default:
		return "Tool is not callable."
	}
}

func schemaParameters(schema openai.Tool) map[string]interface{} {
	if schema.Function == nil {
		return nil
	}
	if params, ok := schema.Function.Parameters.(map[string]interface{}); ok {
		return params
	}
	return nil
}

func schemasFromActiveNames(allSchemas []openai.Tool, activeNames map[string]bool) []openai.Tool {
	var out []openai.Tool
	for _, schema := range allSchemas {
		if schema.Function != nil && activeNames[schema.Function.Name] {
			out = append(out, schema)
		}
	}
	return out
}

func toolSchemaExists(toolName string, schemas []openai.Tool) bool {
	for _, s := range schemas {
		if s.Function != nil && s.Function.Name == toolName {
			return true
		}
	}
	return false
}

func formatDiscoverToolGroupInfo(query string, activeNames, enabledNames map[string]bool, sessionID string) string {
	results := SearchToolsInCategories(query)
	if len(results) == 0 {
		return ""
	}

	var enabledResults []struct {
		Category string
		Entry    ToolCategoryEntry
	}
	for _, r := range results {
		if enabledNames[r.Entry.Name] {
			enabledResults = append(enabledResults, r)
		}
	}
	if len(enabledResults) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tool Output:\nTool family '%s' has %d enabled tools:\n", query, len(enabledResults)))
	for _, r := range enabledResults {
		status := "○"
		if activeNames[r.Entry.Name] {
			status = "●"
		} else {
			MarkDiscoverRequestedTool(sessionID, r.Entry.Name)
		}
		sb.WriteString(fmt.Sprintf("  %s %s [%s] - %s\n", status, r.Entry.Name, r.Category, r.Entry.ShortDesc))
	}
	sb.WriteString("\n● = active   ○ = enabled but hidden by adaptive filtering")
	sb.WriteString("\nUse get_tool_info with one exact tool name above, then call that exact tool.")
	sb.WriteString("\n[STATUS] These enabled tools were requested and will be re-included on the next agent turn.")
	return sb.String()
}
