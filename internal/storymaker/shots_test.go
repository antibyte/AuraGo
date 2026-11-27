package storymaker

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"reflect"
	"testing"
)

func shotRig(t *testing.T, phase string, screenshot func(context.Context, string, []ShotRequest, bool) (CompileSummary, error)) *Studio {
	t.Helper()
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	proj, story, _, err := ps.Create(ctx, "Firewall", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	story.Scenes = []Scene{{ID: "s1", Narration: "Eins."}, {ID: "s2", Narration: "Zwei."}}
	ApplyDefaults(&story)
	data, err := CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, proj.ID, data, ""); err != nil {
		t.Fatal(err)
	}
	studio, err := NewStudio(StudioDeps{Projects: ps, Catalog: loadReal(t), Limits: DefaultLimits(), Screenshot: screenshot},
		StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Language: "de", TargetSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(phase, "")
	return studio
}

func viewCall(t *testing.T, s *Studio, args map[string]any) map[string]any {
	t.Helper()
	res := s.Call(context.Background(), StudioCall{Tool: ToolView, Args: args})
	return answerOf(t, res)
}

func TestShotImageDownscalesToJPEG(t *testing.T) {
	for _, tc := range []struct{ w, h, wantW, wantH int }{{2560, 1440, 1280, 720}, {640, 360, 640, 360}, {900, 1600, 720, 1280}} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))); err != nil {
			t.Fatal(err)
		}
		data, w, h, err := ShotImage(buf.Bytes())
		if err != nil || w != tc.wantW || h != tc.wantH {
			t.Fatalf("%dx%d: got %dx%d, %v", tc.w, tc.h, w, h, err)
		}
		if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
			t.Fatalf("not a JPEG: %v", err)
		}
	}
	if _, _, _, err := ShotImage([]byte("not a png")); err == nil {
		t.Error("a non-PNG must fail")
	}
}

func TestStudioScreenshot(t *testing.T) {
	okSummary := CompileSummary{Available: true, Shots: []SceneShot{{SceneID: "s1", AtMS: 5640, Width: 640, Height: 360, Image: []byte("jpg")}}}
	t.Run("refused without vision and outside check or revise", func(t *testing.T) {
		for _, tc := range []struct {
			phase string
			deps  func(context.Context, string, []ShotRequest, bool) (CompileSummary, error)
		}{
			{PhaseCheck, nil},
			{PhaseFill, func(context.Context, string, []ShotRequest, bool) (CompileSummary, error) { return okSummary, nil }},
		} {
			s := shotRig(t, tc.phase, tc.deps)
			got := viewCall(t, s, map[string]any{"scene_ids": []any{"s1"}})
			if got["status"] != "refused" || got["code"] != "operation_not_in_phase" {
				t.Errorf("%s: %v", tc.phase, got)
			}
		}
	})
	t.Run("refuses bad arguments before the player", func(t *testing.T) {
		calls := 0
		s := shotRig(t, PhaseCheck, func(context.Context, string, []ShotRequest, bool) (CompileSummary, error) {
			calls++
			return okSummary, nil
		})
		for name, tc := range map[string]struct {
			args map[string]any
			code string
		}{
			"unknown scene": {map[string]any{"scene_ids": []any{"nope"}}, "unknown_scene"},
			"five scenes":   {map[string]any{"scene_ids": []any{"s1", "s2", "s3", "s4", "s5"}}, "too_many_scenes"},
			"at_s, two":     {map[string]any{"scene_ids": []any{"s1", "s2"}, "at_s": 2.0}, "invalid_arguments"},
			"at_s, none":    {map[string]any{"at_s": 2.0}, "invalid_arguments"},
			"negative at_s": {map[string]any{"scene_id": "s1", "at_s": -1.0}, "invalid_arguments"},
		} {
			if got := viewCall(t, s, tc.args); got["status"] != "refused" || got["code"] != tc.code {
				t.Errorf("%s: %v", name, got)
			}
		}
		if calls != 0 {
			t.Errorf("%d calls reached the player", calls)
		}
	})
	t.Run("success hands out the images once, and the overview asks for a sheet", func(t *testing.T) {
		var gotReqs [][]ShotRequest
		var gotSheet []bool
		s := shotRig(t, PhaseRevise, func(_ context.Context, _ string, reqs []ShotRequest, sheet bool) (CompileSummary, error) {
			gotReqs, gotSheet = append(gotReqs, reqs), append(gotSheet, sheet)
			if sheet {
				return CompileSummary{Available: true, ShotSheet: []byte("sheet"), Shots: []SceneShot{{SceneID: "s1"}, {SceneID: "s2"}}}, nil
			}
			return okSummary, nil
		})
		got := viewCall(t, s, map[string]any{"scene_id": "s1", "at_s": 5.6})
		if got["status"] != "ok" || got["operation"] != "screenshot" || got["sheet"] != false {
			t.Fatalf("single: %v", got)
		}
		shots, _ := got["shots"].([]any)
		if len(shots) != 1 || shots[0].(map[string]any)["at_s"] != 5.6 || shots[0].(map[string]any)["width"] != 640.0 {
			t.Fatalf("shots %v", got["shots"])
		}
		images := s.TakeImages()
		if len(images) != 1 || images[0].Label != "Scene s1 at 5.6 s" || images[0].MIME != "image/jpeg" {
			t.Fatalf("images %+v", images)
		}
		if again := s.TakeImages(); len(again) != 0 {
			t.Fatalf("images must be cleared: %+v", again)
		}
		if want := (ShotRequest{SceneID: "s1", AtS: 5.6, HasAt: true}); !reflect.DeepEqual(gotReqs[0], []ShotRequest{want}) {
			t.Errorf("request %+v", gotReqs[0])
		}
		got = viewCall(t, s, map[string]any{})
		if got["status"] != "ok" || got["sheet"] != true || got["order"] != "left to right, top to bottom" {
			t.Fatalf("overview: %v", got)
		}
		if !gotSheet[1] || !reflect.DeepEqual(gotReqs[1], []ShotRequest{{SceneID: "s1"}, {SceneID: "s2"}}) {
			t.Errorf("overview request %+v sheet %v", gotReqs[1], gotSheet[1])
		}
		if images := s.TakeImages(); len(images) != 1 || images[0].Label != "Overview sheet: s1, s2 (left to right, top to bottom)" {
			t.Fatalf("overview images %+v", images)
		}
	})
	t.Run("budget", func(t *testing.T) {
		s := shotRig(t, PhaseCheck, func(context.Context, string, []ShotRequest, bool) (CompileSummary, error) { return okSummary, nil })
		for i := 0; i < StudioScreenshotsPerJob; i++ {
			if got := viewCall(t, s, map[string]any{"scene_ids": []any{"s1"}}); got["status"] != "ok" {
				t.Fatalf("look %d: %v", i+1, got)
			}
		}
		if got := viewCall(t, s, map[string]any{"scene_ids": []any{"s1"}}); got["status"] != "refused" || got["code"] != "screenshot_budget" {
			t.Fatalf("over budget: %v", got)
		}
	})
	t.Run("unavailable, compile failure and no picture", func(t *testing.T) {
		summary := CompileSummary{Available: false, Reason: "browser_unavailable"}
		s := shotRig(t, PhaseCheck, func(context.Context, string, []ShotRequest, bool) (CompileSummary, error) { return summary, nil })
		if got := viewCall(t, s, map[string]any{}); got["status"] != "unavailable" || got["next_action"] != "Continue without pictures." {
			t.Errorf("unavailable: %v", got)
		}
		summary = CompileSummary{Available: true, Issues: []Issue{{Severity: SeverityError, Code: CodeCompileFailed, Message: "boom"}}}
		if got := viewCall(t, s, map[string]any{}); got["status"] != "refused" || got["code"] != "compile_failed" {
			t.Errorf("compile failure: %v", got)
		}
		summary = CompileSummary{Available: true}
		if got := viewCall(t, s, map[string]any{}); got["status"] != "error" || got["code"] != "no_picture" {
			t.Errorf("no picture: %v", got)
		}
	})
}
