package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestFilterSchemasByAllowedToolsDistinguishesNilAndEmpty(t *testing.T) {
	schemas := []openai.Tool{
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "status"}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "filesystem"}},
	}
	if got := filterSchemasByAllowedTools(schemas, nil); len(got) != 2 {
		t.Fatalf("nil scope returned %d schemas, want 2", len(got))
	}
	if got := filterSchemasByAllowedTools(schemas, []string{}); len(got) != 0 {
		t.Fatalf("empty scope returned %d schemas, want 0", len(got))
	}
	got := filterSchemasByAllowedTools(schemas, []string{" STATUS "})
	if len(got) != 1 || got[0].Function.Name != "status" {
		t.Fatalf("unexpected filtered schemas: %+v", got)
	}
}

func TestDispatchInnerRejectsToolOutsideScope(t *testing.T) {
	dc := &DispatchContext{
		SessionID:           "sip-scope-test",
		ToolScopeRestricted: true,
		AllowedTools:        normalizedAllowedToolSet([]string{"status"}),
	}
	got := dispatchInner(context.Background(), ToolCall{Action: "filesystem"}, dc)
	if !strings.Contains(got, "tool_scope_denied") {
		t.Fatalf("expected scope denial, got %s", got)
	}
}

func TestInvokeToolCannotBypassScope(t *testing.T) {
	sessionID := "sip-invoke-scope-test"
	schemas := []openai.Tool{
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "invoke_tool"}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "filesystem"}},
	}
	SetDiscoverToolsState(sessionID, schemas, schemas, "")
	dc := &DispatchContext{
		SessionID:           sessionID,
		ToolScopeRestricted: true,
		AllowedTools:        normalizedAllowedToolSet([]string{"invoke_tool"}),
	}
	call := ToolCall{Action: "invoke_tool", Params: map[string]interface{}{
		"tool_name": "filesystem",
		"arguments": map[string]interface{}{"operation": "list"},
	}}
	got := dispatchInvokeTool(context.Background(), call, dc)
	if !strings.Contains(got, "tool_scope_denied") {
		t.Fatalf("expected invoke scope denial, got %s", got)
	}
}

func TestAgentSkillScopeIsBinding(t *testing.T) {
	dc := &DispatchContext{
		SkillScopeRestricted: true,
		AllowedAgentSkills: normalizedAllowedToolSet([]string{
			"aurago-game-maker-director",
			"aurago-phaser4-gameplay",
		}),
	}
	if !agentSkillAllowed(dc, " AURAGO-GAME-MAKER-DIRECTOR ") {
		t.Fatal("curated skill was rejected")
	}
	if agentSkillAllowed(dc, "untrusted-game-skill") {
		t.Fatal("skill outside binding scope was allowed")
	}
	got := dispatchActivateAgentSkill(ToolCall{
		Action: "activate_agent_skill",
		Name:   "untrusted-game-skill",
	}, dc)
	if !strings.Contains(got, "outside the active skill scope") {
		t.Fatalf("unexpected activation result: %s", got)
	}
}
