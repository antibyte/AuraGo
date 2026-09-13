package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestGuidedDesignResolvesCatalogAndCorrections(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "building" {
			if run.Plan.Template != "fps" || run.Plan.SchemaVersion != 3 || len(run.Plan.Assets) != 4 {
				t.Errorf("bad canonical plan: %+v", run.Plan)
			}
			for _, a := range run.Plan.Assets {
				if a.Version != "1.0.0" || a.Collider != "catalog" || a.Scale != 1 {
					t.Errorf("unresolved asset: %+v", a)
				}
			}
			source, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
			if err != nil || !strings.Contains(source, "startGame(") {
				t.Errorf("no guided entry: %s %v", source, err)
			}
			build := s.BuildJob(ctx, run.Job.ID)
			if !build.OK {
				t.Errorf("guided compile: %+v", build)
			}
			return errors.New("checked design")
		}
		bad := []byte(`{"base":"fps","objective":"Defend the forest","features":[],"settings":{"goal":5,"speed":5,"duration":120}}`)
		if err := s.SetDesignJSON(ctx, run.Job.ID, bad); err == nil {
			t.Error("empty features accepted")
		}
		// Provider-null optional fields do not erase a correction's retained fields.
		return s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":null,"features":["Shoot enemies and reload"]}`))
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "checked design" {
		t.Fatal(done.Error)
	}
}

func TestGuidedPerspectiveCorrectionPreservesPlatformer(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "building" {
			if run.Plan.Template != "platformer" || run.Plan.Perspective != "side" || run.Plan.Objective != "Collect coins and avoid enemies" || len(run.Plan.Assets) != 3 || run.Plan.Assets[2].Fallback != "Golden coin" {
				t.Errorf("correction changed intent or discarded a role: %+v", run.Plan)
			}
			return errors.New("perspective repaired")
		}
		design := GameDesign{Base: "platformer", Objective: "Collect coins and avoid enemies", Features: []string{"Jump between platforms"}, Assets: []DesignAsset{
			{Role: "player", PackID: "robots-drones-animated-top-down", AssetID: "service_robot_move_down"},
			{Role: "enemy", PackID: "robots-drones-animated-top-down", AssetID: "service_robot_move_down"},
			{Role: "item", Fallback: "Golden coin"},
		}}
		data, _ := json.Marshal(design)
		err := s.SetDesignJSON(ctx, run.Job.ID, data)
		if err == nil {
			t.Fatal("incompatible artwork accepted")
		}
		for _, want := range []string{"design.assets[0]", "design.assets[1]", `Keep base "platformer"`, `search_assets(view="side")`, "complete corrected assets array"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("missing actionable correction %q: %v", want, err)
			}
		}
		// Use the exact catalog alternatives returned in the failure, not invented IDs.
		parts := strings.Split(err.Error(), "compatible alternatives (choose one): ")
		if len(parts) != 3 {
			t.Fatalf("missing alternatives for both roles: %v", err)
		}
		for i, part := range parts[1:] {
			var alternatives []DesignAsset
			if e := json.NewDecoder(strings.NewReader(part)).Decode(&alternatives); e != nil || len(alternatives) == 0 {
				t.Fatalf("invalid suggestions: %v", e)
			}
			for _, a := range alternatives {
				detail, e := s.DescribeAsset(a.PackID, a.AssetID, a.AssemblyID)
				if e != nil || detail.View != "side" || a.Role != design.Assets[i].Role {
					t.Fatalf("incompatible suggestion: %+v %v", a, e)
				}
			}
			design.Assets[i] = alternatives[0]
		}
		if plan, _ := s.GetPlan(ctx, run.Job.ID); plan != nil {
			t.Error("rejected design installed a plan")
		}
		data, _ = json.Marshal(map[string]any{"assets": design.Assets})
		return s.SetDesignJSON(ctx, run.Job.ID, data)
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "perspective repaired" {
		t.Fatal(done.Error)
	}
}

func TestGuidedAnimatedCoinSupportsBothPerspectives(t *testing.T) {
	s := newTestService(t)
	for _, id := range []string{"coin_spin", "explosion", "smoke", "portal"} {
		detail, err := s.DescribeAsset("assets-animated", id, "")
		if err != nil || !detail.supportsPerspective("side") || !detail.supportsPerspective("top") {
			t.Fatalf("reviewed sprite lost a perspective: %s %v", id, err)
		}
	}
	matches, err := s.SearchAssets("coin", "", "side", 12)
	if err != nil || len(matches) == 0 {
		t.Fatalf("coin search: %+v %v", matches, err)
	}
	for _, match := range matches {
		if match.AssetID == "rotor_spin" {
			t.Fatal("coin search suggested a rotor")
		}
	}
	for _, base := range []string{"platformer", "topdown"} {
		t.Run(base, func(t *testing.T) {
			project := createTestProject(t, s, "2d")
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "building" {
					a := run.Plan.Assets[0]
					if run.Plan.Template != base || a.Role != "coin" || a.PackID != "assets-animated" || a.AssetID != "coin_spin" || a.Version != "3" {
						t.Fatalf("selected animated coin replaced: %+v", run.Plan)
					}
					detail, err := s.DescribeAsset(a.PackID, a.AssetID, "")
					if err != nil || len(detail.Animations) != 1 || detail.Animations[0].ID != "coin_spin" || detail.Animations[0].Entity != "coin" || detail.Animations[0].Action != "spin" {
						t.Fatalf("coin animation not retained: %+v %v", detail, err)
					}
					matches, err := s.SearchAssets("coin", "assets-animated", run.Plan.Perspective, 6)
					if err != nil || len(matches) != 1 || matches[0].AssetID != a.AssetID || len(matches[0].CompatibleViews) != 2 {
						t.Fatalf("compatible coin missing in search: %+v %v", matches, err)
					}
					// Full plans use the same catalog rule and retain their animation checks.
					p := *run.Plan
					p.Assets = append([]PlanAsset(nil), p.Assets...)
					p.Assets[0].Animations = []string{"coin_spin"}
					if err := s.checkPlan(project, p); err != nil {
						t.Fatalf("full plan rejected compatible animation: %v", err)
					}
					if base == "platformer" {
						p.Assets[0].AssetID = "chest_open"
						p.Assets[0].Animations = nil
						if err := s.checkPlan(project, p); err == nil || !strings.Contains(err.Error(), "perspective") {
							t.Fatalf("directional art lost its restriction: %v", err)
						}
					}
					return errors.New("coin accepted on first submission")
				}
				return s.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Collect coins","features":["Animated coins"],"assets":[{"role":"coin","pack_id":"assets-animated","asset_id":"coin_spin"}]}`, base)))
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "coin accepted on first submission" {
				t.Fatal(done.Error)
			}
		})
	}
}

func TestGuidedDesignRejectsUnknownFields(t *testing.T) {
	s := newTestService(t)
	for _, base := range []string{"fps", "exploration", "transport", "flight", "space"} {
		p, err := s.planFromDesign(context.Background(), "", Project{Dimension: "3d"}, GameDesign{Base: base, Objective: "Reach the goal", Features: []string{"A playable challenge"}})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.checkPlan(Project{Dimension: "3d"}, p); err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		sources, err := gameTemplateSources(p)
		if err != nil || len(sources["main.ts"]) == 0 {
			t.Fatalf("%s: %v", base, err)
		}
		if lines := strings.Count(string(sources["main.ts"]), "\n"); lines > 120 {
			t.Fatalf("%s entry requires repeated reads: %d lines", base, lines)
		}
	}
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("unexpected acceptance")
		}
		for i := 0; i < 3; i++ {
			if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","unknown":true}`)); err == nil {
				t.Error("unknown field accepted")
			}
		}
		if !s.PlanningComplete(run.Job.ID) {
			t.Error("design evaded correction limit")
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "failed" || !strings.Contains(done.Error, "unknown") {
		t.Fatal(done)
	}
}

func TestGuidedDesignNormalizesMechanicsPlacement(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, dimension)
			base := "platformer"
			if dimension == "3d" {
				base = "fps"
			}
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage != "planning" {
					plan, err := s.GetPlan(ctx, run.Job.ID)
					if err != nil {
						return err
					}
					if plan.SchemaVersion != 4 || plan.Mechanics["lives"] != float64(1) || len(plan.Mechanics["blocks"].([]any)) != 2 || len(plan.Mechanics["events"].([]any)) != 1 || len(plan.Mechanics["outcomes"].([]any)) != 2 {
						return fmt.Errorf("mechanic values lost: %+v", plan.Mechanics)
					}
					if plan.Objective != "Complete the unusual challenge" || plan.Scope[0] != "Custom player mechanic" {
						return errors.New("creative design was replaced")
					}
					return errors.New("verified mechanics placement")
				}
				// Real failure shape: blocks are nested; the other helper fields are flat.
				data := []byte(fmt.Sprintf(`{"base":%q,"objective":"Complete the unusual challenge","features":["Custom player mechanic"],"outcomes":["won","lost"],"lives":1,"events":[{"event":"hit"}],"mechanics":{"blocks":[{"id":"move","kind":"movement"},{"id":"look","kind":"camera"}]}}`, base))
				// Exercise the encoded provider transport at the shared public boundary.
				if dimension == "3d" {
					data, _ = json.Marshal(string(data))
				}
				return s.SetDesignJSON(ctx, run.Job.ID, data)
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "verified mechanics placement" {
				t.Fatal(done)
			}
		})
	}
}

func TestGuidedMechanicsPlacementPreservesValidationAndCorrections(t *testing.T) {
	for _, tc := range []struct{ name, fields, wantError string }{
		{"flat blocks", `"blocks":[{"id":"move","kind":"movement"}]`, ""},
		{"equal duplicates", `"outcomes":["won"],"mechanics":{"outcomes":[ "won" ]}`, ""},
		{"null optional", `"outcomes":null,"lives":null,"mechanics":{"outcomes":["won"]}`, ""},
		{"conflicting duplicates", `"outcomes":["won"],"mechanics":{"outcomes":["lost"]}`, "design.mechanics.outcomes: conflicts"},
		{"invalid outcome", `"outcomes":["victory"]`, "plan.mechanics.outcomes"},
		{"invalid lives", `"lives":100`, "plan.mechanics.lives"},
		{"invalid block", `"blocks":[{"id":"move","kind":"invented"}]`, "plan.mechanics.blocks"},
		{"invalid event", `"events":[{"event":"hit","invented":true}]`, "plan.mechanics.events"},
		{"invalid mechanics", `"lives":3,"mechanics":[]`, "design.mechanics"},
		{"unknown root", `"outcomes":["won"],"invented":true`, `unknown field "invented"`},
		{"unknown nested", `"outcomes":["won"],"mechanics":{"invented":true}`, "plan.mechanics.invented"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t)
			project := Project{Dimension: "2d"}
			data := []byte(`{"base":"minimal","objective":"Explore","features":["Custom rules"],` + tc.fields + `}`)
			encoded, err := s.expandDesign(context.Background(), "draft", project, data)
			if err == nil {
				var plan GamePlan
				if err = json.Unmarshal(encoded, &plan); err == nil {
					err = s.checkPlan(project, plan)
				}
			}
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("expected %q; got %v", tc.wantError, err)
			}
		})
	}
	s := newTestService(t)
	project := Project{Dimension: "2d"}
	// A correction to a misplaced field must keep existing sibling blocks.
	_, err := s.expandDesign(context.Background(), "draft", project, []byte(`{"base":"minimal","objective":"Explore","features":["Custom rules"],"mechanics":{"outcomes":["invalid"],"blocks":[{"id":"move","kind":"movement"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := s.expandDesign(context.Background(), "draft", project, []byte(`{"outcomes":["won"]}`))
	if err != nil {
		t.Fatal(err)
	}
	var plan GamePlan
	if err = json.Unmarshal(encoded, &plan); err != nil {
		t.Fatal(err)
	}
	if err = s.checkPlan(project, plan); err != nil || len(plan.Mechanics["blocks"].([]any)) != 1 || plan.Objective != "Explore" {
		t.Fatalf("correction lost retained fields: %+v, %v", plan, err)
	}
}

func TestSourceEditsReturnCompilerFeedbackAndPreserveStaleFiles(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		if run.Stage != "building" {
			return errors.New("unexpected repair")
		}
		bad, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", "const broken = ;", "")
		if err != nil || !bad.Written || bad.Build.OK || len(bad.Build.Diagnostics) == 0 {
			t.Errorf("compiler feedback: %+v %v", bad, err)
		}
		read, err := s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 0, 0)
		if err != nil || read.SHA256 != bad.SHA256 {
			t.Fatalf("read: %+v %v", read, err)
		}
		if _, err = s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "const broken = ;", "const fixed = 1;", "wrong"); err == nil {
			t.Error("stale edit accepted")
		}
		if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); !strings.Contains(got, "const broken = ;") {
			t.Error("conflict mutated file")
		}
		good, err := s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "const broken = ;", "const fixed = 1;", read.SHA256)
		if err != nil || !good.Build.OK || good.SHA256 == read.SHA256 {
			t.Errorf("replace: %+v %v", good, err)
		}
		if _, err = s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "", "anything", good.SHA256); err == nil {
			t.Error("empty target accepted")
		}
		for _, path := range []string{"../escape.ts", "vendor/runtime.js", ".aurago/game-plan.json", "dist/game.js"} {
			if _, err = s.WriteJobFileChecked(ctx, run.Job.ID, path, "", ""); err == nil {
				t.Errorf("path writable: %s", path)
			}
		}
		if _, err = s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 1, 300); err == nil {
			t.Error("unbounded range accepted")
		}
		full, err := s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 1, 240)
		if err != nil || full.EndLine != full.TotalLines || full.SHA256 != good.SHA256 || !strings.Contains(full.Content, "const fixed = 1;") {
			t.Errorf("bounded range must stop at EOF and retain the full digest: %+v %v", full, err)
		}
		if _, err = s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", full.TotalLines+1, full.TotalLines+2); err == nil {
			t.Error("range starting past EOF accepted")
		}
		return errors.New("checked edits")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "checked edits" {
		t.Fatal(done.Error)
	}
}

func TestGuidanceIsBoundedAndPhaseSpecific(t *testing.T) {
	for _, stage := range []string{"planning", "building", "repair"} {
		for _, dimension := range []string{"2d", "3d"} {
			text := PhaseGuidance(stage, dimension)
			if len(text) > 2500 || len(text) < 200 {
				t.Fatalf("%s %s has %d chars", stage, dimension, len(text))
			}
			if stage == "planning" && !strings.Contains(text, "set_design") {
				t.Error("no compact plan path")
			}
		}
	}
	// Required scenarios cannot be replaced by an empty user list.
	for _, base := range []string{"fps", "exploration", "transport", "flight", "space"} {
		data, _ := json.Marshal(requiredScenarios(base))
		if !strings.Contains(string(data), "required_restart") {
			t.Fatal(base)
		}
	}
}

func TestGuidedDesignRejectsDuplicateRolesAndIncompatibleFPS(t *testing.T) {
	s := newTestService(t)
	for _, assets := range [][]DesignAsset{
		{{Role: "enemy", PackID: ModelPackID, AssetID: "animals-wolf"}, {Role: "enemy", PackID: ModelPackID, AssetID: "animals-bear"}},
		{{Role: "arms", PackID: ModelPackID, AssetID: "road-pickup"}},
		{{Role: "weapon", PackID: ModelPackID, AssetID: "props-crystal"}},
	} {
		p, err := s.planFromDesign(context.Background(), "", Project{Dimension: "3d"}, GameDesign{Base: "fps", Objective: "Defend the forest", Features: []string{"Aim and shoot"}, Assets: assets})
		if err == nil {
			err = s.checkPlan(Project{Dimension: "3d"}, p)
		}
		if err == nil {
			t.Fatalf("accepted incompatible roles: %+v", assets)
		}
	}
}

func TestGuidedEditPreservesExistingAssetsAndCode(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(testRunner{service: s})
	first, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, first.ID); finished.Status != "ready" {
		t.Fatal(finished)
	}
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			old, err := s.GetPlan(ctx, run.Job.ID)
			if err != nil {
				return err
			}
			d := GameDesign{Base: old.Template, Objective: old.Objective, Features: []string{"Keep scoring and improve the HUD"}}
			data, _ := json.Marshal(d)
			if err = s.SetDesignJSON(ctx, run.Job.ID, data); err != nil {
				return err
			}
			updated, _ := s.GetPlan(ctx, run.Job.ID)
			if len(updated.Preserve) == 0 || len(updated.Assets) != len(old.Assets) {
				t.Error("edit lost prior plan")
			}
			return nil
		}
		source, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if !strings.Contains(source, "this.state.score += 10") {
			t.Error("edit reinstalled the starter")
		}
		return errors.New("checked preserved edit")
	}))
	edit, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Improve the HUD"})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, edit.ID); finished.Error != "checked preserved edit" {
		t.Fatal(finished)
	}
}

func TestDesignCarriesOptionalSceneAndMechanics(t *testing.T) {
	service := newTestService(t)
	project := Project{ID: "project", Dimension: "2d"}
	scene := &Scene{
		SchemaVersion: 1, Dimension: "2d", Seed: 11,
		Levels:      []SceneLevel{{ID: "main", Active: true}},
		WorldBounds: SceneBounds{Min: Vec3{0, 0, 0}, Max: Vec3{960, 540, 0}},
	}
	design := GameDesign{
		Base: "minimal", Objective: "Explore freely",
		Features: []string{"Movement"},
		Scene:    scene, Mechanics: map[string]any{"outcomes": []any{"won"}},
		Scenarios: []GameScenario{{ID: "reach_goal", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Target: "goal", Mode: "reach", MS: 4000}}}},
	}
	plan, err := service.planFromDesign(context.Background(), "", project, design)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != 4 || plan.Scene != scene || plan.Mechanics["outcomes"] == nil {
		t.Fatalf("optional design fields were not propagated: %+v", plan)
	}
	if plan.Rules["failure"] == "Health or time exhausted" || plan.Rules["completion"] == "Reach the objective; show result and allow restart" {
		t.Fatalf("minimal design received forced outcomes: %+v", plan.Rules)
	}
	if len(plan.Scenarios) != 1 || plan.Scenarios[0].Steps[0].Target != "goal" {
		t.Fatalf("targeted scenarios not propagated: %+v", plan.Scenarios)
	}
	design.Scenarios[0].Steps[0].Mode = "teleport"
	invalid, err := service.planFromDesign(context.Background(), "", project, design)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.checkPlan(project, invalid); err == nil {
		t.Fatal("unsupported test control accepted")
	}
}
