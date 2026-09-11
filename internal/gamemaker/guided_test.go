package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestGuidedDesignResolvesCatalogAndCorrections(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "building" {
			if run.Plan.Template != "fps" || run.Plan.SchemaVersion != 2 || len(run.Plan.Assets) != 4 {
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
