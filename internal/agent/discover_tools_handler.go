package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/prompts"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

const maxDiscoverToolSearchResults = 5

type DiscoverToolsResponse struct {
	Revision   string             `json:"revision,omitempty"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Total      int                `json:"total,omitempty"`
	Categories []DiscoverCategory `json:"categories,omitempty"`

	Status   string               `json:"status"`
	Summary  string               `json:"summary,omitempty"`
	Results  []DiscoverToolResult `json:"results,omitempty"`
	Tool     *DiscoverToolResult  `json:"tool,omitempty"`
	Manual   string               `json:"manual,omitempty"`
	Error    string               `json:"error,omitempty"`
	Category string               `json:"category,omitempty"`
}

type DiscoverCategory struct {
	Purpose string `json:"purpose,omitempty"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
}

type DiscoverToolResult struct {
	ReadOnly        bool                   `json:"read_only,omitempty"`
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
	return handleDiscoverToolsContext(context.Background(), tc, cfg, logger, sessionID, nil)
}
func handleDiscoverToolsContext(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, sessionID string, dc *DispatchContext) string {
	op := strings.TrimSpace(stringValueFromMap(tc.Params, "operation", "op"))
	catalog := GetToolCatalogState(sessionID)
	if catalog == nil {
		return discoverToolsJSON(DiscoverToolsResponse{Status: "error", Error: "catalog_unavailable"})
	}
	budget := 6000
	if cfg != nil && cfg.Agent.ToolOutputLimit > 0 {
		budget = min(budget, cfg.Agent.ToolOutputLimit)
	}
	revision := catalogRevision(catalog)
	category := strings.TrimSpace(stringValueFromMap(tc.Params, "category", "category_name", "cat"))
	if op == "list_categories" && category != "" {
		op = "list_family"
	}
	switch op {
	case "list_categories":
		counts := map[string]int{}
		for _, entry := range catalog.Entries() {
			counts[entry.Category]++
		}
		categories := make([]DiscoverCategory, 0, len(counts))
		for name, count := range counts {
			purpose := strings.ReplaceAll(name, "_", " ")
			if entries := catalog.ByCategory(name); len(entries) > 0 {
				purpose = truncateUTF8ToLimit(entries[0].Description, 80, "...")
			}
			categories = append(categories, DiscoverCategory{Name: name, Count: count, Purpose: purpose})
		}
		sort.Slice(categories, func(i, j int) bool { return categories[i].Name < categories[j].Name })
		return discoveryCategoryPage(tc, categories, revision, min(budget, 4000))
	case "search", "list_family":
		query := truncateUTF8ToLimit(strings.TrimSpace(stringValueFromMap(tc.Params, "query", "q", "search_query", "keyword")), 512, "")
		var results []DiscoverToolResult
		if op == "list_family" {
			query = category
			results = discoverySummaryResults(catalog.ByCategory(category), sessionID)
		} else {
			if query == "" {
				return discoverToolsJSON(DiscoverToolsResponse{Status: "error", Error: "query_required"})
			}
			entries := catalog.Search(query)
			if dc != nil {
				entries = catalog.SearchContext(ctx, query, dc.LongTermMem)
			}
			results = discoverySummaryResults(entries, sessionID)
			if entry, ok := catalog.Get("composio_call"); ok && entry.Enabled {
				results = append(results, discoverComposioServiceSearchResults(cfg, query)...)
			}
		}
		data, _ := json.Marshal(results)
		revision = fmt.Sprintf("%x", sha256.Sum256(append([]byte(revision), data...)))[:16]
		return discoveryPage(tc, results, revision, discoveryQueryKey(op, query), budget)
	case "get_tool_info":
		name := strings.TrimSpace(stringValueFromMap(tc.Params, "tool_name", "name", "tool"))
		if entry, ok := catalog.Get(resolveDiscoverToolName(name)); ok {
			result := discoverResultFromEntry(entry, sessionID, true)
			response := DiscoverToolsResponse{Status: "success", Revision: revision, Tool: &result, Summary: "Use get_manual to retrieve workflow guidance separately."}
			budget = 16000
			if cfg != nil && cfg.Agent.ToolOutputLimit > 0 {
				budget = min(budget, cfg.Agent.ToolOutputLimit)
			}
			encoded := discoverToolsJSON(response)
			if len(encoded) > budget {
				return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "error", Error: "schema_exceeds_output_budget", Summary: "The complete input schema does not fit this output budget. Increase tool_output_limit before using this tool."}, budget)
			}
			return encoded
		}
		if entry, ok := catalog.Get("composio_call"); ok && entry.Enabled {
			if service, ok := discoverComposioServiceResult(cfg, name); ok {
				return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "success", Tool: &service}, budget)
			}
		}
		return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "error", Error: "unknown_or_ambiguous_tool", Summary: "Use search to choose an exact namespaced tool ID."}, budget)
	case "get_manual":
		name := strings.TrimSpace(stringValueFromMap(tc.Params, "tool_name", "name", "tool"))
		entry, ok := catalog.Get(name)
		if !ok {
			return discoverToolsJSON(DiscoverToolsResponse{Status: "error", Error: "tool_not_found"})
		}
		guide, ok := prompts.ReadToolGuideFull(entry.ManualPath)
		if !ok {
			return discoverToolsJSON(DiscoverToolsResponse{Status: "error", Error: "manual_unavailable"})
		}
		key := discoveryQueryKey(op, name)
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(guide)))[:16]
		offset, err := discoveryOffset(tc, digest, key)
		if err != "" || offset > len(guide) {
			return discoverToolsJSON(DiscoverToolsResponse{Status: "error", Error: "manual_cursor_invalid_or_changed"})
		}
		part := truncateUTF8Prefix(guide[offset:], max(1, (budget-600)/6))
		response := DiscoverToolsResponse{Status: "success", Revision: digest, Manual: part}
		if offset+len(part) < len(guide) {
			response.NextCursor = nextDiscoveryCursor(digest, key, offset+len(part))
		}
		return discoveryJSONWithinBudget(response, budget)
	default:
		return discoverToolsJSON(DiscoverToolsResponse{Status: "error", Error: "unknown_operation", Summary: "Use list_categories, list_family, search, get_tool_info or get_manual."})
	}
}

func discoverComposioServiceSearchResults(cfg *config.Config, query string) []DiscoverToolResult {
	var results []DiscoverToolResult
	for _, slug := range selectedComposioToolkitSlugs(cfg) {
		if composioServiceMatchesQuery(slug, query) {
			results = append(results, composioServiceResult(cfg, slug))
		}
	}
	return results
}

func discoverComposioServiceResult(cfg *config.Config, name string) (DiscoverToolResult, bool) {
	for _, slug := range selectedComposioToolkitSlugs(cfg) {
		if composioServiceMatchesQuery(slug, name) {
			return composioServiceResult(cfg, slug), true
		}
	}
	return DiscoverToolResult{}, false
}

func selectedComposioToolkitSlugs(cfg *config.Config) []string {
	if cfg == nil || !cfg.Composio.Enabled || strings.TrimSpace(cfg.Composio.APIKey) == "" {
		return nil
	}
	seen := make(map[string]bool, len(cfg.Composio.Toolkits))
	slugs := make([]string, 0, len(cfg.Composio.Toolkits))
	for _, tk := range cfg.Composio.Toolkits {
		slug := strings.ToLower(strings.TrimSpace(tk.Slug))
		if slug == "" || !tk.Enabled || seen[slug] {
			continue
		}
		seen[slug] = true
		slugs = append(slugs, slug)
	}
	return slugs
}

func composioServiceMatchesQuery(slug, query string) bool {
	slug = strings.ToLower(strings.TrimSpace(slug))
	query = strings.ToLower(strings.TrimSpace(query))
	if slug == "" || query == "" {
		return false
	}
	if strings.HasPrefix(query, "composio:") {
		query = strings.TrimSpace(strings.TrimPrefix(query, "composio:"))
	}
	if query == slug || strings.Contains(query, slug) {
		return true
	}
	for _, alias := range composioServiceAliases(slug) {
		if query == alias || strings.Contains(query, alias) {
			return true
		}
	}
	return false
}

func composioServiceAliases(slug string) []string {
	switch slug {
	case "gmail":
		return []string{"google mail", "googlemail", "g mail"}
	default:
		return nil
	}
}

func composioServiceResult(cfg *config.Config, slug string) DiscoverToolResult {
	state := tools.ComposioCachedConnectionState(cfg.Composio, slug)
	readOnly := cfg.Composio.ReadOnly
	for _, toolkit := range cfg.Composio.Toolkits {
		if strings.EqualFold(toolkit.Slug, slug) && toolkit.ReadOnly != nil {
			readOnly = *toolkit.ReadOnly
			break
		}
	}
	return DiscoverToolResult{
		Name:            "composio:" + slug,
		Kind:            "composio_service",
		ToolStatus:      state,
		ReadOnly:        readOnly,
		Description:     fmt.Sprintf("Use the selected Composio %s toolkit through composio_call.", slug),
		CallMethod:      "composio_call",
		CallableNow:     state == "connected",
		SchemaAvailable: false,
		Category:        "data_apis",
		Instruction: fmt.Sprintf("Use composio_call with {\"operation\":\"capabilities\",\"toolkit_slug\":\"%s\"} or {\"operation\":\"list_connected_accounts\",\"toolkit_slug\":\"%s\"}; then use search_tools, get_tool, and execute_tool with \"toolkit_slug\":\"%s\". If a narrow search is empty, retry broadly through composio_call; do not switch to direct third-party APIs for this selected service.",
			slug, slug, slug),
	}
}

func composioServiceManual(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.Directories.PromptsDir) != "" {
		if guide, ok := prompts.ReadToolGuide(filepath.Join(cfg.Directories.PromptsDir, "tools_manuals", "composio_call.md")); ok && strings.TrimSpace(guide) != "" {
			return guide
		}
	}
	return "# composio_call\nUse composio_call to inspect capabilities, list connected accounts, search tools, get tool schemas, and execute user-approved Composio tools."
}

func discoverToolsJSON(resp DiscoverToolsResponse) string {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Sprintf(`Tool Output: {"status":"error","message":"failed to encode response: %s"}`, err)
	}
	return "Tool Output: " + string(b)
}

func discoverToolsLogInfo(logger *slog.Logger, msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
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
		ReadOnly:        entry.ReadOnly,
		Name:            entry.Name,
		Kind:            string(entry.Kind),
		ToolStatus:      string(entry.Status),
		Description:     entry.Description,
		CallMethod:      method,
		CallableNow:     entry.Enabled && (entry.Status == ToolStatusActive || method == "invoke_tool" || method == "execute_skill" || method == "run_tool" || method == "activate_agent_skill"),
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
	case "activate_agent_skill":
		return fmt.Sprintf("Use activate_agent_skill with name=%q to load the package instructions and resources. Scripts remain subject to the active skill scope.", entry.Routing.SkillName)
	case "invoke_tool":
		return fmt.Sprintf("Use invoke_tool with tool_name=%q and arguments matching the parameters shown here. Do not use execute_skill for native tools.", entry.Name)
	case "execute_skill":
		return fmt.Sprintf("Use execute_skill with skill=%q and skill_args matching the parameters shown here.", entry.Routing.SkillName)
	case "run_tool":
		return fmt.Sprintf("Use run_tool with name=%q and params matching the parameters shown here.", entry.Routing.CustomName)
	case "direct":
		return fmt.Sprintf("Call the native tool %q directly.", entry.Name)
	case "disabled":
		if entry.HiddenReason == "config_disabled" {
			return "Tool is disabled in config and cannot be called until the user enables it."
		}
		return "Tool is unavailable under the current configuration or run policy. Check hidden_reason; discovery cannot grant permission."
	case "needs_setup":
		return "The configured tool has no usable local runtime dependency. Resolve hidden_reason before calling it."
	default:
		return "Tool is not callable."
	}
}

func schemaParameters(schema openai.Tool) map[string]interface{} {
	if schema.Function == nil || schema.Function.Parameters == nil {
		return nil
	}
	if params, ok := schema.Function.Parameters.(map[string]interface{}); ok {
		return params
	}
	raw, err := json.Marshal(schema.Function.Parameters)
	if err != nil {
		return nil
	}
	var params map[string]interface{}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	return params
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

func formatDiscoverToolGroupInfo(query string, activeNames, enabledNames map[string]bool) string {
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
		}
		sb.WriteString(fmt.Sprintf("  %s %s [%s] - %s\n", status, r.Entry.Name, r.Category, r.Entry.ShortDesc))
	}
	sb.WriteString("\n● = active   ○ = enabled but hidden by adaptive filtering")
	sb.WriteString("\nUse get_tool_info with one exact tool name above, then call that exact tool.")
	return sb.String()
}
