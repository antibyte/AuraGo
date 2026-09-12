package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestPlanBindingNormalizationPersistsBothInputs(t *testing.T) {
	for _, compact := range []bool{true, false} {
		t.Run(fmt.Sprint("compact=", compact), func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			accepted := false
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage != "planning" {
					return errors.New("test finished")
				}
				design := GameDesign{Base: "minimal", Objective: "Reach the landmark", Features: []string{"Walk freely"},
					Scene: &Scene{SchemaVersion: 1, Dimension: "2d", Levels: []SceneLevel{{ID: "main", Active: true}},
						WorldBounds: SceneBounds{Min: Vec3{}, Max: Vec3{960, 540, 0}},
						Nodes:       []SceneNode{{ID: "hero", Kind: "player", Position: Vec3{50, 50, 0}, Size: Vec3{20, 20, 0}}}},
					Presentation: &Presentation{Sounds: []SoundBinding{{Event: "win", Sound: "win"}, {Event: "lose", Sound: "lose"}}},
					Mechanics:    map[string]any{"blocks": []mechanicBlock{{ID: "walk", Kind: "movement"}}, "events": []mechanicEvent{{Event: "win", Sound: "win"}, {Event: "lose", Sound: "lose"}}}}
				data, _ := json.Marshal(design)
				var err error
				if compact {
					err = s.SetDesignJSON(ctx, run.Job.ID, data)
				} else {
					plan, e := s.planFromDesign(ctx, run.Job.ID, project, design)
					if e != nil {
						return e
					}
					err = s.SetPlan(ctx, run.Job.ID, plan)
				}
				if err != nil {
					return err
				}
				plan, err := s.GetPlan(ctx, run.Job.ID)
				if err != nil {
					return err
				}
				if plan.Presentation.Sounds[0].Sound != "victory" || plan.Presentation.Sounds[1].Sound != "defeat" {
					return fmt.Errorf("noncanonical sounds: %+v", plan.Presentation.Sounds)
				}
				raw, _ := json.Marshal(plan.Mechanics)
				if !strings.Contains(string(raw), `"target":"hero"`) || !strings.Contains(string(raw), `"sound":"victory"`) || !strings.Contains(string(raw), `"sound":"defeat"`) {
					return fmt.Errorf("noncanonical mechanics: %s", raw)
				}
				stage, _ := s.JobDirectory(run.Job.ID)
				if err := checkMechanicsSource(stage, string(raw)); err != nil {
					return err
				}
				config, err := presentationConfig(plan.Presentation)
				if err != nil || !strings.Contains(config, `"id":"victory"`) {
					return fmt.Errorf("runtime config: %s, %v", config, err)
				}
				accepted = true
				return nil
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			finished := waitJob(t, s, job.ID)
			if !accepted || finished.Error != "test finished" {
				t.Fatalf("accepted=%v; error=%s", accepted, finished.Error)
			}
		})
	}
}

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
