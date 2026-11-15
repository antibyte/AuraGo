package gamemaker

import (
	"testing"
)

// driveJobWithReports plays the Studio parent: it answers every new validation
// with a ready report plus the observations chosen by the test.
func TestRepairableCheckClassification(t *testing.T) {
	scenario := GameScenario{ID: "required_rules", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Mode: "reach", Target: "item", MS: 4000}}}
	observation := func(run TargetRun, before, after float64) []GameObservation {
		return []GameObservation{{ID: scenario.ID, Before: map[string]float64{"pickup_events": before}, After: map[string]float64{"pickup_events": after}, TargetRuns: []TargetRun{run}}}
	}
	for name, test := range map[string]struct {
		observations []GameObservation
		status       string
		repairable   bool
	}{
		"missing target":  {observation(TargetRun{Target: "item", Mode: "reach", Reason: "no_target"}, 0, 0), "unavailable", true},
		"blocked route":   {observation(TargetRun{Target: "item", Mode: "reach", Samples: 40, Inputs: 40, Reason: "blocked"}, 0, 0), "unavailable", true},
		"inactive target": {observation(TargetRun{Target: "item", Mode: "reach", Samples: 4, Inputs: 0, Reason: "inactive"}, 0, 0), "unavailable", true},
		// inactive is the game's own pause/end state and stays repairable; a hidden Studio window is not.
		"host hidden":         {observation(TargetRun{Target: "item", Mode: "reach", Samples: 4, Inputs: 0, Reason: "host_hidden"}, 0, 0), "unavailable", false},
		"counter only":        {observation(TargetRun{Target: "item", Mode: "reach", Samples: 40, Inputs: 40, Contacts: 1, Effects: 0, Reason: "complete"}, 0, 1), "unavailable", true},
		"observed effect":     {observation(TargetRun{Target: "item", Mode: "reach", Samples: 40, Inputs: 40, Contacts: 1, Effects: 1, Reason: "complete"}, 0, 1), "passed", false},
		"no observation":      {nil, "unavailable", false},
		"mismatched evidence": {observation(TargetRun{Target: "enemy", Mode: "reach", Samples: 40, Inputs: 40, Contacts: 1, Effects: 1, Reason: "complete"}, 0, 1), "unavailable", false},
	} {
		t.Run(name, func(t *testing.T) {
			checks := compareGameObservations([]GameScenario{scenario}, test.observations)
			if len(checks) != 1 || checks[0].Status != test.status || checks[0].Repairable != test.repairable {
				t.Fatalf("checks = %+v", checks)
			}
		})
	}
}
