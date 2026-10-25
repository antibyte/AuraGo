package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/prompts"

	openai "github.com/sashabaranov/go-openai"
)

func discoverToolsOutput(t *testing.T, cfg *config.Config, sid string, params map[string]interface{}) (DiscoverToolsResponse, string) {
	t.Helper()
	out := handleDiscoverTools(ToolCall{Params: params}, cfg, nil, sid)
	var response DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &response)
	return response, out
}

func TestDiscoveryCursorSentinelsAndStaleCursorReset(t *testing.T) {
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	var schemas []openai.Tool
	for i := 0; i < 8; i++ {
		schemas = append(schemas, testToolSchema(fmt.Sprintf("cursor_fixture_%02d", i), "fixture"))
	}
	SetDiscoverToolsState(sid, schemas, nil, "")
	search := func(cursor string) (DiscoverToolsResponse, string) {
		return discoverToolsOutput(t, &config.Config{}, sid, map[string]interface{}{"operation": "search", "query": "cursor_fixture", "cursor": cursor})
	}

	var first DiscoverToolsResponse
	for _, sentinel := range []string{"None", "null", "undefined", "0", " NONE "} {
		page, out := search(sentinel)
		if page.Status != "success" || page.CursorReset || len(page.Results) != 5 || page.NextCursor == "" {
			t.Fatalf("cursor %q must mean no cursor: %s", sentinel, out)
		}
		first = page
	}
	if page, out := search("abc"); page.Status != "success" || !page.CursorReset || len(page.Results) == 0 || page.Results[0].Name != "cursor_fixture_00" {
		t.Fatalf("an undecodable cursor must restart at the first page: %s", out)
	}

	// Only Active changes here; the adaptive filter does this on every iteration.
	SetDiscoverToolsState(sid, schemas, schemas[:3], "")
	if page, out := search(first.NextCursor); page.CursorReset || len(page.Results) == 0 || page.Results[0].Name != "cursor_fixture_05" {
		t.Fatalf("a cursor must survive an active-set change: %s", out)
	}
}

func TestDiscoveryZeroHitsSuggestAndUnknownCategory(t *testing.T) {
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	schemas := []openai.Tool{testToolSchema("docker", "Manage containers"), testToolSchema("filesystem", "Read and write files")}
	SetDiscoverToolsState(sid, schemas, schemas, "")

	search, out := discoverToolsOutput(t, &config.Config{}, sid, map[string]interface{}{"operation": "search", "query": "dokcer"})
	if search.Status != "success" || !search.NoResults || len(search.Suggestions) == 0 || len(search.Suggestions) > 3 || search.Suggestions[0] != "docker" {
		t.Fatalf("a misspelled query must succeed with suggestions: %s", out)
	}
	if !strings.Contains(search.Summary, "list_categories") || strings.Contains(search.Summary, "get_tool_info") {
		t.Fatalf("zero-hit advice must point at list_categories, not get_tool_info: %s", out)
	}
	if classifyLegacyToolResult(out) != ToolResultSuccess {
		t.Fatalf("a zero-hit search must not be classified as a failure: %s", out)
	}

	info, out := discoverToolsOutput(t, &config.Config{}, sid, map[string]interface{}{"operation": "get_tool_info", "tool_name": "dokcer"})
	if info.Error != "unknown_or_ambiguous_tool" || len(info.Suggestions) == 0 || info.Suggestions[0] != "docker" {
		t.Fatalf("an unknown tool name must carry suggestions: %s", out)
	}

	family, out := discoverToolsOutput(t, &config.Config{}, sid, map[string]interface{}{"operation": "list_family", "category": "nope"})
	if family.Error != "unknown_category" || len(family.ValidCategories) == 0 || !sort.StringsAreSorted(family.ValidCategories) {
		t.Fatalf("an unknown category must list the valid ones: %s", out)
	}
}

func TestGetManualPageFillsBudget(t *testing.T) {
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	promptsDir := t.TempDir()
	path := filepath.Join(promptsDir, "tools_manuals", "fixture_tool.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// Quotes, angle brackets and a multi-byte letter inflate when JSON-encoded.
	var text strings.Builder
	for i := 0; i < 40; i++ {
		line := fmt.Sprintf("%02d <b>\"quoted\"</b> Grüße ", i)
		text.WriteString(line + strings.Repeat("x", 89-len(line)) + "\n")
	}
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	SetDiscoverToolsState(sid, []openai.Tool{testToolSchema("fixture_tool", "fixture")}, nil, promptsDir)
	cfg := &config.Config{}
	cfg.Directories.PromptsDir = promptsDir
	cfg.Agent.ToolOutputLimit = 2000

	var pages []string
	cursor := ""
	for i := 0; i < 50; i++ {
		page, out := discoverToolsOutput(t, cfg, sid, map[string]interface{}{"operation": "get_manual", "tool_name": "fixture_tool", "cursor": cursor})
		if page.Status != "success" || len(out) > 2000 {
			t.Fatalf("page %d is not a success within the budget (%d bytes): %s", i, len(out), out)
		}
		if i == 0 && len(out) < 1200 {
			t.Fatalf("the first page must fill the budget, got %d bytes", len(out))
		}
		pages = append(pages, page.Manual)
		if page.NextCursor == "" {
			break
		}
		if !strings.HasSuffix(page.Manual, "\n") {
			t.Fatalf("page %d must end at a line break: %q", i, page.Manual)
		}
		cursor = page.NextCursor
	}
	want, ok := prompts.ReadToolGuideFull(path)
	if !ok || strings.Join(pages, "") != want {
		t.Fatalf("the pages must add up to the manual (%d pages)", len(pages))
	}
	if len(pages) < 2 {
		t.Fatal("the fixture must need more than one page")
	}
}
