package server

import "testing"

func TestN8nEffectiveAllowedToolsRequiresGlobalAllowlist(t *testing.T) {
	got := n8nEffectiveAllowedTools(nil, []string{"execute_shell"})
	if len(got) != 0 {
		t.Fatalf("n8nEffectiveAllowedTools with empty global allowlist = %#v, want no tools", got)
	}
}

func TestN8nEffectiveAllowedToolsIntersectsRequestWithGlobalAllowlist(t *testing.T) {
	got := n8nEffectiveAllowedTools([]string{"query_memory", "manage_missions"}, []string{"execute_shell", "query_memory"})
	if len(got) != 1 || got[0] != "query_memory" {
		t.Fatalf("n8nEffectiveAllowedTools intersection = %#v, want query_memory only", got)
	}
}
