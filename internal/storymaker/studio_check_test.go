package storymaker

import (
	"context"
	"testing"
)

func TestStudioCheckCueTimesAreFilmTime(t *testing.T) {
	story := &Story{Scenes: []Scene{{ID: "s3"}}}
	summary := CompileSummary{Available: true, DurationMS: 8000, Scenes: []CompileScene{
		{ID: "s3", StartMS: 4000, EndMS: 8000, CuesMS: []CompileCue{{Marker: 1, AtMS: 700}}},
	}}
	rows, _ := checkTiming(story, summary)
	if len(rows) != 1 || len(rows[0].CueTimes) != 1 || rows[0].CueTimes[0] != 4.7 {
		t.Fatalf("rows = %+v, want the cue at film time 4.7", rows)
	}
}

func TestStudioCheckFindsUnfilledAndEnvironmentNeverBlocks(t *testing.T) {
	ctx := context.Background()
	compiles := 0
	studio, _, _ := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		compiles++
		return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
	})

	studio.SetPhase(PhaseFill, "s1")
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","actors":[{"cast":"anna","at":"left","pose":"present"}]}}`)
	if put["status"] != "ok" {
		t.Fatalf("put_scene answer = %v", put)
	}

	// The outline scene s2 passes Validate, but the job lint finds it.
	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != CodeSceneUnfilled || report.Issues[0].SceneID != "s2" {
		t.Fatalf("issues = %+v, want scene_unfilled on s2", report.Issues)
	}
	if report.Compile != "unavailable" || len(report.Environment) != 1 || report.Environment[0].Code != "compile_unverified" {
		t.Fatalf("compile = %q, environment = %+v", report.Compile, report.Environment)
	}
	if report.TotalSeconds <= 0 {
		t.Fatalf("total = %v, want the estimate", report.TotalSeconds)
	}
	progress, err := studio.Progress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if progress.ScenesTotal != 2 || progress.ScenesDone != 1 || len(progress.Unfilled) != 1 || progress.Unfilled[0] != "s2" {
		t.Fatalf("progress = %+v", progress)
	}

	// The same story version is not compiled again, and the open issue keeps the phase running.
	studio.SetPhase(PhaseCheck, "")
	open := studioWriteCall(t, studio, ToolCheck, "validate", `{}`)
	if open["clean"] != false || open["rounds_left"] != float64(StudioCheckRounds-1) {
		t.Fatalf("validate answer = %v", open)
	}
	if compiles != 1 {
		t.Fatalf("compiled %d times, want the cached result", compiles)
	}
	if studio.PhaseDone() {
		t.Fatal("the check phase ended with an open issue")
	}

	patch := studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s2","data":{"actors":[{"cast":"anna","at":"left"}]}}`)
	if patch["status"] != "ok" {
		t.Fatalf("patch_scene answer = %v", patch)
	}

	// The environment item stays listed, is not the model's to fix and does not hold the phase.
	done := studioWriteCall(t, studio, ToolCheck, "validate", `{}`)
	environment, _ := done["environment"].([]any)
	if done["clean"] != true || done["compile"] != "unavailable" || done["rounds_left"] != float64(StudioCheckRounds-2) || len(environment) != 1 {
		t.Fatalf("validate answer = %v", done)
	}
	item := environment[0].(map[string]any)
	if item["code"] != "compile_unverified" || item["model_fixable"] != false {
		t.Fatalf("environment item = %v", item)
	}
	if compiles != 2 {
		t.Fatalf("compiled %d times, want a new compile for the changed story", compiles)
	}
	if !studio.PhaseDone() {
		t.Fatal("an environment item held the check phase open")
	}
}
