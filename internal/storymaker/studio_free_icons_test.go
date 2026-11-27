package storymaker

import (
	"strings"
	"testing"
)

func TestFreeSearchFindsOnlyIcons(t *testing.T) {
	s := newIconStudio(t, ModeFree, PhaseFill)
	found := iconAnswer(t, s, "search", map[string]any{"kind": "prop", "query": "lock"})
	results, _ := found["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("free search answer = %v", found)
	}
	for _, r := range results {
		if id, _ := r.(map[string]any)["id"].(string); !strings.HasPrefix(id, "icon.") {
			t.Fatalf("free search returned %q, want icons only: %v", id, found)
		}
	}
	first := results[0].(map[string]any)
	example, _ := first["usage_example"].(map[string]any)
	code, _ := example["code"].(string)
	if first["id"] != "icon.lock" || !strings.HasPrefix(code, "api.icon('icon.lock'") || !strings.Contains(code, "label: 'Lock'") {
		t.Fatalf("first free hit = %v", first)
	}
	if _, has := example["do"]; has {
		t.Fatalf("a free usage_example must be code only: %v", example)
	}

	miss := iconAnswer(t, s, "search", map[string]any{"query": "zebra"})
	if empty, _ := miss["results"].([]any); len(empty) != 0 || !strings.Contains(miss["next_action"].(string), "No icon matches.") {
		t.Fatalf("free miss = %v", miss)
	}

	s.SetPhase(PhaseCheck, "")
	refused := iconAnswer(t, s, "search", map[string]any{"query": "lock"})
	if refused["code"] != "operation_not_in_phase" {
		t.Fatalf("free check phase must not search: %v", refused)
	}
}
