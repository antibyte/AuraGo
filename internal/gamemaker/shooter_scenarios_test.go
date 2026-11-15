package gamemaker

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func blindShooterScenario() GameScenario {
	return GameScenario{ID: "laser_hits", Metric: "hits", Compare: "increased", Steps: []GameTestStep{
		{Action: "wait", MS: 500},
		{Action: "key", Key: "SPACE", MS: 600}, {Action: "key", Key: "SPACE", MS: 600},
		{Action: "key", Key: "SPACE", MS: 600}, {Action: "key", Key: "SPACE", MS: 600},
		{Action: "key", Key: "SPACE", MS: 600}, {Action: "wait", MS: 500},
	}}
}

func TestShooterScenarioInputPreservesRequirementsAndBudget(t *testing.T) {
	for _, version := range []int{1, 3, 4} {
		plan := GamePlan{SchemaVersion: version, Template: "shooter", Scenarios: []GameScenario{blindShooterScenario()}}
		original, _ := json.Marshal(plan)
		checks := gameScenarios(&plan)
		got := checks[len(checks)-1]
		if err := validateScenario(got); err != nil {
			t.Fatal(err)
		}
		if got.ID != "laser_hits" || got.Metric != "hits" || got.Compare != "increased" || got.Value != 0 || got.inputNote == "" {
			t.Fatalf("requirements changed: %+v", got)
		}
		want := []GameTestStep{{Action: "wait", MS: 500}, {Action: "target", Target: "enemy", Mode: "aim", MS: 3500}}
		if !reflect.DeepEqual(got.Steps, want) {
			t.Fatalf("input budget or spawn wait changed: %+v", got.Steps)
		}
		if again := gameScenarios(&plan); !reflect.DeepEqual(checks, again) {
			t.Fatal("repeated builds changed the test fingerprint")
		}
		after, _ := json.Marshal(plan)
		if string(after) != string(original) {
			t.Fatal("accepted plan was mutated")
		}
		for _, id := range []string{"required_input", "required_primary", "required_assets", "required_restart", "required_end", "required_timed", "required_late_events"} {
			found := false
			for _, check := range checks {
				found = found || check.ID == id
			}
			if !found {
				t.Fatalf("schema %d lost %s", version, id)
			}
		}
		// Only physical input/effects may certify the retained hit requirement.
		obs := GameObservation{ID: got.ID, Before: map[string]float64{"hits": 0}, After: map[string]float64{"hits": 1}}
		if result := compareGameObservations([]GameScenario{got}, []GameObservation{obs})[0]; result.Status != "unavailable" || !strings.Contains(result.Observed, "input adapted:") {
			t.Fatalf("missing evidence passed or adaptation was hidden: %+v", result)
		}
		obs.TargetRuns = []TargetRun{{Target: "enemy", Mode: "aim", Samples: 20, Inputs: 4, Contacts: 1, Effects: 1, Reason: "complete"}}
		if result := compareGameObservations([]GameScenario{got}, []GameObservation{obs})[0]; result.Status != "passed" || !reflect.DeepEqual(result.Steps, want) {
			t.Fatalf("real hit or executed input lost: %+v", result)
		}
		obs.After["hits"] = 0
		if result := compareGameObservations([]GameScenario{got}, []GameObservation{obs})[0]; result.Status != "failed" {
			t.Fatalf("contact without a hit did not fail: %+v", result)
		}
		got.Steps[0].MS = 1
		if plan.Scenarios[0].Steps[0].MS != 500 {
			t.Fatal("execution steps alias the stored plan")
		}
	}
}

func TestShooterScenarioInputLeavesCustomContractsIntact(t *testing.T) {
	for _, name := range []string{"scene", "other_base", "primary", "hit_events", "threshold", "target", "pointer", "movement", "other_key", "passive", "short", "too_long", "too_many"} {
		t.Run(name, func(t *testing.T) {
			plan := GamePlan{Template: "shooter"}
			s := blindShooterScenario()
			switch name {
			case "scene":
				plan.Scene = &Scene{}
			case "other_base":
				plan.Template = "fps"
			case "primary":
				s.Metric = "actions"
			case "hit_events":
				s.Metric = "hit_events"
			case "threshold":
				s.Compare, s.Value = "at_least", 5
			case "target":
				s.Steps[1] = GameTestStep{Action: "target", Target: "boss", Mode: "aim", MS: 1000}
			case "pointer":
				s.Steps[1] = GameTestStep{Action: "pointer", X: 100, Y: 120, MS: 500}
			case "movement":
				s.Steps[1].Key = "LEFT"
			case "other_key":
				s.Steps[1].Key = "F"
			case "passive":
				s.Steps = []GameTestStep{{Action: "wait", MS: 4000}}
			case "short":
				s.Steps = []GameTestStep{{Action: "key", Key: "SPACE", MS: 50}}
			case "too_long":
				s.Steps[0].MS = 4000
			case "too_many":
				s.Steps = append(s.Steps, GameTestStep{Action: "observe"}, GameTestStep{Action: "observe"})
			}
			if got := shooterScenarioInput(&plan, s); !reflect.DeepEqual(got, s) {
				t.Fatalf("custom input was guessed: %+v", got)
			}
		})
	}
}

func TestShooterScenarioInputCapsTargetWindow(t *testing.T) {
	s := blindShooterScenario()
	s.Steps = []GameTestStep{{Action: "key", Key: "SPACE", MS: 3000}, {Action: "wait", MS: 3000}}
	got := shooterScenarioInput(&GamePlan{Template: "shooter"}, s)
	if err := validateScenario(got); err != nil {
		t.Fatal(err)
	}
	want := []GameTestStep{{Action: "target", Target: "enemy", Mode: "aim", MS: 4000}, {Action: "wait", MS: 2000}}
	if !reflect.DeepEqual(got.Steps, want) {
		t.Fatalf("per-command/overall budget changed: %+v", got.Steps)
	}
}
