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

func TestStudioPacketContinuityAndSuggestions(t *testing.T) {
	ctx := context.Background()
	studio, _, _ := newStudioWriteRig(t, nil)

	studio.SetPhase(PhaseOutline, "")
	outlinePacket, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	envs, _ := outlinePacket["environments"].([]string)
	if len(envs) == 0 {
		t.Fatalf("outline packet lacks environments: %v", outlinePacket)
	}

	studio.SetPhase(PhaseCast, "")
	castPacket, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	looks, _ := castPacket["character_looks"].([]map[string]any)
	styleOpts, _ := castPacket["style_options"].(map[string]any)
	if len(looks) == 0 || styleOpts == nil {
		t.Fatalf("cast packet lacks character_looks or style_options: %v", castPacket)
	}

	studio.SetPhase(PhaseFill, "s1")
	put := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","speaker":"anna","narration":"Eine {1}Firewall schuetzt das Netz mit einem Schloss.","actors":[{"cast":"anna","at":"left","pose":"present","expression":"happy"}],"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1}]}}`)
	if put["status"] != "ok" {
		t.Fatalf("put_scene s1 = %v", put)
	}

	fillS1, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	nextS2, _ := fillS1["next_scene"].(map[string]any)
	suggested, _ := fillS1["suggested_props"].([]string)
	if nextS2["id"] != "s2" || len(suggested) == 0 {
		t.Fatalf("fill s1 packet next_scene=%v suggested_props=%v", nextS2, suggested)
	}

	studio.SetPhase(PhaseFill, "s2")
	fillS2, err := studio.Packet(ctx, true) // compact mode still carries continuity
	if err != nil {
		t.Fatal(err)
	}
	prevS1, _ := fillS2["previous_scene"].(map[string]any)
	usedEntries, _ := fillS2["used_entries"].([]string)
	example, _ := fillS2["example"].(map[string]any)
	if prevS1["id"] != "s1" || prevS1["speaker"] != "anna" || len(usedEntries) != 1 || usedEntries[0] != "pop" || example["speaker"] != "anna" {
		t.Fatalf("fill s2 compact packet prev=%v entries=%v example=%v", prevS1, usedEntries, example)
	}
}

func TestStudioCheckFindsStageOverlapMonotonyAndCompileWarnings(t *testing.T) {
	ctx := context.Background()
	studio, _, _ := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{
			Available:  true,
			DurationMS: 8000,
			Issues: []Issue{
				{Severity: SeverityWarning, SceneID: "s1", Path: "/scenes/0/beats/0", Code: CodeLineUnread, Message: "Line unread."},
			},
		}, nil
	})

	studio.SetPhase(PhaseFill, "s1")
	// Actor anna at "left" and show prop.lock also at "left" -> immediate spot_overlap warning on put_scene.
	putS1 := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s1","data":{"environment":"env.office","actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","item":"prop.lock","at":"left","entry":"pop"},{"do":"show","item":"prop.shield","at":"right","entry":"pop"},{"do":"show","item":"prop.key","at":"center","entry":"pop"}]}}`)
	if putS1["status"] != "ok" {
		t.Fatalf("put_scene s1 = %v", putS1)
	}
	warnings, _ := putS1["issues"].([]any)
	if len(warnings) != 1 || warnings[0].(map[string]any)["code"] != CodeSpotOverlap {
		t.Fatalf("put_scene s1 warnings = %v, want spot_overlap", warnings)
	}

	// Walking away before showing at "left" resolves spot_overlap in s2, while fade across environments triggers advice; s1 holds the third "pop" in a row.
	studio.SetPhase(PhaseFill, "s2")
	putS2 := studioWriteCall(t, studio, ToolStory, "put_scene",
		`{"scene_id":"s2","data":{"environment":"env.server-room","transition":"fade","actors":[{"cast":"anna","at":"left","pose":"stand"}],"beats":[{"do":"walk","who":"anna","to":"right"},{"do":"show","item":"prop.shield","at":"left","entry":"pop"}]}}`)
	if putS2["status"] != "ok" {
		t.Fatalf("put_scene s2 = %v", putS2)
	}
	if s2Warn, _ := putS2["issues"].([]any); len(s2Warn) != 0 {
		t.Fatalf("put_scene s2 warnings = %v, want none after walk", s2Warn)
	}

	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seenCodes := map[string]bool{}
	for _, adv := range report.Advice {
		seenCodes[adv.Code] = true
	}
	for _, want := range []string{CodeSpotOverlap, CodeFadeBetweenWorlds, CodeEntryMonotony, CodeLineUnread} {
		if !seenCodes[want] {
			t.Errorf("advice = %+v, missing %q", report.Advice, want)
		}
	}
}

func TestLintForJobEntryStreakResetsPerScene(t *testing.T) {
	show := func(entry string) Beat { return Beat{Do: "show", Item: "prop.lock", At: SpotAt("right"), Entry: entry} }
	actors := []Actor{{Cast: "anna", At: SpotAt("left")}}
	story := &Story{Scenes: []Scene{
		{ID: "s1", Actors: actors, Beats: []Beat{show("pop"), show("pop")}},
		{ID: "s2", Actors: actors, Beats: []Beat{show("pop"), show("pop")}},
		{ID: "s3", Actors: actors, Beats: []Beat{show("fade"), show("fade"), show("fade"), show("pop"), show("pop"), show("pop")}},
	}}
	var got []string
	for _, is := range LintForJob(story) {
		if is.Code == CodeEntryMonotony {
			got = append(got, is.SceneID)
		}
	}
	if len(got) != 2 || got[0] != "s3" || got[1] != "s3" {
		t.Fatalf("entry_monotony scenes = %v, want s3 twice (two streaks of three) and nothing for two scenes of two", got)
	}
}
