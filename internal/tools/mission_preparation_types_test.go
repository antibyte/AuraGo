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
