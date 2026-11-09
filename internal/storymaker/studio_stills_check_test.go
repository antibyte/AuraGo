package storymaker

import (
	"context"
	"testing"
)

func TestCheckReportsScenesThatLookAlike(t *testing.T) {
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{
			Available: true, DurationMS: 8000, StoryboardScenes: 3,
			Stills: []StillInfo{{SceneID: "s1", Index: 0, Hash: 0}, {SceneID: "s2", Index: 1, Hash: 1}, {SceneID: "s3", Index: 2, Hash: 3}},
		}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, is := range report.Issues {
		if is.Code == CodeScenesLookAlike && is.SceneID == "s3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues = %+v", report.Issues)
	}
	outcome, _ := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	if outcome == OutcomeIncomplete {
		t.Fatalf("a look-alike finding must not block: %s", outcome)
	}
}
