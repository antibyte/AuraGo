package agent

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

var gameMakerAllTools = []string{"game_maker_project", "game_maker_file", "game_maker_asset", "game_maker_validate"}

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
		"set_design": "game_maker_project", "scene_patch": "game_maker_project", "list_files": "game_maker_project",
		"replace_many": "game_maker_file", "search": "game_maker_file", "search_assets": "game_maker_asset",
		"import_pack": "game_maker_asset", "validate": "game_maker_validate",
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
	design := map[string]interface{}{"base": "shooter", "objective": "Defend the base", "features": []interface{}{"waves"}}
	for _, test := range []struct {
		name, text    string
		wantTool      string
		wantOperation string
		check         func(*testing.T, ToolCall)
	}{
		{
			name:     "minimax set_design with design wrapper",
			text:     "<tool_call>\n[{\n  \"tool\": \"set_design\",\n  \"args\": {\n    \"design\": { \"base\": \"shooter\", \"objective\": \"Defend the base\", \"features\": [\"waves\"] }\n  }\n}]",
			wantTool: "game_maker_project", wantOperation: "set_design",
			check: func(t *testing.T, tc ToolCall) {
				if !reflect.DeepEqual(tc.Params["design"], design) {
					t.Fatalf("design changed: %#v", tc.Params["design"])
				}
			},
		},
		{
			name:     "glm set_design with flat design fields",
			text:     `{"tool":"set_design","arguments":{"base":"exploration","objective":"Run endlessly","features":["jump"],"stages":["a"],"dry_run":true}}`,
			wantTool: "game_maker_project", wantOperation: "set_design",
			check: func(t *testing.T, tc ToolCall) {
				moved, _ := tc.Params["design"].(map[string]interface{})
				if moved["base"] != "exploration" || moved["objective"] != "Run endlessly" || moved["features"] == nil || moved["stages"] == nil || len(moved) != 4 {
					t.Fatalf("design fields not wrapped: %#v", tc.Params)
				}
				for _, key := range []string{"base", "objective", "features", "stages"} {
					if _, left := tc.Params[key]; left {
						t.Fatalf("%s left at top level: %#v", key, tc.Params)
					}
				}
				if tc.Params["dry_run"] != true || tc.Params["operation"] != "set_design" {
					t.Fatalf("control keys must stay top level: %#v", tc.Params)
				}
			},
		},
		{name: "minimax list_files batch entry", text: `<tool_call>[{"tool":"list_files","args":{"path":"src"}}, {"tool":"get_plan","args":{}}]`, wantTool: "game_maker_project", wantOperation: "list_files"},
		{name: "glm get_plan", text: `{"tool":"get_plan","arguments":{}}`, wantTool: "game_maker_project", wantOperation: "get_plan"},
		{
			name:     "glm search_assets",
			text:     `{"tool":"search_assets","arguments":{"asset_kind":"audio"}}`,
			wantTool: "game_maker_asset", wantOperation: "search_assets",
			check: func(t *testing.T, tc ToolCall) {
				if tc.Params["asset_kind"] != "audio" {
					t.Fatalf("arguments lost: %#v", tc.Params)
				}
			},
		},
		{name: "search is not discover_tools", text: `{"tool":"search","arguments":{"query":"player"}}`, wantTool: "game_maker_file", wantOperation: "search"},
		{name: "replace_many", text: `{"name":"replace_many","arguments":{"edits":[]}}`, wantTool: "game_maker_file", wantOperation: "replace_many"},
		{name: "validate", text: `{"tool":"validate","arguments":{"scope":"full"}}`, wantTool: "game_maker_validate", wantOperation: "validate"},
		{
			name:     "game_maker_file list_files belongs to the project tool",
			text:     `{"tool":"game_maker_file","arguments":{"operation":"list_files"}}`,
			wantTool: "game_maker_project", wantOperation: "list_files",
		},
		{
			name:     "game_maker_project set_design with flat fields",
			text:     `{"tool":"game_maker_project","arguments":{"operation":"set_design","base":"topdown","objective":"Escape"}}`,
			wantTool: "game_maker_project", wantOperation: "set_design",
			check: func(t *testing.T, tc ToolCall) {
				if moved, _ := tc.Params["design"].(map[string]interface{}); moved["base"] != "topdown" || moved["objective"] != "Escape" {
					t.Fatalf("design fields not wrapped: %#v", tc.Params)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dc := gameMakerAliasContext(gameMakerAllTools)
			got := prepareToolCall(ParseToolCall(test.text), dc)
			if got.Action != test.wantTool || got.Operation != test.wantOperation || toolArgString(got.Params, "operation") != test.wantOperation || got.PreparationError != "" {
				t.Fatalf("got action=%q operation=%q params=%#v err=%q", got.Action, got.Operation, got.Params, got.PreparationError)
			}
			if got.TransportAction != "" {
				t.Fatalf("a rewrite must not claim a transport wrapper: %q", got.TransportAction)
			}
			if test.check != nil {
				test.check(t, got)
			}
			if again := prepareToolCall(got, dc); !reflect.DeepEqual(again, got) {
				t.Fatalf("rewrite is not idempotent:\n%#v\n%#v", got, again)
			}
		})
	}
}

func TestGameMakerOperationAliasKeepsExplicitFields(t *testing.T) {
	dc := gameMakerAliasContext(gameMakerAllTools)
	// An explicit, different operation is the model's own choice.
	got := prepareToolCall(ToolCall{Action: "set_design", Params: map[string]interface{}{"operation": "inspect"}}, dc)
	if got.Action != "game_maker_project" || got.Operation != "inspect" || got.Params["operation"] != "inspect" {
		t.Fatalf("explicit operation overwritten: %+v", got)
	}
	// An operation of another tool contradicts the name: left for the ordinary refusal.
	for _, call := range []ToolCall{
		ParseToolCall(`{"tool":"get_plan","operation":"write"}`),
		{Action: "set_design", Params: map[string]interface{}{"operation": "search_assets"}},
	} {
		if got := prepareToolCall(call, dc); got.Action != call.Action || got.normalizedFrom != "" {
			t.Fatalf("contradicting call %q rewritten to %s", call.Action, got.Action)
		}
	}
	// A design that is already wrapped, or has no design-shaped key, is not touched.
	for name, params := range map[string]map[string]interface{}{
		"wrapped":  {"design": map[string]interface{}{"base": "blocks"}, "base": "stray"},
		"no shape": {"description": "a game"},
	} {
		got := prepareToolCall(ToolCall{Action: "set_design", Params: params}, dc)
		if want := len(params); len(got.Params) != want+1 { // + operation
			t.Fatalf("%s: params rewritten: %#v", name, got.Params)
		}
		if _, wrapped := got.Params["design"]; wrapped != (name == "wrapped") {
			t.Fatalf("%s: design wrapper decision wrong: %#v", name, got.Params)
		}
	}
	// A different Game Maker operation on a correct tool is left alone.
	for _, call := range []ToolCall{
		{Action: "game_maker_file", Params: map[string]interface{}{"operation": "read", "path": "src/main.ts"}},
		{Action: "game_maker_asset", Params: map[string]interface{}{"job_id": "job-alias"}},
		{Action: "game_maker_validate", Params: map[string]interface{}{"scope": "full"}},
	} {
		if got := prepareToolCall(call, dc); got.Action != call.Action {
			t.Fatalf("%s rewritten to %s", call.Action, got.Action)
		}
	}
}

func TestGameMakerOperationAliasNeedsBoundJobAndScope(t *testing.T) {
	calls := []string{
		`{"tool":"set_design","arguments":{"base":"shooter","objective":"x","features":["y"]}}`,
		`{"tool":"get_plan","arguments":{}}`,
		`{"tool":"search_assets","arguments":{"asset_kind":"audio"}}`,
		`{"tool":"validate","arguments":{}}`,
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
		{calls[0], []string{"game_maker_file", "game_maker_asset"}, "set_design"},
		{calls[1], []string{"game_maker_file"}, "get_plan"},
		{calls[2], []string{"game_maker_project", "game_maker_file"}, "search_assets"},
		{calls[3], []string{"game_maker_project", "game_maker_file", "game_maker_asset"}, "validate"}, // planning scope
		{calls[4], []string{"game_maker_file"}, "game_maker_file"},
	} {
		got := prepareToolCall(ParseToolCall(test.text), gameMakerAliasContext(test.allowed))
		if got.Action != test.tool {
			t.Fatalf("%s: rewritten to %s outside the run's scope", test.text, got.Action)
		}
		if got.Params["operation"] == "set_design" || got.Params["design"] != nil {
			t.Fatalf("out-of-scope call was reshaped: %#v", got.Params)
		}
	}
	if got := prepareToolCall(ParseToolCall(calls[0]), gameMakerAliasContext([]string{})); got.Action != "set_design" {
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
	call := ParseToolCall(`{"tool":"get_plan","arguments":{}}`)
	result := DispatchToolCallResult(bound, &call, dc, "fixture")
	if result.IsError || len(hooked) != 1 || hooked[0].Action != "game_maker_project" || hooked[0].Operation != "get_plan" {
		t.Fatalf("alias did not run as the owning tool: %+v hooked=%+v", result, hooked)
	}

	hooked = nil
	denied := ParseToolCall(`{"tool":"search_assets","arguments":{"asset_kind":"audio"}}`)
	result = DispatchToolCallResult(bound, &denied, dc, "fixture")
	if !result.IsError || len(hooked) != 0 || !strings.Contains(result.Output, "tool_scope_denied") {
		t.Fatalf("out-of-scope alias must keep the scope denial: %+v hooked=%d", result, len(hooked))
	}

	denied = ParseToolCall(`{"tool":"get_plan","arguments":{}}`)
	result = DispatchToolCallResult(context.Background(), &denied, dc, "fixture")
	if !result.IsError || len(hooked) != 0 {
		t.Fatalf("an unbound run must not accept operation names: %+v hooked=%d", result, len(hooked))
	}
}

func TestGameMakerNormalizedCallAnnotation(t *testing.T) {
	call := ToolCall{Action: "game_maker_project", Operation: "set_design", normalizedFrom: "set_design"}
	got := annotateGameMakerNormalizedCall(`Tool Output: {"next_action":"x","status":"ok"}`, call)
	if got != `Tool Output: {"next_action":"x","status":"ok","normalized_call":{"tool":"game_maker_project","operation":"set_design"}}` {
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

// The rewritten calls must be accepted by the real tools, flat design included.
func TestGameMakerOperationAliasRunsAgainstService(t *testing.T) {
	root := t.TempDir()
	s, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(s)
	defer gamemaker.SetDefaultService(previous)
	s.SetSkillStatus(nil, true)
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Grove", Dimension: "2d", Description: "Explore a grove"})
	if err != nil {
		t.Fatal(err)
	}
	flat, _ := json.Marshal(gamemaker.ExampleGameDesign(project))
	var arguments map[string]any
	_ = json.Unmarshal(flat, &arguments)
	arguments["dry_run"] = true
	flat, _ = json.Marshal(map[string]any{"tool": "set_design", "arguments": arguments})

	outputs := make(chan []string, 1)
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		bound := gamemaker.WithJobContext(ctx, run.Job.ID)
		dc := gameMakerAliasContext([]string{"game_maker_project", "game_maker_file"})
		dc.Ctx = bound
		var got []string
		for _, text := range []string{string(flat), `{"tool":"list_files","arguments":{}}`, `{"tool":"game_maker_file","arguments":{"operation":"list_files"}}`, `{"tool":"search_assets","arguments":{"query":"tree"}}`} {
			out, _ := dispatchGameMaker(bound, prepareToolCall(ParseToolCall(text), dc), dc)
			got = append(got, out)
		}
		outputs <- got
		return errors.New("fixture complete")
	}))
	if _, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-outputs:
		for i, want := range []string{`"dry_run":true`, `"files"`, `"files"`} {
			if !strings.Contains(got[i], `"status":"ok"`) || !strings.Contains(got[i], want) {
				t.Fatalf("call %d not accepted: %s", i, got[i])
			}
		}
		if !strings.Contains(got[0], `"normalized_call":{"tool":"game_maker_project","operation":"set_design"}`) ||
			!strings.Contains(got[2], `"normalized_call":{"tool":"game_maker_project","operation":"list_files"}`) {
			t.Fatalf("rewritten calls lack the normalized_call note: %s / %s", got[0], got[2])
		}
		// The asset tool is outside this run's scope: its operation name stays an unknown action.
		if strings.Contains(got[3], "normalized_call") {
			t.Fatalf("out-of-scope alias ran: %s", got[3])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("fixture did not run")
	}
}
