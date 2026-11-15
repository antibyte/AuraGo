package gamemaker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMechanicBindingDefaultsAndRuntimePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, block string
		players     int
		wantTarget  string
		wantErr     bool
	}{
		{"sole player", `{"id":"move","kind":"movement"}`, 1, "hero", false},
		{"ambiguous players", `{"id":"move","kind":"movement"}`, 2, "", true},
		{"no player", `{"id":"move","kind":"movement"}`, 0, "", true},
		{"explicit target", `{"id":"move","kind":"movement","target":"other"}`, 1, "other", false},
		{"null parameters", `{"id":"move","kind":"movement","params":null}`, 1, "hero", false},
		{"object parameters", `{"id":"move","kind":"movement","params":{"target":"other","speed":180}}`, 1, "", false},
		{"parameter target", `{"id":"move","kind":"movement","params":"{\"target\":\"other\"}"}`, 1, "", false},
		{"top level overrides parameters", `{"id":"move","kind":"movement","target":"hero","params":"{\"target\":\"absent\"}"}`, 1, "hero", false},
		{"target overrides role", `{"id":"move","kind":"movement","target":"hero","role":"unplaced"}`, 1, "hero", false},
		{"bad parameter target", `{"id":"move","kind":"movement","params":"{\"target\":\"absent\"}"}`, 1, "", true},
		{"parameter role", `{"id":"move","kind":"movement","params":"{\"role\":\"hero-art\"}"}`, 1, "", false},
		{"bad parameter type", `{"id":"move","kind":"movement","params":"{\"target\":12}"}`, 1, "", true},
		{"unknown property", `{"id":"move","kind":"movement","typo":true}`, 1, "", true},
		{"other mechanics need binding", `{"id":"hit","kind":"damage"}`, 1, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := GamePlan{Scene: &Scene{Nodes: []SceneNode{{ID: "other"}}}}
			if tc.players > 0 {
				plan.Scene.Nodes = append(plan.Scene.Nodes, SceneNode{ID: "hero", Kind: "player"})
			}
			if tc.players > 1 {
				plan.Scene.Nodes = append(plan.Scene.Nodes, SceneNode{ID: "second", Properties: map[string]any{"player": true}})
			}
			plan.Scene.Placements = []ScenePlacement{{NodeID: "hero", AssetRole: "hero-art"}}
			if err := json.Unmarshal([]byte(`{"blocks":[`+tc.block+`]}`), &plan.Mechanics); err != nil {
				t.Fatal(err)
			}
			err := normalizePlanBindings(&plan)
			if err == nil {
				err = validateMechanicBindings(plan)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			data, _ := json.Marshal(plan.Mechanics["blocks"])
			var blocks []mechanicBlock
			_ = json.Unmarshal(data, &blocks)
			if blocks[0].Target != tc.wantTarget {
				t.Fatalf("target=%q, want %q", blocks[0].Target, tc.wantTarget)
			}
		})
	}
}

func TestMechanicsRejectMalformedAndDisconnectedBindings(t *testing.T) {
	for _, source := range []string{
		`{"blocks":[{"id":"move","kind":"imaginary"}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":[]}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":"null"}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":{"speed":-1}}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":{"target":12}}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":{"nested":{"constructor":{}}}}]}`,
		`{"blocks":[{"id":"move","kind":"movement","params":{"custom":"` + strings.Repeat("x", 4000) + `"}}]}`,
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
