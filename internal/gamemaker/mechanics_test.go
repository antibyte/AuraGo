package gamemaker

import "testing"

func TestMechanicsRejectMalformedAndDisconnectedBindings(t *testing.T) {
	for _, source := range []string{
		`{"blocks":[{"id":"move","kind":"imaginary"}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":"[]"}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":"{\"__proto__\":{}}"}]}`,
		`{"events":[{"event":"hit"},{"event":"hit"}]}`,
		`{"lives":2.5}`,
		`{} {}`,
		`{"blocks":[{"id":"move","kind":"movement","params":"{\"speed\":-1}"}]}`,
		`{"blocks":[{"id":"camera","kind":"camera","params":"{\"offset\":[0,\"x\",1]}"}]}`,
	} {
		if validateMechanicsSource(source) == nil {
			t.Fatalf("invalid mechanics accepted: %s", source)
		}
	}
	if err := validateMechanicsSource(`{"outcomes":["won"],"blocks":[{"id":"move","kind":"movement","params":"{\"speed\":180}"}]}`); err != nil {
		t.Fatal(err)
	}
	plan := GamePlan{Scene: &Scene{Nodes: []SceneNode{{ID: "hero"}}}, Mechanics: map[string]any{"blocks": []any{map[string]any{"id": "control", "kind": "movement", "target": "absent"}}}}
	if validateMechanicBindings(plan) == nil {
		t.Fatal("missing scene target accepted")
	}
	plan.Mechanics = map[string]any{"events": []any{map[string]any{"event": "collect", "effect": "pickup-glow"}}}
	if validateMechanicBindings(plan) == nil {
		t.Fatal("unimported effect accepted")
	}
	plan.Presentation = &Presentation{Effects: []string{"pickup-glow"}}
	if err := validateMechanicBindings(plan); err != nil {
		t.Fatal(err)
	}
}
