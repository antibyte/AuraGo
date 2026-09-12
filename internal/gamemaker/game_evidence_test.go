package gamemaker

import (
	"math"
	"strings"
	"testing"
)

func TestGameEvidenceDoesNotCertifyCountersOrForcedEnd(t *testing.T) {
	plan := &GamePlan{Template: "blocks"}
	for _, observation := range []GameObservation{
		{ID: "required_rules", After: map[string]float64{"score": 400, "hits": 40}},
		{ID: "required_end", EvidenceAfter: &GameplayEvidence{Outcome: "lost", Events: map[string]int{"lose": 1}}},
	} {
		status, _ := compareGameplayEvidence(plan, []GameObservation{observation})
		if status != "unverified" {
			t.Fatalf("counter/ESC self-certified gameplay: %s", status)
		}
	}
}

func TestGameEvidenceRejectsKnownModelRegressions(t *testing.T) {
	plan := &GamePlan{Template: "blocks", Assets: []PlanAsset{{Role: "ball", AssetID: "ball_01"}}}
	for _, test := range []struct {
		name     string
		evidence GameplayEvidence
		after    map[string]float64
	}{
		{"wrong ball", GameplayEvidence{Roles: []ObservedAssetRole{{Role: "ball", AssetID: "weapon_01", Count: 1}}}, nil},
		{"late defeat marked victory", GameplayEvidence{Outcome: "won"}, map[string]float64{"lives": 0}},
		{"unreachable remaining targets", GameplayEvidence{Outcome: "won"}, map[string]float64{"goal_remaining": 18}},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _ := compareGameplayEvidence(plan, []GameObservation{{ID: "play", After: test.after, EvidenceAfter: &test.evidence}})
			if status != "failed" {
				t.Fatalf("known regression accepted: %s", status)
			}
		})
	}
}

func TestGameEvidenceMissingRoleAndUnusedPresentationStayExplicit(t *testing.T) {
	plan := &GamePlan{Template: "blocks", Assets: []PlanAsset{{Role: "ball", AssetID: "ball_01"}}}
	status, checks := compareGameplayEvidence(plan, []GameObservation{{ID: "play", EvidenceAfter: &GameplayEvidence{Outcome: "playing"}}})
	if status != "unverified" {
		t.Fatal(status)
	}
	for _, check := range checks {
		if check.ID == "role_ball" && check.Status == "unverified" {
			return
		}
	}
	t.Fatal("missing role was silently certified")
}

func TestGameEvidenceMatchesTargetsAndRejectsDuplicateFeedback(t *testing.T) {
	plan := &GamePlan{Mechanics: map[string]any{"outcomes": []string{"won"}}}
	observation := GameObservation{ID: "combat", EvidenceBefore: &GameplayEvidence{Events: map[string]int{}, Nodes: []ObservedSceneNode{{ID: "target", Active: true}}, Presentation: &PresentationEvidence{Events: map[string]int{}}}, EvidenceAfter: &GameplayEvidence{Outcome: "won", Events: map[string]int{"hit": 1, "win": 1}, Nodes: []ObservedSceneNode{{ID: "target", Active: false}}, Trace: []ObservedGameEvent{{Type: "hit", ID: "target", At: 1}}, Presentation: &PresentationEvidence{Events: map[string]int{"hit": 1}}}}
	if status, _ := compareGameplayEvidence(plan, []GameObservation{observation}); status != "passed" {
		t.Fatal(status)
	}
	observation.EvidenceAfter.Trace[0].ID = "unrelated"
	if status, _ := compareGameplayEvidence(plan, []GameObservation{observation}); status != "unverified" {
		t.Fatal("unrelated object change certified hit", status)
	}
	observation.EvidenceAfter.Presentation.Events["hit"] = 2
	if status, _ := compareGameplayEvidence(plan, []GameObservation{observation}); status != "failed" {
		t.Fatal("duplicate feedback accepted", status)
	}
}

func TestGameEvidenceBounds(t *testing.T) {
	bad := math.NaN()
	for _, evidence := range []GameplayEvidence{
		{Outcome: "passed"}, {Roles: make([]ObservedAssetRole, 65)}, {Nodes: make([]ObservedSceneNode, 129)},
		{Events: map[string]int{"hit": -1}}, {Faults: []string{strings.Repeat("x", 161)}},
		{Nodes: []ObservedSceneNode{{ID: "a", X: bad}}},
		{Nodes: []ObservedSceneNode{{ID: "a"}, {ID: "a"}}},
	} {
		if validateGameplayEvidence(&evidence) == nil {
			t.Fatalf("invalid evidence accepted: %+v", evidence)
		}
	}
	if err := validateGameplayEvidence(&GameplayEvidence{Outcome: "playing", Nodes: []ObservedSceneNode{{ID: "ball", Active: true, X: 3, Y: 4}}, Events: map[string]int{"hit": 1}}); err != nil {
		t.Fatal(err)
	}
}
