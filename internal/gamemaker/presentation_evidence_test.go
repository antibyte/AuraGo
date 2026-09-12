package gamemaker

import "testing"

func TestGameEvidenceRequiresSelectedPickupEffect(t *testing.T) {
	plan := &GamePlan{Presentation: &Presentation{Effects: []string{"pickup-glow"}}}
	e := &GameplayEvidence{Events: map[string]int{"pickup": 1}, Presentation: &PresentationEvidence{Effects: map[string]int{}}}
	status, _ := compareGameplayEvidence(plan, []GameObservation{{ID: "collect", EvidenceAfter: e}})
	if status != "failed" {
		t.Fatal("collection without mandatory effect was accepted")
	}
	e.Presentation.Effects["pickup-glow"] = 1
	status, _ = compareGameplayEvidence(plan, []GameObservation{{ID: "collect", EvidenceAfter: e}})
	if status == "failed" {
		t.Fatal("delivered effect still fails")
	}
}

func TestGameEvidenceDoesNotInventLivesForPeacefulGame(t *testing.T) {
	plan := &GamePlan{Template: "three", Mechanics: map[string]any{"outcomes": []string{"won"}}}
	status, _ := compareGameplayEvidence(plan, []GameObservation{{ID: "delivery", After: map[string]float64{"lives": 0, "goal_remaining": 0}, EvidenceAfter: &GameplayEvidence{Outcome: "won", Events: map[string]int{"win": 1}}}})
	if status != "passed" {
		t.Fatal("a peaceful game was assigned an undeclared loss condition: " + status)
	}
}

func TestGameEvidenceBoundsPresentationCounts(t *testing.T) {
	for _, p := range []PresentationEvidence{{Events: map[string]int{"hit": -1}}, {Effects: map[string]int{"": 1}}, {Sounds: map[string]int{"rifle": 1000001}}} {
		if validateGameplayEvidence(&GameplayEvidence{Presentation: &p}) == nil {
			t.Fatal("invalid controller evidence accepted")
		}
	}
}
