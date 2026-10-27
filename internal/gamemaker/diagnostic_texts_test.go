package gamemaker

import (
	"strings"
	"testing"
)

func TestPreviewAssetHandleErrorGetsOneHint(t *testing.T) {
	input := []Diagnostic{{Message: "Uncaught TypeError: Cannot read properties of undefined (reading 'released')"}}
	first := boundedPreviewDiagnostics(input)
	second := boundedPreviewDiagnostics(first)
	if len(second) != 1 || second[0] != first[0] || strings.Count(second[0].Message, "Asset handle:") != 1 {
		t.Fatalf("hint missing or repeated: %+v", second)
	}
	for _, want := range []string{"createInstance/createInstances", "await loadAsset(", "runtime.api_reference", "reading 'released'"} {
		if !strings.Contains(second[0].Message, want) {
			t.Errorf("hint lacks %q: %s", want, second[0].Message)
		}
	}
	// The hint must not leak onto unrelated errors.
	other := boundedPreviewDiagnostics([]Diagnostic{{Message: "Uncaught TypeError: Cannot read properties of undefined (reading 'x')"}})
	if strings.Contains(other[0].Message, "Asset handle:") {
		t.Fatalf("hint attached to an unrelated error: %s", other[0].Message)
	}
}

func TestComparisonsWithoutAThresholdPrintNoValue(t *testing.T) {
	for _, compare := range []string{"increased", "decreased", "changed"} {
		check := compareGameObservations([]GameScenario{{ID: "c", Metric: "pickup_events", Compare: compare}}, nil)[0]
		if check.Expected != "pickup_events "+compare {
			t.Errorf("%s expectation = %q", compare, check.Expected)
		}
	}
	for compare, want := range map[string]string{
		"at_least":      "stage_count at_least 3",
		"equals":        "stage_count equals 3",
		"not_decreased": "stage_count not_decreased 3",
	} {
		check := compareGameObservations([]GameScenario{{ID: "c", Metric: "stage_count", Compare: compare, Value: 3}}, nil)[0]
		if check.Expected != want {
			t.Errorf("%s expectation = %q, want %q", compare, check.Expected, want)
		}
	}
}

func TestTargetReasonsKeepTheRawTokenAndAddPlainLanguage(t *testing.T) {
	scenario := GameScenario{ID: "required_rules", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "item", Mode: "reach"}}}
	for reason, note := range map[string]string{
		"no_target":   "no object with this role was visible",
		"timeout":     "ran out of time",
		"blocked":     "no free path",
		"unsupported": "does not expose this target",
		"inactive":    "not active during the test",
		"host_hidden": "Studio preview was hidden or in the background",
	} {
		observation := GameObservation{ID: scenario.ID, Before: map[string]float64{"pickup_events": 0}, After: map[string]float64{"pickup_events": 0},
			TargetRuns: []TargetRun{{Target: "item", Mode: "reach", Samples: 3, Inputs: 1, Reason: reason}}}
		check := compareGameObservations([]GameScenario{scenario}, []GameObservation{observation})[0]
		raw := "target=item " + reason + " (inputs=1, contacts=0, effects=0)"
		if !strings.Contains(check.Observed, raw+": ") || !strings.Contains(check.Observed, note) {
			t.Errorf("%s: raw token or plain-language note missing: %s", reason, check.Observed)
		}
	}
	observation := GameObservation{ID: scenario.ID, Before: map[string]float64{"pickup_events": 0}, After: map[string]float64{"pickup_events": 1},
		TargetRuns: []TargetRun{{Target: "item", Mode: "reach", Samples: 3, Inputs: 1, Contacts: 1, Effects: 1, Reason: "complete"}}}
	if check := compareGameObservations([]GameScenario{scenario}, []GameObservation{observation})[0]; !strings.HasSuffix(check.Observed, "effects=1)") {
		t.Errorf("a complete run gained a note: %s", check.Observed)
	}
	// A failed rule without any target step is blind input.
	blind := GameScenario{ID: "required_rules", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "observe"}}}
	observation = GameObservation{ID: blind.ID, Before: map[string]float64{"pickup_events": 0}, After: map[string]float64{"pickup_events": 0}}
	check := compareGameObservations([]GameScenario{blind}, []GameObservation{observation})[0]
	if !strings.Contains(check.Observed, "blind input did not establish the required gameplay opportunity") || !strings.Contains(check.Observed, "blind input means the test only pressed generic controls") {
		t.Errorf("blind input is not explained: %s", check.Observed)
	}
}
