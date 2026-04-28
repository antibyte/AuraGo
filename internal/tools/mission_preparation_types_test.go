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

func TestStripMissionExecutionPlanAdvisoryKeepsUserAuthoredHeading(t *testing.T) {
	prompt := strings.Join([]string{
		"Document this section.",
		"",
		"## Mission Execution Plan (Advisory)",
		"This is a user-authored heading in a document, not an AuraGo advisory block.",
		"",
		"Keep this text.",
	}, "\n")

	cleaned := StripMissionExecutionPlanAdvisory(prompt)

	if cleaned != prompt {
		t.Fatalf("user-authored advisory heading was stripped:\n%s", cleaned)
	}
}

func TestRenderPreparedContextUsesStripMarkers(t *testing.T) {
	pm := &PreparedMission{
		Status:     PrepStatusPrepared,
		PreparedAt: time.Now(),
		Analysis: &PreparationAnalysis{
			Summary: "Use the current state.",
		},
	}

	rendered := pm.RenderPreparedContext()
	if !strings.Contains(rendered, missionExecutionPlanStartMarker) || !strings.Contains(rendered, missionExecutionPlanEndMarker) {
		t.Fatalf("rendered advisory lacks strip markers:\n%s", rendered)
	}
	cleaned := StripMissionExecutionPlanAdvisory("Original prompt" + rendered)
	if cleaned != "Original prompt" {
		t.Fatalf("marked advisory was not stripped cleanly:\n%s", cleaned)
	}
}

func TestStripMissionExecutionPlanAdvisoryRemovesMultipleStalePlans(t *testing.T) {
	prompt := strings.Join([]string{
		"Check world news.",
		"",
		"---",
		"## Mission Execution Plan (Advisory)",
		"Scheduler-generated guidance for organizing this mission.",
		"old plan one",
		"",
		"---",
		"",
		"---",
		"## Mission Execution Plan (Advisory)",
		"Scheduler-generated guidance for organizing this mission.",
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
