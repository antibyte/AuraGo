package gamemaker

import (
	"strings"
	"testing"
)

// A hidden or backgrounded Studio window is not a game defect: the check is
// unavailable, never repairable, and the job ends with the preview code without
// spending a repair pass. A game that paused itself ("inactive") stays repairable.
func TestHostHiddenTargetRunIsUnavailableAndNotRepairable(t *testing.T) {
	scenario := GameScenario{ID: "required_rules", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Mode: "reach", Target: "item", MS: 4000}}}
	for name, run := range map[string]TargetRun{
		"hidden before the first sample": {Target: "item", Mode: "reach", Reason: "host_hidden"},
		"hidden after real input":        {Target: "item", Mode: "reach", Samples: 30, Inputs: 12, Contacts: 3, Reason: "host_hidden"},
	} {
		t.Run(name, func(t *testing.T) {
			observations := []GameObservation{{ID: scenario.ID, Before: map[string]float64{"pickup_events": 0}, After: map[string]float64{"pickup_events": 0}, TargetRuns: []TargetRun{run}}}
			if err := validateGameReport(PreviewReport{Observations: observations}); err != nil {
				t.Fatalf("host_hidden evidence rejected: %v", err)
			}
			checks := compareGameObservations([]GameScenario{scenario}, observations)
			if len(checks) != 1 || checks[0].Status != "unavailable" || checks[0].Repairable {
				t.Fatalf("checks = %+v", checks)
			}
			if !strings.Contains(checks[0].Observed, "target=item host_hidden (") || !strings.Contains(checks[0].Observed, "Studio preview was hidden or in the background") || !strings.Contains(checks[0].Observed, "keep the preview visible") {
				t.Fatalf("raw token or note missing: %s", checks[0].Observed)
			}
			// The result neither enters the repair loop nor fails as a validation error.
			result := BuildResult{GameplayStatus: "unavailable", Checks: checks}
			if repairableChecks(checks) || validationFailureCode(result) != "preview" {
				t.Fatalf("host-hidden result would be repaired or end as a validation failure: %+v", result)
			}
			if action := ValidationNextAction(result); !strings.Contains(action, "source edits cannot repair") {
				t.Fatalf("next action tells the model to repair: %s", action)
			}
		})
	}
	// A hidden run can never certify a pass, even when the metric moved.
	moved := []GameObservation{{ID: scenario.ID, Before: map[string]float64{"pickup_events": 0}, After: map[string]float64{"pickup_events": 1},
		TargetRuns: []TargetRun{{Target: "item", Mode: "reach", Samples: 30, Inputs: 12, Contacts: 3, Effects: 1, Reason: "host_hidden"}}}}
	if check := compareGameObservations([]GameScenario{scenario}, moved)[0]; check.Status != "unavailable" || check.Repairable {
		t.Fatalf("hidden run passed or became repairable: %+v", check)
	}
}
