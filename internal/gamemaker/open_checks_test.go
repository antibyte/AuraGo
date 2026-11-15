package gamemaker

import (
	"testing"
)

func TestOpenChecksRoundTripThroughTheOutcomeColumn(t *testing.T) {
	checks := []OpenCheck{
		{ID: "hits", Status: "failed", Expected: "hits increased", Observed: "hits stayed 0", NextAction: "Fix the first failed check."},
		{ID: "goal", Status: "unavailable", Repairable: true, Steps: []GameTestStep{{Action: "move", Key: "RIGHT", MS: 500}}},
	}
	encoded := encodeOpenChecks(checks)
	decoded := decodeOpenChecks(encoded)
	if len(decoded) != len(checks) || decoded[0].ID != "hits" || decoded[1].Steps[0].Key != "RIGHT" {
		t.Fatalf("round trip = %+v", decoded)
	}
	if encodeOpenChecks(nil) != "" || decodeOpenChecks("") != nil {
		t.Fatal("empty list must not serialize payload")
	}
	if decodeOpenChecks("{not json") != nil {
		t.Fatal("broken column must degrade to no open checks")
	}
	if decodeOpenChecks(`{"id":"object"}`) != nil {
		t.Fatal("non-list column must degrade to no open checks")
	}
}
