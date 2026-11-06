package agent

import (
	"testing"

	"aurago/internal/storymaker"
)

func TestStorymakerOperationAsToolIsRerouted(t *testing.T) {
	scoped := func(allowed ...string) *DispatchContext {
		return &DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet(allowed)}
	}
	fill := scoped(storymaker.ToolCatalog, storymaker.ToolStory)
	data := map[string]interface{}{"environment": "env.office"}

	for _, test := range []struct {
		name          string
		call          ToolCall
		wantTool      string
		wantOperation string
	}{
		{"operation as tool", ToolCall{Action: "put_scene", Params: map[string]interface{}{"scene_id": "s1", "data": data}}, storymaker.ToolStory, "put_scene"},
		{"search is not discover_tools", ToolCall{Action: "search", Params: map[string]interface{}{"query": "lock"}}, storymaker.ToolCatalog, "search"},
		{"catalog operation on the story tool", ToolCall{Action: storymaker.ToolStory, Params: map[string]interface{}{"operation": "search", "query": "lock"}}, storymaker.ToolCatalog, "search"},
		{"story operation on the catalog tool", ToolCall{Action: storymaker.ToolCatalog, Params: map[string]interface{}{"operation": "put_scene", "scene_id": "s1"}}, storymaker.ToolStory, "put_scene"},
		{"dry_run as tool keeps its target", ToolCall{Action: "dry_run", Params: map[string]interface{}{"op": "put_scene", "scene_id": "s1"}}, storymaker.ToolStory, "dry_run"},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := test.call.Action
			_, hadOperation := test.call.Params["operation"]
			got := prepareToolCall(test.call, fill)
			if got.Action != test.wantTool || got.Operation != test.wantOperation || got.Params["operation"] != test.wantOperation || got.normalizedFrom != before {
				t.Fatalf("rewritten to %s %s from %q: %#v", got.Action, got.Operation, got.normalizedFrom, got.Params)
			}
			if _, has := test.call.Params["operation"]; has != hadOperation {
				t.Fatal("the caller's params were mutated")
			}
		})
	}
	if got := prepareToolCall(ToolCall{Action: "put_scene", Params: map[string]interface{}{"scene_id": "s1", "data": data}}, fill); got.Params["scene_id"] != "s1" || got.Params["data"] == nil {
		t.Fatalf("arguments lost: %#v", got.Params)
	}
	if got := prepareToolCall(ToolCall{Action: "dry_run", Params: map[string]interface{}{"op": "put_scene"}}, fill); got.Params["op"] != "put_scene" {
		t.Fatalf("dry_run target lost: %#v", got.Params)
	}
	if got := prepareToolCall(ToolCall{Action: storymaker.ToolStory, Operation: "put_scene", Params: map[string]interface{}{"op": "put_scene", "scene_id": "s1"}}, fill); toolArgString(got.Params, "operation") != "" {
		t.Fatalf("an op argument became the operation: %+v", got)
	}

	for _, test := range []struct {
		name string
		call ToolCall
		dc   *DispatchContext
	}{
		{"owner outside the scope", ToolCall{Action: "validate"}, fill},
		{"operation owner outside the scope", ToolCall{Action: storymaker.ToolStory, Params: map[string]interface{}{"operation": "search"}}, scoped(storymaker.ToolStory)},
		{"contradictory operation", ToolCall{Action: "put_scene", Params: map[string]interface{}{"operation": "search"}}, fill},
		{"mixed scope", ToolCall{Action: "put_scene"}, scoped(storymaker.ToolStory, "execute_shell")},
		{"no scope", ToolCall{Action: "put_scene"}, &DispatchContext{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := prepareToolCall(test.call, test.dc); got.Action != test.call.Action || got.normalizedFrom != "" {
				t.Fatalf("rewritten to %s from %q", got.Action, got.normalizedFrom)
			}
		})
	}

	// A real tool keeps its name: no operation may shadow a builtin.
	for operation := range storymaker.OperationOwners() {
		if _, builtin := allBuiltinToolNameSet()[operation]; builtin {
			t.Fatalf("operation %q is also a builtin tool name", operation)
		}
	}
}
