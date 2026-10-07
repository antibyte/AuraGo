package agent

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type discoveryCursor struct {
	Revision string `json:"r"`
	Query    string `json:"q"`
	Offset   int    `json:"o"`
}

func catalogRevision(catalog *ToolCatalog) string {
	h := sha256.New()
	for _, entry := range catalog.Entries() {
		data, _ := json.Marshal(entry)
		h.Write(data)
		data, _ = json.Marshal(entry.Schema)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func discoveryOffset(tc ToolCall, revision, query string) (int, string) {
	raw := stringValueFromMap(tc.Params, "cursor")
	if raw == "" {
		return 0, ""
	}
	if len(raw) > 2048 {
		return 0, "invalid_cursor"
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	var cursor discoveryCursor
	if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Offset < 0 || cursor.Query != query {
		return 0, "invalid_cursor"
	}
	if cursor.Revision != revision {
		return 0, "catalog_changed"
	}
	return cursor.Offset, ""
}

func nextDiscoveryCursor(revision, query string, offset int) string {
	data, _ := json.Marshal(discoveryCursor{revision, query, offset})
	return base64.RawURLEncoding.EncodeToString(data)
}

func discoveryPage(tc ToolCall, results []DiscoverToolResult, revision, query string, budget int) string {
	offset, err := discoveryOffset(tc, revision, query)
	if err != "" || offset > len(results) {
		if err == "" {
			err = "invalid_cursor"
		}
		return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "error", Error: err}, budget)
	}
	count := maxDiscoverToolSearchResults
	if raw := tc.Params["limit"]; raw != nil {
		if n, err := strconv.Atoi(fmt.Sprint(raw)); err == nil && n > 0 && n < count {
			count = n
		}
	}
	end := min(len(results), offset+count)
	for {
		page := DiscoverToolsResponse{Category: stringValueFromMap(tc.Params, "category", "category_name", "cat"), Status: "success", Revision: revision, Total: len(results), Results: results[offset:end], Summary: fmt.Sprintf("showing %d of %d tools; use get_tool_info for schema details", end-offset, len(results))}
		if end < len(results) {
			page.NextCursor = nextDiscoveryCursor(revision, query, end)
		}
		data, _ := json.Marshal(page)
		if len(data)+13 <= budget {
			return "Tool Output: " + string(data)
		}
		if end <= offset+1 {
			return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "error", Error: "output_budget_too_small", Revision: revision}, budget)
		}
		end--
	}
}

func discoveryJSONWithinBudget(response DiscoverToolsResponse, budget int) string {
	output := discoverToolsJSON(response)
	if len(output) <= budget {
		return output
	}
	return boundedToolResult(output, budget, classifyLegacyToolResult(output))
}

func discoverySummaryResults(entries []*ToolCatalogEntry, session string) []DiscoverToolResult {
	results := discoverResultsFromEntries(entries, session, false)
	for i := range results {
		results[i].Parameters = nil
		results[i].Description = truncateUTF8ToLimit(results[i].Description, 240, "…")
		results[i].Instruction = "Use get_tool_info before the first call."
	}
	return results
}

func discoveryQueryKey(operation, query string) string {
	sum := sha256.Sum256([]byte(operation + "\x00" + strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:8])
}

func discoveryCategoryPage(tc ToolCall, categories []DiscoverCategory, revision string, budget int) string {
	key := discoveryQueryKey("list_categories", "")
	offset, err := discoveryOffset(tc, revision, key)
	if err != "" || offset > len(categories) {
		if err == "" {
			err = "invalid_cursor"
		}
		return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "error", Error: err}, budget)
	}
	end := min(offset+20, len(categories))
	for {
		page := DiscoverToolsResponse{Status: "success", Revision: revision, Total: len(categories), Categories: categories[offset:end], Summary: "Use list_family or search; get_tool_info returns the schema."}
		if end < len(categories) {
			page.NextCursor = nextDiscoveryCursor(revision, key, end)
		}
		if output := discoverToolsJSON(page); len(output) <= budget {
			return output
		}
		if end <= offset+1 {
			return discoveryJSONWithinBudget(DiscoverToolsResponse{Status: "error", Error: "output_budget_too_small"}, budget)
		}
		end--
	}
}
