package tools

import (
	"strings"
	"testing"
	"time"
)

func TestRenderPreparedContextAvoidsInstructionOverrideWording(t *testing.T) {
	pm := &PreparedMission{
		Status:     PrepStatusPrepared,
		PreparedAt: time.Now(),
		Analysis: &PreparationAnalysis{
			Summary: "Check the page and report findings.",
			StepPlan: []StepGuide{{
				Step:   1,
				Action: "Inspect the current state",
			}},
		},
	}

	rendered := pm.RenderPreparedContext()
	for _, forbidden := range []string{
		"Above is your original mission prompt",
		"follow it as the primary instruction",
		"primary instruction",
	} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("prepared mission context should avoid instruction-override wording %q, got:\n%s", forbidden, rendered)
		}
	}
}

func TestStripMissionExecutionPlanAdvisoryRemovesStalePlan(t *testing.T) {
	prompt := strings.Join([]string{
		"Create a funny image.",
		"",
		"---",
		"## Mission Execution Plan (Advisory)",
		"Scheduler-generated guidance for organizing this mission.",
		"",
		"### Summary",
		"Old plan",
		"",
		"---",
	}, "\n")

	cleaned := StripMissionExecutionPlanAdvisory(prompt)

	if strings.TrimSpace(cleaned) != "Create a funny image." {
		t.Fatalf("unexpected cleaned prompt:\n%s", cleaned)
	}
	if strings.Contains(cleaned, "Mission Execution Plan") || strings.Contains(cleaned, "Old plan") {
		t.Fatalf("stale advisory block was not removed:\n%s", cleaned)
	}
}

func TestStripMissionExecutionPlanAdvisoryRemovesMultipleStalePlans(t *testing.T) {
	prompt := strings.Join([]string{
		"Check world news.",
		"",
		"---",
		"## Mission Execution Plan (Advisory)",
		"old plan one",
		"",
		"---",
		"",
		"---",
		"## Mission Execution Plan (Advisory)",
		"old plan two",
		"",
		"---",
		"",
		"Keep this user-authored note.",
	}, "\n")

	cleaned := StripMissionExecutionPlanAdvisory(prompt)

	if !strings.Contains(cleaned, "Check world news.") || !strings.Contains(cleaned, "Keep this user-authored note.") {
		t.Fatalf("cleaned prompt lost user content:\n%s", cleaned)
	}
	if strings.Contains(cleaned, "old plan one") || strings.Contains(cleaned, "old plan two") {
		t.Fatalf("cleaned prompt still contains stale plans:\n%s", cleaned)
	}
}
