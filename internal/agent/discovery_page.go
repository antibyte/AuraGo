package agent

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type discoveryCursor struct {
	Revision string `json:"r"`
	Query    string `json:"q"`
	Offset   int    `json:"o"`
}

// discoveryCursorResetNotice tells the model that its cursor was not honored,
// so it does not mistake the first page for a continuation.
const discoveryCursorResetNotice = "The cursor was invalid or the catalog changed; showing the first page again."

// catalogRevision hashes what a cursor must stay valid against: the set of
// entries and their schemas. Whether an entry is currently in the active tool
// list (Active, active/hidden status, the adaptive_filter reason) flips with
// every adaptive-filter pass, so a normalized copy is hashed instead.
func catalogRevision(catalog *ToolCatalog) string {
	h := sha256.New()
	for _, entry := range catalog.Entries() {
		stable := *entry
		stable.Active = false
		if stable.Status == ToolStatusActive || stable.Status == ToolStatusHidden {
			stable.Status = ToolStatusHidden
		}
		if stable.HiddenReason == "adaptive_filter" {
			stable.HiddenReason = ""
		}
		data, _ := json.Marshal(stable)
		h.Write(data)
		data, _ = json.Marshal(entry.Schema)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// discoveryOffset decodes the request cursor. Models send "None", "null" or 0
// for "no cursor"; those start at the first page. reset reports a cursor that
// cannot be honored (garbage, another query, another catalog revision): the
// caller restarts at offset 0 and says so instead of failing the call.
func discoveryOffset(tc ToolCall, revision, query string) (offset int, reset bool) {
	raw := strings.TrimSpace(stringValueFromMap(tc.Params, "cursor"))
	switch strings.ToLower(raw) {
	case "", "none", "null", "nil", "undefined", "0":
		return 0, false
	}
	if len(raw) > 2048 {
		return 0, true
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	var cursor discoveryCursor
	if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Offset < 0 || cursor.Query != query || cursor.Revision != revision {
		return 0, true
	}
	return cursor.Offset, false
}

func nextDiscoveryCursor(revision, query string, offset int) string {
	data, _ := json.Marshal(discoveryCursor{revision, query, offset})
	return base64.RawURLEncoding.EncodeToString(data)
}

func discoveryPage(tc ToolCall, results []DiscoverToolResult, revision, query string, budget int) string {
	offset, reset := discoveryOffset(tc, revision, query)
	if offset > len(results) {
		offset, reset = 0, true
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
		if reset {
			page.CursorReset, page.Notice = true, discoveryCursorResetNotice
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
		// A disabled or unconfigured tool keeps the reason it cannot be called;
		// "get_tool_info before the first call" would promise a call that fails.
		if method := results[i].CallMethod; method != "disabled" && method != "needs_setup" {
			results[i].Instruction = "Use get_tool_info before the first call."
		}
	}
	return results
}

func discoveryQueryKey(operation, query string) string {
	sum := sha256.Sum256([]byte(operation + "\x00" + strings.TrimSpace(query)))
	return hex.EncodeToString(sum[:8])
}

func discoveryCategoryPage(tc ToolCall, categories []DiscoverCategory, revision string, budget int) string {
	key := discoveryQueryKey("list_categories", "")
	offset, reset := discoveryOffset(tc, revision, key)
	if offset > len(categories) {
		offset, reset = 0, true
	}
	end := min(offset+20, len(categories))
	for {
		page := DiscoverToolsResponse{Status: "success", Revision: revision, Total: len(categories), Categories: categories[offset:end], Summary: "Use list_family or search; get_tool_info returns the schema."}
		if end < len(categories) {
			page.NextCursor = nextDiscoveryCursor(revision, key, end)
		}
		if reset {
			page.CursorReset, page.Notice = true, discoveryCursorResetNotice
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

// discoveryManualPage returns the manual page starting at offset, filled up to
// budget by its encoded JSON size (escaping inflates the manual text, so the
// raw length alone says nothing). A page that does not end the manual is cut
// at its last line break when that keeps at least half of it. ok is false when
// not even one character fits; the caller then reports output_budget_too_small.
func discoveryManualPage(guide string, offset int, reset bool, digest, key string, budget int) (page string, ok bool) {
	rest := guide[offset:]
	n := min(len(rest), budget)
	for {
		part := truncateUTF8Prefix(rest, n)
		if len(part) < len(rest) {
			if part == "" {
				return "", false
			}
			if i := strings.LastIndexByte(part, '\n'); i >= len(part)/2 {
				part = part[:i+1]
			}
		}
		response := DiscoverToolsResponse{Status: "success", Revision: digest, Manual: part}
		if offset+len(part) < len(guide) {
			response.NextCursor = nextDiscoveryCursor(digest, key, offset+len(part))
		}
		if reset {
			response.CursorReset, response.Notice = true, discoveryCursorResetNotice
		}
		out := discoverToolsJSON(response)
		if len(out) <= budget {
			return out, true
		}
		// Drop the overshoot from the raw text; escaped characters make this
		// conservative, so the loop converges within a few rounds.
		n = min(len(part)-(len(out)-budget), len(part)-1)
		if n <= 0 {
			return "", false
		}
	}
}

// discoveryValidCategories lists the category names list_family accepts, sorted.
func discoveryValidCategories(catalog *ToolCatalog) []string {
	seen := map[string]bool{}
	var names []string
	for _, entry := range catalog.Entries() {
		if entry.Category != "" && !seen[entry.Category] {
			seen[entry.Category] = true
			names = append(names, entry.Category)
		}
	}
	sort.Strings(names)
	return names
}
