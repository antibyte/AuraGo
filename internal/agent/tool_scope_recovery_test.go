package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aurago/internal/gamemaker"
)

type scopeRecoveryBody struct {
	Status       string   `json:"status"`
	Code         string   `json:"code"`
	Message      string   `json:"message"`
	AllowedTools []string `json:"allowed_tools"`
	Hint         string   `json:"hint"`
}

func decodeScopeRecovery(t *testing.T, output string) scopeRecoveryBody {
	t.Helper()
	var body scopeRecoveryBody
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &body); err != nil {
		t.Fatalf("refusal is not JSON: %v: %s", err, output)
	}
	return body
}

func gameMakerPhaseScope(t *testing.T, stage string) (*DispatchContext, []string) {
	t.Helper()
	var names []string
	for _, definition := range GameMakerPhaseToolSchemas(stage, "2d") {
		names = append(names, definition.Function.Name)
	}
	slices.Sort(names)
	return &DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet(names)}, names
}

func TestScopeRefusalsListWhatTheRunMayCall(t *testing.T) {
	dc, phase := gameMakerPhaseScope(t, "planning")
	if slices.Contains(phase, "game_maker_validate") || len(phase) != 3 {
		t.Fatalf("fixture: planning must offer project, file and asset only: %v", phase)
	}
	const generic = "Call one of allowed_tools by its exact name; operations such as set_design go in the operation argument."

	// Both refusal paths answer from the same allow-list with the same fields.
	policy := checkDispatchActionPolicy(context.Background(), ToolCall{Action: "filesystem"}, dc)
	if policy == nil || policy.Status != ToolResultDenied || !policy.IsError {
		t.Fatalf("policy path: %+v", policy)
	}
	fromPolicy := decodeScopeRecovery(t, policy.Output)
	fromScope := decodeScopeRecovery(t, toolScopeDeniedFor(dc, "filesystem"))
	if fromPolicy.Code != "tool_scope_denied" || fromPolicy.Status != "denied" || fromPolicy.Message != "The tool is outside this run's scope." ||
		fromScope.Code != "tool_scope_denied" || fromScope.Status != "error" || fromScope.Message != `tool "filesystem" is not allowed in this agent run` {
		t.Fatalf("existing fields changed: %+v / %+v", fromPolicy, fromScope)
	}
	for name, body := range map[string]scopeRecoveryBody{"policy": fromPolicy, "scope": fromScope} {
		if !reflect.DeepEqual(body.AllowedTools, phase) || body.Hint != generic {
			t.Fatalf("%s refusal: allowed_tools=%v hint=%q, want %v", name, body.AllowedTools, body.Hint, phase)
		}
	}

	// The end-to-end dispatch carries the same recovery fields.
	fixture := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { return `{"status":"ok"}`, true })
	fixture.ToolScopeRestricted, fixture.AllowedTools = dc.ToolScopeRestricted, dc.AllowedTools
	call := ToolCall{Action: "filesystem"}
	if result := DispatchToolCallResult(context.Background(), &call, fixture, "fixture"); !result.IsError || !reflect.DeepEqual(decodeScopeRecovery(t, result.Output).AllowedTools, phase) {
		t.Fatalf("dispatch refusal: %+v", result)
	}
}

func TestScopeRefusalHintNamesTheOwnerOfAnOutOfPhaseOperation(t *testing.T) {
	planning, _ := gameMakerPhaseScope(t, "planning")
	building, _ := gameMakerPhaseScope(t, "building")
	for _, test := range []struct {
		name     string
		dc       *DispatchContext
		denied   string
		wantHint string
	}{
		{"validate during planning", planning, "validate", "validate is an operation of game_maker_validate, which is not available in this phase. Call one of allowed_tools by its exact name."},
		{"owner is in scope", planning, "write", "Call one of allowed_tools by its exact name; operations such as set_design go in the operation argument."},
		{"owner is in scope when building", building, "validate", "Call one of allowed_tools by its exact name; operations such as set_design go in the operation argument."},
		{"not a Game Maker run", &DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{"status"})}, "validate", "Call one of allowed_tools by its exact name; operations go in the operation argument."},
		{"no tools at all", &DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{})}, "validate", "No tool is available in this run; answer without a tool call."},
	} {
		t.Run(test.name, func(t *testing.T) {
			for path, output := range map[string]string{
				"scope":  toolScopeDeniedFor(test.dc, test.denied),
				"policy": checkDispatchActionPolicy(context.Background(), ToolCall{Action: test.denied}, test.dc).Output,
			} {
				if hint := decodeScopeRecovery(t, output).Hint; hint != test.wantHint {
					t.Fatalf("%s: hint = %q, want %q", path, hint, test.wantHint)
				}
			}
		})
	}
	// A bound planning run: P1 leaves the unreachable operation alone, so the
	// model learns the owner instead of "all tools are out of scope".
	dispatchDC := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { return `{"status":"ok"}`, true })
	dispatchDC.ToolScopeRestricted, dispatchDC.AllowedTools = planning.ToolScopeRestricted, planning.AllowedTools
	call := ParseToolCall(`{"tool":"validate","arguments":{}}`)
	result := DispatchToolCallResult(gamemaker.WithJobContext(context.Background(), "job-recovery"), &call, dispatchDC, "fixture")
	if body := decodeScopeRecovery(t, result.Output); body.Code != "tool_scope_denied" || !strings.Contains(body.Hint, "game_maker_validate") {
		t.Fatalf("planning validate refusal: %s", result.Output)
	}
}

func TestScopeRefusalAllowListIsBoundedAndSorted(t *testing.T) {
	var names []string
	for i := 20; i > 0; i-- {
		names = append(names, fmt.Sprintf("tool_%02d", i))
	}
	dc := &DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet(names)}
	got := decodeScopeRecovery(t, toolScopeDeniedFor(dc, "other")).AllowedTools
	if len(got) != 13 || got[12] != "…" || !slices.IsSorted(got[:12]) || got[0] != "tool_01" || got[11] != "tool_12" {
		t.Fatalf("allowed_tools = %v", got)
	}
	if empty := decodeScopeRecovery(t, toolScopeDeniedFor(&DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{})}, "x")); empty.AllowedTools == nil || len(empty.AllowedTools) != 0 {
		t.Fatalf("an empty scope must list an empty set, got %#v", empty.AllowedTools)
	}
}

// Without a restricted scope the refusals are exactly what they were.
func TestScopeRefusalsAreUnchangedWithoutRestrictedScope(t *testing.T) {
	const (
		scopeOutput  = `Tool Output: {"status":"error","message":"tool \"filesystem\" is not allowed in this agent run","code":"tool_scope_denied"}`
		policyOutput = `Tool Output: {"code":"tool_scope_denied","message":"The tool is outside this run's scope.","status":"denied"}`
	)
	// AllowedTools without ToolScopeRestricted is not a scope.
	for name, dc := range map[string]*DispatchContext{
		"nil":                  nil,
		"unrestricted":         {},
		"allow-list not bound": {AllowedTools: normalizedAllowedToolSet([]string{"status"})},
	} {
		if got := toolScopeDeniedFor(dc, "filesystem"); got != scopeOutput || got != toolScopeDeniedOutput("filesystem") {
			t.Fatalf("%s: scope refusal changed: %s", name, got)
		}
		if got := scopeDeniedRefusal(dc, "filesystem"); got.Output != policyOutput || got.Status != ToolResultDenied || !got.IsError {
			t.Fatalf("%s: policy refusal changed: %+v", name, got)
		}
	}
}
