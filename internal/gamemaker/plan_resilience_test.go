package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// designBudgetFailing submits one compact design and returns its structured
// correction error.
func designBudgetFailing(t *testing.T, s *Service, jobID, body string) *DesignValidationError {
	t.Helper()
	err := s.SetDesignJSON(context.Background(), jobID, []byte(body))
	var detail *DesignValidationError
	if !errors.As(err, &detail) {
		t.Fatalf("expected a structured design correction, got %v", err)
	}
	return detail
}

func TestPlanningNextActionOmitsSetPlan(t *testing.T) {
	action := JobNextAction(Job{Phase: "planning"})
	if strings.Contains(action, "set_plan") {
		t.Fatalf("planning next_action still advertises set_plan: %s", action)
	}
	if !strings.Contains(action, "dry_run") {
		t.Fatalf("planning next_action does not mention the dry run: %s", action)
	}
}

func TestDesignDryRunConsumesNoCorrection(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		invalid := `{"base":"minimal","objective":"","features":[]}`
		for i := 0; i < planPreviewLimit; i++ {
			err := s.PreviewDesignJSON(ctx, run.Job.ID, []byte(invalid))
			var detail *DesignValidationError
			if !errors.As(err, &detail) {
				t.Fatalf("preview %d did not return a structured correction: %v", i, err)
			}
			if detail.RemainingAttempts != planChargedLimit {
				t.Fatalf("preview %d consumed a correction, remaining=%d", i, detail.RemainingAttempts)
			}
			if s.PlanningComplete(run.Job.ID) {
				t.Fatal("preview completed planning")
			}
		}
		if err := s.PreviewDesignJSON(ctx, run.Job.ID, []byte(invalid)); err == nil || !strings.Contains(err.Error(), "preview limit") {
			t.Fatalf("fourth preview admitted: %v", err)
		}
		// A real submission still has its full charged budget.
		detail := designBudgetFailing(t, s, run.Job.ID, `{"base":"minimal","objective":"","features":[]}`)
		if detail.RemainingAttempts != planChargedLimit-1 {
			t.Fatalf("first charged correction left remaining=%d", detail.RemainingAttempts)
		}
		if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Explore the grove","features":["Collect glowing seeds"]}`)); err != nil {
			return err
		}
		return errors.New("dry run kept the budget")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "dry run kept the budget" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestDesignDryRunReturnsCanonicalPlan(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if err := s.PreviewDesignJSON(ctx, run.Job.ID, []byte(`{"base":"topdown","objective":"Explore","features":["Collect seeds"]}`)); err != nil {
			return err
		}
		raw := s.DesignPreview(run.Job.ID)
		if len(raw) == 0 {
			return errors.New("preview plan missing")
		}
		var plan GamePlan
		if err := json.Unmarshal(raw, &plan); err != nil {
			return err
		}
		if plan.Template != "topdown" {
			return errors.New("preview plan lost the requested base")
		}
		if stored, err := s.GetPlan(ctx, run.Job.ID); err != nil || stored != nil {
			return errors.New("preview persisted a plan")
		}
		return errors.New("preview returned a plan")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "preview returned a plan" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestDesignProgressBudgetDoesNotChargeSmallerIssueSets(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		first := designBudgetFailing(t, s, run.Job.ID, `{"base":"minimal","objective":"","features":[],"assets":[{"role":""}],"settings":{"goal":2}}`)
		if len(first.Issues) < 4 || first.RemainingAttempts != 2 {
			t.Fatalf("first correction = %d issues, remaining %d", len(first.Issues), first.RemainingAttempts)
		}
		// Fewer independent issues: the correction is free.
		second := designBudgetFailing(t, s, run.Job.ID, `{"objective":"Explore","features":["Collect seeds"],"assets":[{"role":""}],"settings":{"goal":2}}`)
		if second.RemainingAttempts != 2 {
			t.Fatalf("smaller issue set consumed a correction, remaining=%d", second.RemainingAttempts)
		}
		if s.PlanningComplete(run.Job.ID) {
			t.Fatal("planning completed before the charged budget was spent")
		}
		// No further progress: the next failure is charged again.
		third := designBudgetFailing(t, s, run.Job.ID, `{"objective":"Explore","features":["Collect seeds"],"assets":[{"role":""}],"settings":{"goal":2}}`)
		if third.RemainingAttempts != 1 {
			t.Fatalf("unchanged issue set was not charged, remaining=%d", third.RemainingAttempts)
		}
		return errors.New("progress budget verified")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "progress budget verified" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestDesignBudgetKeepsAHardCallCeiling(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		bodies := []string{
			`{"base":"minimal","objective":"","features":[],"assets":[{"role":""}],"settings":{"goal":2}}`,
			`{"objective":"Explore","features":[],"assets":[{"role":""}],"settings":{"goal":2}}`,
			`{"objective":"Explore","features":["Collect seeds"],"assets":[{"role":""}],"settings":{"goal":2}}`,
			`{"objective":"Explore","features":["Collect seeds"],"settings":{"goal":2}}`,
			`{"objective":"Explore","features":["Collect seeds"],"settings":{"goal":2}}`,
		}
		for i, body := range bodies {
			if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(body)); err == nil {
				t.Fatalf("submission %d unexpectedly accepted", i)
			}
		}
		if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Explore","features":["Collect seeds"]}`)); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("hard call ceiling not enforced: %v", err)
		}
		return errors.New("call ceiling verified")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "call ceiling verified" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestDesignAutoRepairsPluralAssetIDs(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if err := s.PreviewDesignJSON(ctx, run.Job.ID, []byte(`{"base":"topdown","objective":"Explore","features":["Collect seeds"],"assets":[{"role":"item","pack_ids":["robots-drones-animated-top-down"],"asset_ids":["service_robot_move_down","service_robot_move_down"]}]}`)); err != nil {
			return err
		}
		plan := previewPlan(t, s, run.Job.ID)
		if len(plan.Assets) != 2 || plan.Assets[0].Role != "item" || plan.Assets[1].Role != "item_2" {
			t.Fatalf("plural asset ids were not split: %+v", plan.Assets)
		}
		if plan.Assets[0].PackID != "robots-drones-animated-top-down" || plan.Assets[1].PackID != "robots-drones-animated-top-down" {
			t.Fatalf("pack ids lost: %+v", plan.Assets)
		}
		warnings := s.TakeDesignWarnings(run.Job.ID)
		if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, " "), "assets") {
			t.Fatalf("normalization was not reported: %+v", warnings)
		}
		return errors.New("plural ids repaired")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "plural ids repaired" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestDesignAutoRepairsDuplicateRolesAndUnknownAssetFields(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		body := `{"base":"topdown","objective":"Explore","features":["Collect seeds"],"assets":[{"role":"enemy","fallback":"red boar","view":"top"},{"role":"enemy","fallback":"blue bat"}]}`
		if err := s.PreviewDesignJSON(ctx, run.Job.ID, []byte(body)); err != nil {
			return err
		}
		plan := previewPlan(t, s, run.Job.ID)
		if len(plan.Assets) != 2 || plan.Assets[0].Role != "enemy" || plan.Assets[1].Role != "enemy_2" {
			t.Fatalf("duplicate roles were not suffixed: %+v", plan.Assets)
		}
		return errors.New("duplicate roles repaired")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "duplicate roles repaired" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestTwoDDesignWithoutAssetsGetsFallbackRoles(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"platformer","objective":"Reach the forest exit","features":["Jump across gaps"]}`)); err != nil {
			return err
		}
		plan, err := s.GetPlan(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		if len(plan.Assets) == 0 {
			t.Fatal("2D plan without explicit assets has no roles")
		}
		for _, asset := range plan.Assets {
			if strings.Contains(asset.Fallback, "Replace with") || strings.TrimSpace(asset.Fallback) == "" {
				t.Fatalf("plan asset fallback still carries example prose: %+v", asset)
			}
		}
		return errors.New("default roles filled")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "default roles filled" {
		t.Fatalf("job = %+v", finished)
	}
}

func TestDesignRejectsExamplePlaceholderText(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Replace with the requested objective","features":["A concrete feature"]}`))
		if err == nil || !strings.Contains(err.Error(), "design.objective") {
			t.Fatalf("placeholder objective accepted: %v", err)
		}
		if plan, _ := s.GetPlan(ctx, run.Job.ID); plan != nil {
			t.Fatal("placeholder design published a plan")
		}
		return errors.New("placeholder text rejected")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Error != "placeholder text rejected" {
		t.Fatalf("job = %+v", finished)
	}
}

func previewPlan(t *testing.T, s *Service, jobID string) GamePlan {
	t.Helper()
	raw := s.DesignPreview(jobID)
	if len(raw) == 0 {
		t.Fatal("preview plan missing")
	}
	var plan GamePlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatalf("decode preview plan: %v", err)
	}
	return plan
}
