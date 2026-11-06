package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
)

func storymakerJobDispatch(t *testing.T, policy bool, hooks *ExecutionHooks) *DispatchContext {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.ActionPolicy.Enabled = policy
	dc := &DispatchContext{
		Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{"storymaker_catalog", "storymaker_story"}),
		ExecutionHooks: hooks,
	}
	t.Cleanup(func() { clearIsolatedModelBodies(dc.SessionID) })
	return dc
}

func TestStorymakerGeneralToolRefusedInJob(t *testing.T) {
	calls := 0
	dc := storymakerJobDispatch(t, true, &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
		calls++
		return `{"status":"ok"}`, true
	}})

	// remove_scene matches the remove_ keyword, but the server binds this tool to the job's project.
	removal := ToolCall{Action: "storymaker_story", Operation: "remove_scene", Params: map[string]interface{}{"operation": "remove_scene", "scene_id": "s1"}}
	if DescribeAction(removal, dc).HighRisk {
		t.Fatal("remove_scene is high-risk for a storymaker_story job call")
	}
	// The private route passes an enabled action policy without a catalog entry.
	own := ToolCall{Action: "storymaker_story", Operation: "put_scene", Params: map[string]interface{}{"operation": "put_scene", "scene_id": "s1"}}
	if result := DispatchToolCallResult(context.Background(), &own, dc, "fill the scene"); calls != 1 || result.Status == ToolResultDenied {
		t.Fatalf("job tool did not reach its hook: calls=%d result=%+v", calls, result)
	}

	for _, test := range []struct {
		action, hint string
	}{
		{"execute_shell", "operations such as put_scene go in the operation argument"},
		{"validate", "validate is an operation of storymaker_check, which is not available in this phase"},
	} {
		call := ToolCall{Action: test.action, Params: map[string]interface{}{"command": "id"}}
		result := DispatchToolCallResult(context.Background(), &call, dc, "fill the scene")
		var body struct {
			Code         string   `json:"code"`
			AllowedTools []string `json:"allowed_tools"`
			Hint         string   `json:"hint"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(result.Output, "Tool Output: ")), &body); err != nil {
			t.Fatalf("%s: not a refusal envelope: %v: %s", test.action, err, result.Output)
		}
		if result.Status != ToolResultDenied || body.Code != "tool_scope_denied" {
			t.Fatalf("%s: refusal = %+v", test.action, result)
		}
		if !slices.Equal(body.AllowedTools, []string{"storymaker_catalog", "storymaker_story"}) || !strings.Contains(body.Hint, test.hint) {
			t.Fatalf("%s: allowed=%v hint=%q", test.action, body.AllowedTools, body.Hint)
		}
	}
	if calls != 1 {
		t.Fatalf("a refused call reached the hook: calls=%d", calls)
	}
}

func TestStorymakerToolRefusedWithoutHooks(t *testing.T) {
	for _, test := range []struct {
		name   string
		policy bool
		want   string
	}{
		{"policy on", true, `"code":"unknown_tool"`},
		{"policy off", false, "ERROR unknown action"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dc := storymakerJobDispatch(t, test.policy, nil)
			call := ToolCall{Action: "storymaker_story", Operation: "get", Params: map[string]interface{}{"operation": "get"}}
			result := DispatchToolCallResult(context.Background(), &call, dc, "read the story")
			if !strings.Contains(result.Output, test.want) {
				t.Fatalf("a job tool without hooks was not refused: %+v", result)
			}
		})
	}
}
