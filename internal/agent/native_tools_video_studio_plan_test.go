package agent

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestVideoStudioPlanToolSchemasFollowThePhaseTable(t *testing.T) {
	table := map[string][]string{
		VideoStudioPlanPhaseOutline: {"vs_plan_read", "vs_plan_write"},
		VideoStudioPlanPhaseDetails: {"vs_plan_read", "vs_plan_write", "vs_plan_patch_scene"},
		VideoStudioPlanPhaseCheck:   {"vs_plan_read", "vs_plan_check", "vs_plan_patch_scene", "vs_plan_write", "vs_plan_finish"},
		VideoStudioPlanPhaseRevise:  {"vs_plan_read", "vs_plan_patch_scene", "vs_plan_check", "vs_plan_finish"},
	}
	for phase, want := range table {
		for _, compact := range []bool{false, true} {
			schemas := VideoStudioPlanToolSchemas(phase, compact)
			var got []string
			for _, definition := range schemas {
				got = append(got, definition.Function.Name)
				storymakerRequireClosed(t, phase+"/"+definition.Function.Name, definition.Function.Parameters.(map[string]interface{}))
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s compact=%v tools = %v, want %v", phase, compact, got, want)
			}
			write := storymakerToolParams(schemas, VideoStudioPlanToolWrite)
			if write == nil {
				continue
			}
			outline := phase == VideoStudioPlanPhaseOutline
			scene := storymakerSchemaAt(write, "plan", "scenes", "[]")
			if scene == nil {
				t.Fatalf("%s compact=%v: vs_plan_write has no scene items", phase, compact)
			}
			if (storymakerSchemaAt(scene, "video_prompt") != nil) == outline {
				t.Errorf("%s compact=%v: video_prompt has the wrong presence", phase, compact)
			}
			if (storymakerSchemaAt(write, "plan", "style") != nil) == outline {
				t.Errorf("%s compact=%v: style has the wrong presence", phase, compact)
			}
			if (storymakerSchemaAt(scene, "negative_prompt") != nil) != (!compact && !outline) {
				t.Errorf("%s compact=%v: negative_prompt has the wrong presence", phase, compact)
			}
			if _, described := storymakerSchemaAt(scene, "title")["description"]; described == compact {
				t.Errorf("%s compact=%v: the title description has the wrong presence", phase, compact)
			}
			for _, owned := range []string{"state", "clip_asset_id", "still_asset_id", "timeline_clip_id", "cost", "error"} {
				if storymakerSchemaAt(scene, owned) != nil {
					t.Errorf("%s compact=%v offers the server field %s", phase, compact, owned)
				}
			}
			if storymakerSchemaAt(scene, "keyframe", "asset_id") != nil {
				t.Errorf("%s compact=%v offers keyframe.asset_id", phase, compact)
			}
		}
	}
	patch := storymakerToolParams(VideoStudioPlanToolSchemas(VideoStudioPlanPhaseRevise, false), VideoStudioPlanToolPatchScene)
	if storymakerSchemaAt(patch, "scene", "transition_in", "kind") == nil || storymakerSchemaAt(patch, "scene", "id") != nil {
		t.Errorf("patch scene schema = %v", storymakerSchemaAt(patch, "scene"))
	}
	full := estimateSingleToolSchemaTokens(VideoStudioPlanToolSchemas(VideoStudioPlanPhaseDetails, false)[1])
	small := estimateSingleToolSchemaTokens(VideoStudioPlanToolSchemas(VideoStudioPlanPhaseDetails, true)[1])
	if small >= full {
		t.Errorf("the compact write schema has %d tokens, the full one %d", small, full)
	}
	if len(VideoStudioPlanToolSchemas("unknown", false)) != 0 {
		t.Error("an unknown phase offered tools")
	}
}

func TestVideoStudioPlanToolsStayOutOfTheCatalogAndStrictProjection(t *testing.T) {
	for _, name := range builtinToolNames(allBuiltinToolFeatureFlags()) {
		if strings.HasPrefix(name, VideoStudioPlanToolPrefix) {
			t.Fatalf("%s is in the normal tool catalog", name)
		}
	}
	schemas := append(VideoStudioPlanToolSchemas(VideoStudioPlanPhaseCheck, false), tool("ordinary_probe", "probe", schema(map[string]interface{}{"q": prop("string", "query")})))
	for _, definition := range strictSchemasForActiveTools(newNativeToolSchemaSnapshot(schemas), schemas) {
		if want := definition.Function.Name == "ordinary_probe"; definition.Function.Strict != want {
			t.Errorf("%s strict = %v, want %v", definition.Function.Name, definition.Function.Strict, want)
		}
	}
}

func TestVideoStudioPlanToolsAreJobPrivate(t *testing.T) {
	dispatch := func(t *testing.T, policy bool, hooks *ExecutionHooks) *DispatchContext {
		t.Helper()
		cfg := &config.Config{}
		cfg.Agent.ActionPolicy.Enabled = policy
		dc := &DispatchContext{
			Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet(VideoStudioPlanPhaseTools(VideoStudioPlanPhaseCheck)),
			ExecutionHooks: hooks,
		}
		t.Cleanup(func() { clearIsolatedModelBodies(dc.SessionID) })
		return dc
	}
	calls := 0
	dc := dispatch(t, true, &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
		calls++
		return `Tool Output: {"status":"ok"}`, true
	}})
	write := ToolCall{Action: VideoStudioPlanToolWrite, Params: map[string]interface{}{"plan": map[string]interface{}{"style": "35 mm film look"}}}
	if result := DispatchToolCallResult(context.Background(), &write, dc, "plan the video"); calls != 1 || result.Status == ToolResultDenied {
		t.Fatalf("the plan tool did not reach its hook under the action policy: calls=%d result=%+v", calls, result)
	}
	for _, test := range []struct {
		name   string
		policy bool
		want   string
	}{
		{"policy on", true, `"code":"unknown_tool"`},
		{"policy off", false, "ERROR unknown action"},
	} {
		t.Run(test.name, func(t *testing.T) {
			call := ToolCall{Action: VideoStudioPlanToolRead}
			result := DispatchToolCallResult(context.Background(), &call, dispatch(t, test.policy, nil), "read the plan")
			if !strings.Contains(result.Output, test.want) {
				t.Fatalf("a plan tool without the job hooks was not refused: %+v", result)
			}
		})
	}
}

func TestVideoStudioPlanToolOutputIsFencedAndCopyable(t *testing.T) {
	g := security.NewGuardian(slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := `{"status":"ok","issues":[{"code":"prompt_missing","scene_id":"s1","message":"Scene s1 has no video prompt."}]}`
	for _, name := range []string{VideoStudioPlanToolRead, VideoStudioPlanToolWrite, VideoStudioPlanToolPatchScene, VideoStudioPlanToolCheck, VideoStudioPlanToolFinish} {
		got := g.SanitizeToolOutput(name, body)
		if !strings.HasPrefix(got, "<external_data>\n") || !strings.Contains(got, `"scene_id":"s1"`) || strings.Contains(got, "&#34;") {
			t.Fatalf("%s output is not fenced and copyable: %q", name, got)
		}
	}
}
