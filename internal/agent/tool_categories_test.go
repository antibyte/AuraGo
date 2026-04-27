package agent

import (
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestToolCategoryOrderMatchesDefs(t *testing.T) {
	for _, cat := range toolCategoryOrder {
		if _, ok := toolCategoryDef[cat]; !ok {
			t.Errorf("category %q in toolCategoryOrder but not in toolCategoryDef", cat)
		}
		if _, ok := toolCategoryLabels[cat]; !ok {
			t.Errorf("category %q missing label in toolCategoryLabels", cat)
		}
	}
	for cat := range toolCategoryDef {
		found := false
		for _, c := range toolCategoryOrder {
			if c == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("category %q in toolCategoryDef but not in toolCategoryOrder", cat)
		}
	}
}

func TestToolCategoryCount(t *testing.T) {
	if len(toolCategoryOrder) != 8 {
		t.Errorf("expected 8 categories, got %d", len(toolCategoryOrder))
	}
}

func TestGetToolCategory(t *testing.T) {
	tests := []struct {
		tool    string
		wantCat string
	}{
		{"docker", "infrastructure"},
		{"home_assistant", "smart_home"},
		{"generate_music", "media"},
		{"api_request", "network"},
		{"filesystem", "files"},
		{"execute_shell", "system"},
		{"send_email", "communication"},
		{"yepapi_instagram", "data_apis"},
		{"nonexistent_tool", ""},
	}
	for _, tt := range tests {
		got := GetToolCategory(tt.tool)
		if got != tt.wantCat {
			t.Errorf("GetToolCategory(%q) = %q, want %q", tt.tool, got, tt.wantCat)
		}
	}
}

func TestSearchToolsInCategories(t *testing.T) {
	results := SearchToolsInCategories("docker")
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'docker'")
	}
	found := false
	for _, r := range results {
		if r.Entry.Name == "docker" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'docker' in search results for 'docker'")
	}

	// Search by description keyword
	results = SearchToolsInCategories("DNS")
	if len(results) == 0 {
		t.Fatal("expected results for 'DNS'")
	}

	results = SearchToolsInCategories("koofr")
	if len(results) == 0 {
		t.Fatal("expected results for 'koofr'")
	}

	results = SearchToolsInCategories("browser")
	if len(results) == 0 {
		t.Fatal("expected results for 'browser'")
	}
	found = false
	for _, r := range results {
		if r.Entry.Name == "browser_automation" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'browser_automation' in search results for 'browser'")
	}

	results = SearchToolsInCategories("wikipedia")
	if len(results) == 0 {
		t.Fatal("expected results for 'wikipedia'")
	}
	found = false
	for _, r := range results {
		if r.Entry.Name == "wikipedia_search" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'wikipedia_search' in search results for 'wikipedia'")
	}

	results = SearchToolsInCategories("mcp")
	if len(results) == 0 {
		t.Fatal("expected results for 'mcp'")
	}
	found = false
	for _, r := range results {
		if r.Entry.Name == "mcp_call" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'mcp_call' in search results for 'mcp'")
	}

	results = SearchToolsInCategories("instagram")
	if len(results) == 0 {
		t.Fatal("expected results for 'instagram'")
	}
	found = false
	for _, r := range results {
		if r.Entry.Name == "yepapi_instagram" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'yepapi_instagram' in search results for 'instagram'")
	}

	results = SearchToolsInCategories("yepapi")
	if len(results) == 0 {
		t.Fatal("expected results for 'yepapi'")
	}
	found = false
	for _, r := range results {
		if r.Entry.Name == "yepapi_instagram" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'yepapi_instagram' in search results for 'yepapi'")
	}

	// No results
	results = SearchToolsInCategories("zzz_nonexistent_xyz")
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent query, got %d", len(results))
	}
}

func TestFormatToolCategories_AllCategories(t *testing.T) {
	active := map[string]bool{"docker": true, "filesystem": true}
	enabled := map[string]bool{"docker": true, "filesystem": true, "proxmox": true, "generate_music": true}
	output := FormatToolCategories("", active, enabled)

	if !strings.Contains(output, "● docker") {
		t.Error("expected active docker to show ● marker")
	}
	if !strings.Contains(output, "○ proxmox") {
		t.Error("expected hidden proxmox to show ○ marker")
	}
	if !strings.Contains(output, "● = active in context") {
		t.Error("expected legend line")
	}
}

func TestFormatToolCategories_SingleCategory(t *testing.T) {
	active := map[string]bool{}
	enabled := map[string]bool{"docker": true, "proxmox": true}
	output := FormatToolCategories("infrastructure", active, enabled)

	if !strings.Contains(output, "Infrastructure & DevOps") {
		t.Error("expected infrastructure label")
	}
	// Should not contain other categories
	if strings.Contains(output, "Smart Home") {
		t.Error("should not contain other categories when filtering by one")
	}
}

func TestFormatToolCategories_InvalidCategory(t *testing.T) {
	output := FormatToolCategories("nonexistent", nil, nil)
	if !strings.Contains(output, "Unknown category") {
		t.Error("expected error message for invalid category")
	}
}

func TestFormatToolInfo_Found(t *testing.T) {
	schemas := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "test_tool",
				Description: "A test tool for unit tests",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Op to perform",
							"enum":        []interface{}{"list", "get"},
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "The name param",
						},
					},
					"required": []interface{}{"operation"},
				},
			},
		},
	}

	output := FormatToolInfo("test_tool", schemas, "This is the guide content.")
	if !strings.Contains(output, "# test_tool") {
		t.Error("expected tool name header")
	}
	if !strings.Contains(output, "A test tool for unit tests") {
		t.Error("expected description")
	}
	if !strings.Contains(output, "operation") {
		t.Error("expected parameter listing")
	}
	if !strings.Contains(output, "(required)") {
		t.Error("expected required marker")
	}
	if !strings.Contains(output, "list, get") {
		t.Error("expected enum values")
	}
	if !strings.Contains(output, "Tool Guide") {
		t.Error("expected guide section")
	}
}

func TestFormatToolInfo_NotFound(t *testing.T) {
	output := FormatToolInfo("missing_tool", nil, "")
	if !strings.Contains(output, "not found") {
		t.Error("expected not-found message")
	}
}

func TestResolveDiscoverToolNameAlias(t *testing.T) {
	if got := resolveDiscoverToolName("mcp"); got != "mcp_call" {
		t.Fatalf("resolveDiscoverToolName(mcp) = %q, want %q", got, "mcp_call")
	}
	if got := resolveDiscoverToolName("mcp server"); got != "mcp_call" {
		t.Fatalf("resolveDiscoverToolName(mcp server) = %q, want %q", got, "mcp_call")
	}
	if got := resolveDiscoverToolName("wikipedia"); got != "wikipedia_search" {
		t.Fatalf("resolveDiscoverToolName(wikipedia) = %q, want %q", got, "wikipedia_search")
	}
	if got := resolveDiscoverToolName("ddg"); got != "ddg_search" {
		t.Fatalf("resolveDiscoverToolName(ddg) = %q, want %q", got, "ddg_search")
	}
	if got := resolveDiscoverToolName("chromecast"); got != "chromecast" {
		t.Fatalf("resolveDiscoverToolName(chromecast) = %q, want %q", got, "chromecast")
	}
}

func TestNoDuplicateToolsAcrossCategories(t *testing.T) {
	seen := make(map[string]string) // tool_name → category
	for cat, entries := range toolCategoryDef {
		for _, entry := range entries {
			if prevCat, dup := seen[entry.Name]; dup {
				t.Errorf("tool %q appears in both %q and %q", entry.Name, prevCat, cat)
			}
			seen[entry.Name] = cat
		}
	}
}
