package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestInferRuleFromResolution_DockerPortConflict(t *testing.T) {
	rule := inferRuleFromResolution("docker_run", "port already in use: 8080", "")
	if rule == "" {
		t.Error("expected a rule for docker port conflict")
	}
	if !containsIgnoreCase(rule, "docker ps") {
		t.Errorf("expected rule to mention 'docker ps', got: %s", rule)
	}
}

func TestInferRuleFromResolution_ShellPermissionDenied(t *testing.T) {
	rule := inferRuleFromResolution("execute_shell", "permission denied: /etc/config", "")
	if rule == "" {
		t.Error("expected a rule for shell permission denied")
	}
	if !containsIgnoreCase(rule, "ls -l") {
		t.Errorf("expected rule to mention 'ls -l', got: %s", rule)
	}
}

func TestInferRuleFromResolution_NoMatch(t *testing.T) {
	rule := inferRuleFromResolution("unknown_tool", "some random error", "")
	if rule != "" {
		t.Errorf("expected empty rule for unknown pattern, got: %s", rule)
	}
}

func TestBuildLearnedRulesContext(t *testing.T) {
	rules := []memory.LearnedRule{
		{ToolName: "docker_run", Rule: "check ports first", Confidence: 0.8},
		{ToolName: "execute_shell", Rule: "use sudo", Confidence: 0.6},
	}
	ctx := buildLearnedRulesContext(rules, 100)
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if !containsIgnoreCase(ctx, "docker_run") {
		t.Error("expected context to contain docker_run")
	}
	if !containsIgnoreCase(ctx, "execute_shell") {
		t.Error("expected context to contain execute_shell")
	}
}

func TestBuildLearnedRulesContext_Empty(t *testing.T) {
	ctx := buildLearnedRulesContext(nil, 100)
	if ctx != "" {
		t.Errorf("expected empty context, got: %s", ctx)
	}
}

func TestBuildLearnedRulesContext_TokenBudget(t *testing.T) {
	rules := []memory.LearnedRule{
		{ToolName: "docker_run", Rule: "check ports first", Confidence: 0.8},
		{ToolName: "execute_shell", Rule: "use sudo", Confidence: 0.6},
	}
	// Tight budget of 20 tokens should return only the first rule (~12 tokens)
	ctx := buildLearnedRulesContext(rules, 20)
	if ctx == "" {
		t.Fatal("expected non-empty context with 20-token budget")
	}
	// With only 20 tokens we should not fit both rules
	if containsIgnoreCase(ctx, "execute_shell") {
		t.Error("expected second rule to be dropped with tight budget")
	}
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
