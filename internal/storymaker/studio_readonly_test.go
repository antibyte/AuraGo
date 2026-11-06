package storymaker

import (
	"context"
	"encoding/json"
	"testing"
)

// fixtureStudio returns a studio on a new project of 60 seconds with the mini catalog.
func fixtureStudio(t *testing.T, readOnly func() bool) (*Studio, *ProjectStore, string) {
	t.Helper()
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(context.Background(), "Was ist eine Firewall?", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewStudio(StudioDeps{Projects: ps, Catalog: loadMini(t), ReadOnly: readOnly}, StudioJob{
		JobID: "job-1", ProjectID: proj.ID, Kind: AgentKindCreate, Language: "de", Audience: "Einsteiger", TargetSeconds: 60, VoiceAvailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, ps, proj.ID
}

// answerOf decodes the model-facing answer and checks that it agrees with the result status.
func answerOf(t *testing.T, res StudioResult) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(res.JSON, &out); err != nil {
		t.Fatalf("answer %q: %v", res.JSON, err)
	}
	if out["status"] != res.Status {
		t.Fatalf("status field %v, result status %q", out["status"], res.Status)
	}
	return out
}

func TestStudioRefusesWritesWhenReadOnly(t *testing.T) {
	ctx := context.Background()
	s, ps, id := fixtureStudio(t, func() bool { return true })
	before, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ phase, operation string }{
		{PhaseOutline, "outline"},
		{PhaseCast, "set_meta"},
		{PhaseFill, "put_scene"},
		{PhaseFill, "patch_scene"},
		{PhaseCheck, "patch_scene"},
		{PhaseRevise, "remove_scene"},
		{PhaseRevise, "move_scene"},
	} {
		s.SetPhase(tc.phase, "s1")
		res := s.Call(ctx, StudioCall{Tool: ToolStory, Operation: tc.operation, Args: map[string]any{"scene_id": "s1", "data": map[string]any{}}})
		answer := answerOf(t, res)
		if res.Status != "refused" || answer["code"] != "desktop_readonly" || res.Operation != tc.operation {
			t.Errorf("%s/%s: %s", tc.phase, tc.operation, res.JSON)
		}
	}
	after, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("a refused write changed the story:\n%s", after)
	}
	s.SetPhase(PhaseOutline, "")
	if answer := answerOf(t, s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "get"})); answer["code"] == "desktop_readonly" {
		t.Errorf("reading must stay possible: %v", answer)
	}
}
