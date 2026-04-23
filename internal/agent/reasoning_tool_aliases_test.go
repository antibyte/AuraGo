package agent

import (
	"testing"

	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

func TestNormalizeParsedToolShortcut_ConvertsSkillShortcutToExecuteSkill(t *testing.T) {
	tc := ToolCall{
		IsTool: true,
		Action: "skill__brave_search",
		SkillArgs: map[string]interface{}{
			"query": "Wetter Pforzheim aktuell April 2026",
		},
	}

	got := normalizeParsedToolShortcut(tc)

	if got.Action != "execute_skill" {
		t.Fatalf("Action = %q, want execute_skill", got.Action)
	}
	if got.Skill != "brave_search" {
		t.Fatalf("Skill = %q, want brave_search", got.Skill)
	}
	if query, _ := got.SkillArgs["query"].(string); query != "Wetter Pforzheim aktuell April 2026" {
		t.Fatalf("SkillArgs[query] = %q, want weather query", query)
	}
	if query, _ := got.Params["query"].(string); query != "Wetter Pforzheim aktuell April 2026" {
		t.Fatalf("Params[query] = %q, want weather query", query)
	}
}

func TestKnownReasoningExtractedActionSet_IncludesBuiltinActions(t *testing.T) {
	known := knownReasoningExtractedActionSet(nil, nil)

	for _, action := range []string{"brave_search", "web_scraper", "wikipedia_search", "ddg_search", "obsidian", "execute_skill"} {
		if _, ok := known[action]; !ok {
			t.Fatalf("expected builtin action %q in known reasoning action set", action)
		}
	}
}

func TestKnownReasoningExtractedActionSet_IncludesSkillShortcutUnderlyingAction(t *testing.T) {
	currentTools := []openai.Tool{
		makeTool("skill__brave_search"),
	}

	known := knownReasoningExtractedActionSet(currentTools, nil)

	if _, ok := known["skill__brave_search"]; !ok {
		t.Fatal("expected skill__brave_search alias in known reasoning action set")
	}
	if _, ok := known["brave_search"]; !ok {
		t.Fatal("expected underlying brave_search action in known reasoning action set")
	}
}

func TestKnownReasoningExtractedActionSet_IncludesManifestCustomToolNames(t *testing.T) {
	toolsDir := t.TempDir()
	manifest := tools.NewManifest(toolsDir)
	if err := manifest.Register("my_custom_tool.py", "Custom tool"); err != nil {
		t.Fatalf("register custom tool: %v", err)
	}

	known := knownReasoningExtractedActionSet(nil, manifest)

	if _, ok := known["my_custom_tool.py"]; !ok {
		t.Fatal("expected manifest custom tool name in known reasoning action set")
	}
	if _, ok := known["tool__my_custom_tool.py"]; !ok {
		t.Fatal("expected tool__ manifest alias in known reasoning action set")
	}
}

func TestShouldAcceptParsedTextToolCallsInNativeMode_AcceptsActiveContentJSONTool(t *testing.T) {
	currentTools := []openai.Tool{
		makeTool("skill__brave_search"),
		makeTool("document_creator"),
	}

	ok := shouldAcceptParsedTextToolCallsInNativeMode(
		currentTools,
		ToolCallParseSourceContentJSON,
		ToolCall{IsTool: true, Action: "brave_search"},
		[]ToolCall{{IsTool: true, Action: "document_creator"}},
	)

	if !ok {
		t.Fatal("expected active content_json tool calls to be accepted in native mode")
	}
}

func TestShouldAcceptParsedTextToolCallsInNativeMode_RejectsUnknownAction(t *testing.T) {
	currentTools := []openai.Tool{
		makeTool("document_creator"),
	}

	ok := shouldAcceptParsedTextToolCallsInNativeMode(
		currentTools,
		ToolCallParseSourceContentJSON,
		ToolCall{IsTool: true, Action: "docker_exec"},
		nil,
	)

	if ok {
		t.Fatal("did not expect unknown content_json tool call to be accepted in native mode")
	}
}

func TestShouldAcceptParsedTextToolCallsInNativeMode_RejectsReasoningJSON(t *testing.T) {
	currentTools := []openai.Tool{
		makeTool("document_creator"),
	}

	ok := shouldAcceptParsedTextToolCallsInNativeMode(
		currentTools,
		ToolCallParseSourceReasoningCleanJSON,
		ToolCall{IsTool: true, Action: "document_creator"},
		nil,
	)

	if ok {
		t.Fatal("did not expect reasoning_clean_json tool calls to be auto-accepted in native mode")
	}
}
