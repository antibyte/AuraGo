package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"

	openai "github.com/sashabaranov/go-openai"
)

func TestGameMakerBoundWritesPreserveScopeAcrossTransports(t *testing.T) {
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
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Breakout", Dimension: "2d", Description: "Breakout with power-ups"})
	if err != nil {
		t.Fatal(err)
	}
	native := func(args map[string]any) ToolCall {
		data, _ := json.Marshal(args)
		return NativeToolCallToToolCall(openai.ToolCall{Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "game_maker_file", Arguments: string(data)}}, nil)
	}
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		bound := gamemaker.WithJobContext(ctx, run.Job.ID)
		write := native(map[string]any{"path": "src/probe.ts", "content": "export const probe = 7;"})
		if run.Stage == "planning" {
			out, _ := dispatchGameMaker(bound, write, nil)
			if !strings.Contains(out, "planning_required") {
				return fmt.Errorf("implicit write bypassed planning: %s", out)
			}
			return s.SetPlan(ctx, run.Job.ID, gamemaker.ExampleGamePlan(project))
		}
		for _, tc := range []ToolCall{
			write,
			ParseToolCall(`<tool_call><function=game_maker_file><parameter=path>src/probe.ts</parameter><parameter=content>export const probe = 7;</parameter></function></tool_call>`),
			ParseToolCall(`<tool_call><function=game_maker_file><parameter=operation>write</parameter><parameter=path>src/probe.ts</parameter><parameter=content>export const probe = 7;</parameter></function></tool_call>`),
		} {
			out, handled := dispatchGameMaker(bound, tc, nil)
			got, err := s.ReadJobFile(ctx, run.Job.ID, "src/probe.ts")
			if !handled || !strings.Contains(out, `"operation":"write"`) || err != nil || got != "export const probe = 7;" {
				return fmt.Errorf("bound native/XML write failed: %s, read=%q, err=%v", out, got, err)
			}
		}
		for _, action := range []string{"game_maker_file", "game_maker_project", "game_maker_asset", "game_maker_validate"} {
			out, _ := dispatchGameMaker(bound, ToolCall{Action: action, Params: map[string]any{"job_id": "foreign-job"}}, nil)
			if !strings.Contains(out, "does not match") {
				return fmt.Errorf("%s admitted a foreign job: %s", action, out)
			}
		}
		// Native JSON numbers and XML text must address the same bounded range.
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/lines.ts", strings.Repeat("// line\n", 350), ""); err != nil {
			return err
		}
		var previousRead string
		for _, tc := range []ToolCall{
			native(map[string]any{"operation": "read", "path": "src/lines.ts", "start_line": 121, "end_line": 160}),
			ParseToolCall(`<tool_call><function=game_maker_file><parameter=operation>read</parameter><parameter=path>src/lines.ts</parameter><parameter=start_line>121</parameter><parameter=end_line>160</parameter></function></tool_call>`),
			native(map[string]any{"operation": "read", "path": "src/lines.ts", "start_line": "121", "end_line": "160"}),
		} {
			out, _ := dispatchGameMaker(bound, tc, nil)
			if !strings.Contains(out, `"start_line":121`) || !strings.Contains(out, `"end_line":160`) || previousRead != "" && previousRead != out {
				return fmt.Errorf("native/XML line range mismatch: %s", out)
			}
			previousRead = out
		}
		for _, field := range []string{"start_line", "end_line"} {
			for _, bad := range []any{"1.5", "-1", "NaN", "Inf", "10000001", "1e500", "", "true", "121x", true} {
				out, _ := dispatchGameMaker(bound, native(map[string]any{"operation": "read", "path": "src/lines.ts", field: bad}), nil)
				if !strings.Contains(out, field+" must be a nonnegative integer") {
					return fmt.Errorf("invalid %s=%v accepted: %s", field, bad, out)
				}
			}
		}
		for _, check := range []struct {
			ctx  context.Context
			tc   ToolCall
			want string
		}{
			{ctx, write, "job_id is required"},
			{ctx, native(map[string]any{"job_id": run.Job.ID, "path": "src/probe.ts", "content": "changed"}), "operation must be read, write or replace"},
			{bound, native(map[string]any{"path": "src/probe.ts"}), "operation must be read, write or replace"},
			{bound, native(map[string]any{"path": "src/probe.ts", "content": 42}), "operation must be read, write or replace"},
			{bound, native(map[string]any{"operation": "write", "path": "src/probe.ts"}), "explicit string content"},
			{bound, native(map[string]any{"operation": "replace", "path": "src/probe.ts", "old_text": "probe"}), "explicit new_text"},
			{bound, native(map[string]any{"operation": "read", "path": "src/probe.ts", "start_line": 1.5}), "nonnegative integer"},
			{bound, native(map[string]any{"path": "../escape.ts", "content": "changed"}), `"status":"error"`},
			{bound, native(map[string]any{"path": "vendor/aurago-game-1.js", "content": "changed"}), `"status":"error"`},
		} {
			out, _ := dispatchGameMaker(check.ctx, check.tc, nil)
			if !strings.Contains(out, check.want) {
				return fmt.Errorf("write gate %q: %s", check.want, out)
			}
		}
		out, _ := dispatchGameMaker(bound, native(map[string]any{"path": "src/probe.ts", "content": ""}), nil)
		got, err := s.ReadJobFile(ctx, run.Job.ID, "src/probe.ts")
		if !strings.Contains(out, `"status":"ok"`) || err != nil || got != "" {
			return fmt.Errorf("explicit empty content was not written: %s", out)
		}
		s.UpdatePolicy(gamemaker.Policy{Enabled: true, ReadOnly: true, AllowEdit: true})
		out, _ = dispatchGameMaker(bound, write, nil)
		if !strings.Contains(out, "read-only") {
			return fmt.Errorf("bound write bypassed read-only: %s", out)
		}
		return fmt.Errorf("verified bound writes")
	}))
	job, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); ; {
		finished, err := s.GetJob(context.Background(), job.ID)
		if err == nil && (finished.Status == "failed" || finished.Status == "cancelled") {
			if finished.Error != "verified bound writes" {
				t.Fatalf("unexpected result: %+v", finished)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("bound dispatch test did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
