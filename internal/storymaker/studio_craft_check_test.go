package storymaker

import (
	"context"
	"os"
	"slices"
	"testing"
)

// saveFixture replaces the story of the rig's project with a fixture file.
func saveFixture(t *testing.T, ps *ProjectStore, projectID, name string) {
	t.Helper()
	data, err := os.ReadFile("testdata/stories/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(context.Background(), projectID, data, ""); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsCraftFindingsForACreateJobOnly(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-firewall-run")

	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, is := range report.Issues {
		codes = append(codes, is.Code)
	}
	for _, want := range []string{CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused} {
		if !slices.Contains(codes, want) {
			t.Errorf("issues = %v, missing %s", codes, want)
		}
	}
	outcome, checks := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	if outcome != OutcomeReadyUnverified {
		t.Errorf("outcome = %s (%v): craft findings must not block", outcome, checks)
	}

	studio.job.Kind = AgentKindRevise
	report, err = studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range report.Issues {
		if IssueCraft(is.Code) {
			t.Errorf("a revise job must not get %s", is.Code)
		}
	}
}

func TestCheckLeavesTheVariedStoryClean(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range report.Issues {
		if IssueCraft(is.Code) {
			t.Errorf("the varied story must pass the craft lint, got %+v", is)
		}
	}
}
