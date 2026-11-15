package agent

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/gamemaker"
)

var gameMakerAllTools = []string{"game_maker_project", "game_maker_file", "game_maker_asset", "game_maker_preview"}

// gameMakerAliasContext binds a Studio job like the server does; allowed nil
// leaves the run unscoped.
func gameMakerAliasContext(allowed []string) *DispatchContext {
	dc := &DispatchContext{Ctx: gamemaker.WithJobContext(context.Background(), "job-alias")}
	if allowed != nil {
		dc.ToolScopeRestricted, dc.AllowedTools = true, normalizedAllowedToolSet(allowed)
	}
	return dc
}

func TestGameMakerOperationEnumsAreDisjoint(t *testing.T) {
	seen := map[string]string{}
	for tool, operations := range gameMakerToolOperations() {
		if len(operations) == 0 {
			t.Fatalf("%s has no operations", tool)
		}
		for _, operation := range operations {
			if previous, dup := seen[operation]; dup {
				t.Fatalf("operation %q belongs to %s and %s; a bare operation name would be ambiguous", operation, previous, tool)
			}
			seen[operation] = tool
		}
	}
	for operation, tool := range map[string]string{
		"finish": "game_maker_project", "use_template": "game_maker_project", "list_files": "game_maker_project",
		"replace_many": "game_maker_file", "search": "game_maker_file", "search_assets": "game_maker_asset",
		"import_pack": "game_maker_asset", "screenshot": "game_maker_preview",
	} {
		if seen[operation] != tool {
			t.Fatalf("operation %q owned by %q, want %q", operation, seen[operation], tool)
		}
	}
	// A real tool keeps its name: no operation may shadow a builtin.
	for operation := range seen {
		if _, builtin := allBuiltinToolNameSet()[operation]; builtin {
			t.Fatalf("operation %q is also a builtin tool name", operation)
		}
	}
}

func TestGameMakerOperationNamesBecomeTools(t *testing.T) {
	for _, test := range []struct{ text, tool, operation string }{
		{`{"tool":"finish","arguments":{}}`, "game_maker_project", "finish"},
		{`<tool_call>[{"tool":"build","args":{}}]</tool_call>`, "game_maker_project", "build"},
		{`{"tool":"game_maker_file","arguments":{"operation":"list_files"}}`, "game_maker_project", "list_files"},
		{`{"tool":"search_assets","arguments":{"asset_kind":"audio"}}`, "game_maker_asset", "search_assets"},
		{`{"tool":"screenshot","arguments":{}}`, "game_maker_preview", "screenshot"},
	} {
		dc := gameMakerAliasContext(gameMakerAllTools)
		got := prepareToolCall(ParseToolCall(test.text), dc)
		if got.Action != test.tool || got.Operation != test.operation || got.PreparationError != "" {
			t.Fatalf("call: %+v", got)
		}
		if again := prepareToolCall(got, dc); !reflect.DeepEqual(again, got) {
			t.Fatal("normalization is not idempotent")
		}
	}
}

func TestGameMakerOperationAliasKeepsExplicitFields(t *testing.T) {
	dc := gameMakerAliasContext(gameMakerAllTools)
	// An explicit, different operation is the model's own choice.
	got := prepareToolCall(ToolCall{Action: "finish", Params: map[string]interface{}{"operation": "inspect"}}, dc)
	if got.Action != "game_maker_project" || got.Operation != "inspect" || got.Params["operation"] != "inspect" {
		t.Fatalf("explicit operation overwritten: %+v", got)
	}
	// An operation of another tool contradicts the name: left for the ordinary refusal.
	for _, call := range []ToolCall{
		ParseToolCall(`{"tool":"inspect","operation":"write"}`),
		{Action: "finish", Params: map[string]interface{}{"operation": "search_assets"}},
	} {
		if got := prepareToolCall(call, dc); got.Action != call.Action || got.normalizedFrom != "" {
			t.Fatalf("contradicting call %q rewritten to %s", call.Action, got.Action)
		}
	}
	// A different Game Maker operation on a correct tool is left alone.
	for _, call := range []ToolCall{
		{Action: "game_maker_file", Params: map[string]interface{}{"operation": "read", "path": "src/main.ts"}},
		{Action: "game_maker_asset", Params: map[string]interface{}{"job_id": "job-alias"}},
		{Action: "game_maker_preview", Params: map[string]interface{}{"scope": "full"}},
	} {
		if got := prepareToolCall(call, dc); got.Action != call.Action {
			t.Fatalf("%s rewritten to %s", call.Action, got.Action)
		}
	}
}

func TestGameMakerOperationAliasNeedsBoundJobAndScope(t *testing.T) {
	calls := []string{
		`{"tool":"finish","arguments":{"base":"shooter","objective":"x","features":["y"]}}`,
		`{"tool":"inspect","arguments":{}}`,
		`{"tool":"search_assets","arguments":{"asset_kind":"audio"}}`,
		`{"tool":"logs","arguments":{}}`,
		`{"tool":"game_maker_file","arguments":{"operation":"list_files"}}`,
	}
	unbound := map[string]*DispatchContext{
		"no dispatch context": nil,
		"no owner context":    {},
		"context without job": {Ctx: context.Background()},
		"scoped, no job":      {ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet(gameMakerAllTools)},
	}
	for name, dc := range unbound {
		for _, text := range calls {
			call := ParseToolCall(text)
			want := call.Action
			before, _ := json.Marshal(call.Params)
			got := prepareToolCall(call, dc)
			after, _ := json.Marshal(got.Params)
			if got.Action != want || string(before) != string(after) {
				t.Fatalf("%s: %s rewritten to %s %s", name, text, got.Action, after)
			}
		}
	}

	// Scope is never widened: with the owning tool outside the run the call is
	// left as it came, so the ordinary denial still happens.
	for _, test := range []struct {
		text    string
		allowed []string
		tool    string
	}{
		{calls[0], []string{"game_maker_file", "game_maker_asset"}, "finish"},
		{calls[1], []string{"game_maker_file"}, "inspect"},
		{calls[2], []string{"game_maker_project", "game_maker_file"}, "search_assets"},
		{calls[3], []string{"game_maker_project", "game_maker_file", "game_maker_asset"}, "logs"}, // planning scope
		{calls[4], []string{"game_maker_file"}, "game_maker_file"},
	} {
		got := prepareToolCall(ParseToolCall(test.text), gameMakerAliasContext(test.allowed))
		if got.Action != test.tool {
			t.Fatalf("%s: rewritten to %s outside the run's scope", test.text, got.Action)
		}
		if got.Params["operation"] == "finish" || got.Params["design"] != nil {
			t.Fatalf("out-of-scope call was reshaped: %#v", got.Params)
		}
	}
	if got := prepareToolCall(ParseToolCall(calls[0]), gameMakerAliasContext([]string{})); got.Action != "finish" {
		t.Fatalf("empty scope rewritten to %s", got.Action)
	}
}

func TestGameMakerOperationAliasDispatchesWithinScope(t *testing.T) {
	var hooked []ToolCall
	dc := actionSecurityFixture(t, func(_ context.Context, call ToolCall) (string, bool) {
		hooked = append(hooked, call)
		return `{"status":"ok"}`, true
	})
	dc.ToolScopeRestricted, dc.AllowedTools = true, normalizedAllowedToolSet([]string{"game_maker_project"})
	bound := gamemaker.WithJobContext(context.Background(), "job-alias")
	call := ParseToolCall(`{"tool":"inspect","arguments":{}}`)
	result := DispatchToolCallResult(bound, &call, dc, "fixture")
	if result.IsError || len(hooked) != 1 || hooked[0].Action != "game_maker_project" || hooked[0].Operation != "inspect" {
		t.Fatalf("alias did not run as the owning tool: %+v hooked=%+v", result, hooked)
	}

	hooked = nil
	denied := ParseToolCall(`{"tool":"search_assets","arguments":{"asset_kind":"audio"}}`)
	result = DispatchToolCallResult(bound, &denied, dc, "fixture")
	if !result.IsError || len(hooked) != 0 || !strings.Contains(result.Output, "tool_scope_denied") {
		t.Fatalf("out-of-scope alias must keep the scope denial: %+v hooked=%d", result, len(hooked))
	}

	denied = ParseToolCall(`{"tool":"inspect","arguments":{}}`)
	result = DispatchToolCallResult(context.Background(), &denied, dc, "fixture")
	if !result.IsError || len(hooked) != 0 {
		t.Fatalf("an unbound run must not accept operation names: %+v hooked=%d", result, len(hooked))
	}
}

func TestGameMakerNormalizedCallAnnotation(t *testing.T) {
	call := ToolCall{Action: "game_maker_project", Operation: "finish", normalizedFrom: "finish"}
	got := annotateGameMakerNormalizedCall(`Tool Output: {"next_action":"x","status":"ok"}`, call)
	if got != `Tool Output: {"next_action":"x","status":"ok","normalized_call":{"tool":"game_maker_project","operation":"finish"}}` {
		t.Fatalf("annotated output: %s", got)
	}
	if got := annotateGameMakerNormalizedCall(`Tool Output: {}`, call); !strings.HasPrefix(got, `Tool Output: {"normalized_call":`) || !json.Valid([]byte(strings.TrimPrefix(got, "Tool Output: "))) {
		t.Fatalf("empty object: %s", got)
	}
	// Unchanged calls and non-object results pass through byte for byte.
	if got := annotateGameMakerNormalizedCall(`Tool Output: {"status":"ok"}`, ToolCall{Action: "game_maker_project"}); got != `Tool Output: {"status":"ok"}` {
		t.Fatalf("untouched call changed output: %q", got)
	}
	for _, output := range []string{"Tool Output: not json", "Tool Output: [1]", "Tool Output: {broken}", "plain", ""} {
		if got := annotateGameMakerNormalizedCall(output, call); got != output {
			t.Fatalf("non-object result changed: %q", got)
		}
	}
}
