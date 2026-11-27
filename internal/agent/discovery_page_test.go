package agent

import (
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestDiscoverySummaryResultsKeepRealInstructionForUncallableTools(t *testing.T) {
	catalog := BuildToolCatalog([]openai.Tool{testToolSchema("yepapi_instagram", "Instagram data via YepAPI")}, nil, "")
	catalog.add(&ToolCatalogEntry{
		Name: "fixture_needs_setup", Kind: ToolKindNative, Status: ToolStatusNeedsSetup, Enabled: true,
		HiddenReason: "runtime_missing", Description: "Fixture without a runtime",
	})
	disabled, ok := catalog.Get("yepapi_youtube")
	if !ok {
		t.Fatal("expected a disabled categorized entry")
	}
	hidden, _ := catalog.Get("yepapi_instagram")
	setup, _ := catalog.Get("fixture_needs_setup")

	results := discoverySummaryResults([]*ToolCatalogEntry{hidden, disabled, setup}, "sess-summary-instruction", false)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if got := results[0].Instruction; !strings.Contains(got, "invoke_tool with tool_name=yepapi_instagram") {
		t.Fatalf("callable result instruction = %q, want the invoke_tool hint", got)
	}
	if got := discoverySummaryResults([]*ToolCatalogEntry{hidden}, "sess-summary-instruction", true)[0].Instruction; !strings.Contains(got, "get_tool_info") || !strings.Contains(got, "queues") {
		t.Fatalf("re-injecting run instruction = %q, want get_tool_info that queues the schema", got)
	}
	if got := results[1].Instruction; !strings.Contains(got, "disabled in config") || strings.Contains(got, "get_tool_info") {
		t.Fatalf("disabled result instruction = %q, want the disabled explanation", got)
	}
	if got := results[2].Instruction; !strings.Contains(got, "hidden_reason") || strings.Contains(got, "get_tool_info") {
		t.Fatalf("needs_setup result instruction = %q, want the setup explanation", got)
	}
}
