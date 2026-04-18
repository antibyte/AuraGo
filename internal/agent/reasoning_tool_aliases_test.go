package agent

import (
	"testing"

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

func TestKnownReasoningExtractedActionSet_IncludesDirectBuiltinSkillActions(t *testing.T) {
	known := knownReasoningExtractedActionSet(nil, nil)

	for _, action := range []string{"brave_search", "web_scraper", "wikipedia_search", "ddg_search"} {
		if _, ok := known[action]; !ok {
			t.Fatalf("expected %q in known reasoning action set", action)
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

