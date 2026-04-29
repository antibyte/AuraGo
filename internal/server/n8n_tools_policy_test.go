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

func TestN8nScopeAllowedRequiresExplicitScope(t *testing.T) {
	if n8nScopeAllowed(nil, N8nScopeTools) {
		t.Fatal("empty n8n scopes must not allow tool execution")
	}
	if !n8nScopeAllowed([]string{N8nScopeRead, N8nScopeTools}, N8nScopeTools) {
		t.Fatal("configured n8n tool scope should be allowed")
	}
	if !n8nScopeAllowed([]string{N8nScopeAdmin}, N8nScopeTools) {
		t.Fatal("n8n admin scope should allow subordinate scopes")
	}
}
