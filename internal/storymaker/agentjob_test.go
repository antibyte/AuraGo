package storymaker

import (
	"bytes"
	"context"
	"path"
	"reflect"
	"testing"
)

func TestDecideOutcomeTable(t *testing.T) {
	done := StudioProgress{ScenesTotal: 2, ScenesDone: 2}
	layout := AgentCheck{Code: CodeLayoutUnverified}
	cases := []struct {
		name    string
		in      OutcomeInput
		outcome string
		checks  []AgentCheck
	}{
		{name: "no outline", in: OutcomeInput{Progress: StudioProgress{ScenesTotal: 0}}, outcome: OutcomeFailed, checks: []AgentCheck{layout}},
		{name: "clean, advice is not listed", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Advice: []Issue{{Severity: SeverityWarning, Code: CodeCueLate, SceneID: "s1"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout}},
		{name: "a craft finding is listed and never blocks", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Issues: []Issue{{Severity: SeverityWarning, Code: CodeEnvironmentMonotony}, {Severity: SeverityWarning, Code: CodeCastUnused, SceneID: "s3"}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout, {Code: CodeEnvironmentMonotony}, {Code: CodeCastUnused, SceneID: "s3"}}},
		{name: "a real model-fixable error still blocks next to a craft finding", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Issues: []Issue{{Severity: SeverityWarning, Code: CodeGestureMonotony}, {Severity: SeverityError, Code: CodeSceneUnfilled, SceneID: "s2"}}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeGestureMonotony}, {Code: CodeSceneUnfilled, SceneID: "s2"}}},
		{name: "timeout", in: OutcomeInput{Progress: done, TimedOut: true}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout}},
		{name: "model error", in: OutcomeInput{Progress: done, ModelError: true}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: "model_error"}}},
		{name: "unfilled scene", in: OutcomeInput{Progress: StudioProgress{ScenesTotal: 2, ScenesDone: 1, Unfilled: []string{"s2"}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeSceneUnfilled, SceneID: "s2"}}},
		{name: "model-fixable issue", in: OutcomeInput{Progress: done, Check: CheckReport{Issues: []Issue{{Severity: SeverityError, Code: CodeUnknownName, SceneID: "s1"}}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeUnknownName, SceneID: "s1"}}},
		{name: "compile failed", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "failed"}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeCompileFailed}}},
		{name: "environment never downgrades", in: OutcomeInput{Progress: done, Narration: "narration_unavailable", Check: CheckReport{Compile: "unavailable", Environment: []Issue{{Severity: SeverityWarning, Code: CodeOverTarget}}}}, outcome: OutcomeReadyUnverified, checks: []AgentCheck{layout, {Code: "compile_unverified"}, {Code: "narration_unavailable"}, {Code: CodeOverTarget}}},
		{name: "environment error blocks by its code", in: OutcomeInput{Progress: done, Check: CheckReport{Compile: "ok", Environment: []Issue{{Severity: SeverityError, Code: CodeDeadline}}}}, outcome: OutcomeIncomplete, checks: []AgentCheck{layout, {Code: CodeDeadline}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, checks := DecideOutcome(tc.in)
			if outcome != tc.outcome {
				t.Errorf("outcome = %q, want %q", outcome, tc.outcome)
			}
			if !reflect.DeepEqual(checks, tc.checks) {
				t.Errorf("checks = %+v, want %+v", checks, tc.checks)
			}
		})
	}
}

const silentFixture = `{"schema":"storymaker/1","title":"Firewall",
 "cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],
 "scenes":[
  {"id":"s1","environment":"env.office","narration":"A firewall {1} checks every packet.",
   "actors":[{"cast":"anna","at":"left"}],
   "beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue":1},
            {"do":"say","who":"anna","text":"This is {2} a packet.","style":"speech"}]},
  {"id":"s2","actors":[{"cast":"anna"}],
   "beats":[{"do":"say","who":"anna","text":"Only one line.","style":"speech"}]}]}`

func TestMakeSilentPassesTheAudioGate(t *testing.T) {
	ps, mem, _ := newTestStore(t)
	ctx := context.Background()
	proj, _, _, err := ps.Create(ctx, "Firewall", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := DecodeStory([]byte(silentFixture), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	ApplyDefaults(&story)
	before, err := ps.AudioAnswer(ctx, proj.ID, &story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Missing) != 3 { // the narration of s1 and the say beats of s1 and s2
		t.Fatalf("missing before = %+v", before.Missing)
	}
	backup, err := CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.SaveSilentBackup(ctx, proj.ID, backup); err != nil {
		t.Fatal(err)
	}

	if changed := MakeSilent(&story); changed != 2 {
		t.Fatalf("changed = %d, want 2", changed)
	}
	if again := MakeSilent(&story); again != 0 {
		t.Fatalf("a second call changed %d scenes", again)
	}
	ApplyDefaults(&story)
	after, err := ps.AudioAnswer(ctx, proj.ID, &story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Missing) != 0 {
		t.Fatalf("missing after = %+v", after.Missing)
	}
	issues := Validate(&story, loadReal(t), DefaultLimits())
	if HasErrors(issues) {
		t.Fatalf("issues = %+v", issues)
	}
	for _, is := range issues {
		if is.Code == CodeUnknownCue {
			t.Errorf("a cue survived: %+v", is)
		}
	}
	wantSeconds := []float64{
		max(1, float64(WordCount("A firewall checks every packet.")+WordCount("This is a packet."))/2.3+0.4),
		max(1, float64(WordCount("Only one line."))/2.3+0.4),
	}
	for i := range story.Scenes {
		sc := &story.Scenes[i]
		if !sc.Silent || sc.Duration == nil || sc.Duration.Seconds != wantSeconds[i] {
			t.Errorf("scene %s: silent %v duration %+v, want %v seconds", sc.ID, sc.Silent, sc.Duration, wantSeconds[i])
		}
		for j := range sc.Beats {
			if sc.Beats[j].Do == "say" || sc.Beats[j].Cue != 0 {
				t.Errorf("scene %s beat %d = %+v", sc.ID, j, sc.Beats[j])
			}
		}
	}
	shown := story.Scenes[0].Beats[1]
	if shown.Do != "show" || shown.Text != "This is a packet." || shown.At.String() != "center" || shown.Entry != "fade" {
		t.Errorf("the say became %+v", shown)
	}

	// The speech can come back from the copy next to story.json.
	stored, _, err := mem.ReadFile(ctx, path.Join(testRoot, proj.ID, "story.before-silent.json"))
	if err != nil || !bytes.Equal(stored, backup) {
		t.Fatalf("stored copy: %v", err)
	}
	data, err := ps.LoadSilentBackup(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	restored, _, err := DecodeStory(data, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	spoken, err := ps.AudioAnswer(ctx, proj.ID, &restored, nil)
	if err != nil || len(spoken.Missing) != 3 {
		t.Fatalf("restored story: missing %+v, error %v", spoken.Missing, err)
	}
	if err := ps.RemoveSilentBackup(ctx, proj.ID); err != nil {
		t.Fatal(err)
	}
	if has, err := ps.HasSilentBackup(ctx, proj.ID); has || err != nil {
		t.Fatalf("has backup = %v, error %v", has, err)
	}
}
