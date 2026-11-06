package storymaker

import (
	"context"
	"encoding/json"
	"testing"
)

// newStudioWriteRig makes a project with a cast and two outline scenes, s1 and s2.
func newStudioWriteRig(t *testing.T, compile func(context.Context, string) (CompileSummary, error)) (*Studio, *ProjectStore, string) {
	t.Helper()
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	proj, story, _, err := ps.Create(ctx, "Firewall", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	story.Cast = []CastMember{{ID: "anna", Name: "Anna", Character: "char.office-woman-1"}}
	story.Scenes = []Scene{
		{ID: "s1", Narration: "Eine Firewall schuetzt das Netz."},
		{ID: "s2", Narration: "Sie prueft jedes Paket."},
	}
	ApplyDefaults(&story)
	data, err := CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, proj.ID, data, ""); err != nil {
		t.Fatal(err)
	}
	if compile == nil {
		compile = func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		}
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Voices:   func(context.Context) []VoiceProvider { return nil },
		Compile:  compile,
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: "create", Language: "de", Audience: "general", TargetSeconds: 60, VoiceAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	return studio, ps, proj.ID
}

func studioWriteCall(t *testing.T, studio *Studio, tool, op, args string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		t.Fatalf("args %s: %v", args, err)
	}
	res := studio.Call(context.Background(), StudioCall{Tool: tool, Operation: op, Args: parsed})
	var answer map[string]any
	if err := json.Unmarshal(res.JSON, &answer); err != nil {
		t.Fatalf("%s answer %s: %v", op, res.JSON, err)
	}
	return answer
}

func TestStudioDraftLifecycleAndCharging(t *testing.T) {
	ctx := context.Background()
	studio, ps, id := newStudioWriteRig(t, nil)
	_, before, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	limit := StudioSceneChargedLimit
	wantDraft := func(answer map[string]any, left int) {
		t.Helper()
		if answer["status"] != "draft" || answer["draft"] != true || answer["corrections_left"] != float64(left) {
			t.Fatalf("answer = %v, want a draft with %d corrections left", answer, left)
		}
	}
	patch := func(sceneID, data string) map[string]any {
		t.Helper()
		return studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"`+sceneID+`","data":`+data+`}`)
	}

	// Two errors: an unknown cast and an unknown asset. A first write is free.
	const bad = `"data":{"environment":"env.office","actors":[{"cast":"ghost"}],"beats":[{"do":"show","item":"prop.nope","at":"right","entry":"pop"}]}`
	studio.SetPhase(PhaseFill, "s1")
	wantDraft(studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1",`+bad+`}`), limit)
	// The same two errors are charged.
	wantDraft(patch("s1", `{"variant":"night"}`), limit-1)
	// One error fewer is free.
	wantDraft(patch("s1", `{"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]}`), limit-1)
	// The same one error is charged until the budget is gone.
	wantDraft(patch("s1", `{"variant":"day"}`), limit-2)
	wantDraft(patch("s1", `{"variant":"night"}`), limit-3)
	if answer := patch("s1", `{"variant":"day"}`); answer["code"] != "corrections_exhausted" {
		t.Fatalf("answer = %v, want corrections_exhausted", answer)
	}
	if _, now, _ := ps.LoadStory(ctx, id); now != before {
		t.Fatal("a draft reached story.json")
	}
	if studio.PhaseDone() {
		t.Fatal("the fill phase ended on a draft")
	}

	// A patch that fixes the last error is still accepted, merges into the draft and saves it.
	answer := patch("s1", `{"actors":[{"cast":"anna","at":"left"}]}`)
	data, etag, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if answer["status"] != "ok" || answer["rev"] != studioRev(etag) {
		t.Fatalf("answer = %v, want ok with the current rev", answer)
	}
	story, _, err := DecodeStory(data, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	s1 := story.Scenes[0]
	if s1.Actors[0].Cast != "anna" || s1.Beats[0].Item != "prop.lock" || s1.Narration != "Eine Firewall schuetzt das Netz." {
		t.Fatalf("saved scene = %+v", s1)
	}
	if !studio.PhaseDone() {
		t.Fatal("the fill phase did not end on the saved scene")
	}

	// A new scene starts free, and remove_scene discards its draft.
	studio.SetPhase(PhaseRevise, "")
	wantDraft(studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s3",`+bad+`}`), limit)
	if answer := studioWriteCall(t, studio, ToolStory, "remove_scene", `{"scene_id":"s3"}`); answer["status"] != "ok" {
		t.Fatalf("remove answer = %v", answer)
	}
	if answer := patch("s3", `{"variant":"day"}`); answer["code"] != "scene_not_found" {
		t.Fatalf("answer = %v, want scene_not_found after the draft was removed", answer)
	}

	// Dry runs save nothing and have their own cap.
	_, beforeDry, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(PhaseFill, "s2")
	const dry = `{"op":"put_scene","scene_id":"s2","data":{"environment":"env.office"}}`
	for range StudioDryRunsPerScene {
		answer := studioWriteCall(t, studio, ToolStory, "dry_run", dry)
		applied, _ := answer["defaults_applied"].([]any)
		if answer["status"] != "ok" || len(applied) == 0 {
			t.Fatalf("dry run answer = %v", answer)
		}
	}
	if answer := studioWriteCall(t, studio, ToolStory, "dry_run", dry); answer["code"] != "dry_runs_exhausted" {
		t.Fatalf("answer = %v, want dry_runs_exhausted", answer)
	}
	if _, now, _ := ps.LoadStory(ctx, id); now != beforeDry {
		t.Fatal("a dry run reached story.json")
	}
}
