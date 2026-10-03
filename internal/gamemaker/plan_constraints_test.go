package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestSelectedSpriteMustPassBeforeLegacyPlanAcceptance(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	selected := AssetSelection{PackID: "robots-drones-animated-top-down", AssetID: "service_robot_move_down"}
	planningRounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			planningRounds++
			plan := ExampleGamePlan(project)
			plan.Assets = append(plan.Assets, PlanAsset{
				Role: "custom-prop", Direction: "none", DisplayHeight: 24,
				Origin: Point{.5, .5}, Collider: "none", Fallback: "A hand-drawn lantern",
			})
			if planningRounds == 1 {
				err := s.SetPlan(ctx, run.Job.ID, plan)
				if err == nil || !strings.Contains(err.Error(), selected.PackID+"/"+selected.AssetID) {
					return fmt.Errorf("missing selected sprite was not rejected at submission: %v", err)
				}
				s.mu.RLock()
				accepted := s.acceptedPlans[run.Job.ID]
				s.mu.RUnlock()
				if accepted || s.PlanningComplete(run.Job.ID) {
					return errors.New("invalid selected-asset plan ended planning")
				}
				if stored, err := s.GetPlan(ctx, run.Job.ID); err != nil || stored != nil {
					return fmt.Errorf("invalid selected-asset plan was persisted: plan=%+v err=%v", stored, err)
				}
				return nil
			}
			if len(run.Diagnostics) != 1 || !strings.Contains(run.Diagnostics[0].Message, selected.PackID+"/"+selected.AssetID) {
				return fmt.Errorf("selection correction was not carried into the next round: %+v", run.Diagnostics)
			}
			plan.Assets = append(plan.Assets, PlanAsset{
				Role: "selected-robot", PackID: selected.PackID, Version: "2", AssetID: selected.AssetID,
				Direction: "left", DisplayHeight: 64, Origin: Point{.5, .5}, Collider: "rectangle",
				Animations: []string{"service_robot_move_left"},
			})
			if err := s.SetPlan(ctx, run.Job.ID, plan); err != nil {
				return err
			}
			if !s.PlanningComplete(run.Job.ID) {
				return errors.New("corrected plan was not accepted")
			}
			return nil
		}
		if run.Stage == "building" {
			if len(run.Plan.Assets) != 3 || !slices.ContainsFunc(run.Plan.Assets, func(asset PlanAsset) bool {
				return asset.Role == "custom-prop" && asset.Fallback == "A hand-drawn lantern"
			}) {
				t.Errorf("custom plan assets were lost while preserving the selected sprite: %+v", run.Plan.Assets)
			}
			s.mu.RLock()
			accepted := s.acceptedPlans[run.Job.ID]
			s.mu.RUnlock()
			if !accepted {
				t.Error("accepted plan was revoked before building")
			}
			return errors.New("verified legacy plan acceptance")
		}
		return nil
	}))

	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{AssetSelections: []AssetSelection{selected}})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if planningRounds != 2 || finished.Error != "verified legacy plan acceptance" {
		t.Fatalf("selection correction did not finish in the planning round: rounds=%d job=%+v", planningRounds, finished)
	}
}

func TestSelectedPresentationMustPassBeforeCompactDesignAcceptance(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	presentation := &Presentation{
		Environment: "forest-rain",
		Effects:     []string{"pickup-glow"},
		Sounds:      []SoundBinding{{Event: "pickup", Sound: "pickup"}},
	}
	presentationJSON := `{"environment":"forest-rain","effects":["pickup-glow"],"sounds":[{"event":"pickup","sound":"pickup"}]}`
	planningRounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			planningRounds++
			if planningRounds == 1 {
				err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Find the trail marker","features":["Collect glowing seeds"]}`))
				var detail *DesignValidationError
				if !errors.As(err, &detail) || len(detail.Issues) != 1 || detail.Issues[0].Path != "design.presentation" || detail.RemainingAttempts != 2 {
					return fmt.Errorf("selection mismatch was not a structured design correction: %+v err=%v", detail, err)
				}
				s.mu.RLock()
				accepted := s.acceptedPlans[run.Job.ID]
				s.mu.RUnlock()
				if accepted || s.PlanningComplete(run.Job.ID) {
					return errors.New("invalid selected-presentation design ended planning")
				}
				if stored, err := s.GetPlan(ctx, run.Job.ID); err != nil || stored != nil {
					return fmt.Errorf("invalid selected-presentation design was persisted: plan=%+v err=%v", stored, err)
				}
				return nil
			}
			if len(run.Diagnostics) != 1 || !strings.Contains(run.Diagnostics[0].Message, "design.presentation") {
				return fmt.Errorf("structured correction was not returned to the next planning round: %+v", run.Diagnostics)
			}
			if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"presentation":`+presentationJSON+`}`)); err != nil {
				return err
			}
			return nil
		}
		if run.Stage == "building" {
			if run.Plan.Presentation == nil || run.Plan.Presentation.Environment != presentation.Environment || !slices.Equal(run.Plan.Presentation.Effects, presentation.Effects) || !slices.Equal(run.Plan.Presentation.Sounds, presentation.Sounds) {
				t.Errorf("accepted plan lost selected presentation choices: %+v", run.Plan.Presentation)
			}
			s.mu.RLock()
			accepted := s.acceptedPlans[run.Job.ID]
			s.mu.RUnlock()
			if !accepted {
				t.Error("accepted presentation plan was revoked before building")
			}
			return errors.New("verified compact design acceptance")
		}
		return nil
	}))

	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Presentation: presentation})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if planningRounds != 2 || finished.Error != "verified compact design acceptance" {
		t.Fatalf("presentation correction did not finish in the planning round: rounds=%d job=%+v", planningRounds, finished)
	}
}

// Accepted model bindings survive a selection-less resume in the saved plan.
// New explicit constraints are captured from each resume request; rejected
// request metadata is not stored beyond the failed job.
func TestResumeRechecksAndCarriesSelectedModels(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	firstSelection := []string{"road-sedan"}
	resumeSelection := []string{"aircraft-prop-plane"}
	firstFailed := false
	planningRounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			planningRounds++
			if run.Job.ResumeFrom == "" {
				if !slices.Equal(run.ModelAssetIDs, firstSelection) {
					return fmt.Errorf("initial model selection was lost: %v", run.ModelAssetIDs)
				}
				plan := ExampleGamePlan(project)
				plan.Assets = append(plan.Assets, PlanAsset{Role: "car", PackID: ModelPackID, Version: "1.0.0", AssetID: "road-sedan", Scale: 1, Collider: "catalog"})
				return s.SetPlan(ctx, run.Job.ID, plan)
			}
			if !slices.Equal(run.ModelAssetIDs, resumeSelection) {
				return fmt.Errorf("resumed model selection was lost: %v", run.ModelAssetIDs)
			}
			if len(run.Diagnostics) != 1 || !strings.Contains(run.Diagnostics[0].Message, resumeSelection[0]) {
				return fmt.Errorf("restored plan did not fail the new selection before acceptance: %+v", run.Diagnostics)
			}
			plan := ExampleGamePlan(project)
			plan.Assets = append(plan.Assets, PlanAsset{Role: "plane", PackID: ModelPackID, Version: "1.0.0", AssetID: resumeSelection[0], Scale: 1, Collider: "catalog"})
			return s.SetPlan(ctx, run.Job.ID, plan)
		}
		if run.Stage == "building" {
			if run.Job.ResumeFrom == "" {
				firstFailed = true
				return errors.New("preserve model selection resume fixture")
			}
			if slices.ContainsFunc(run.Plan.Assets, func(asset PlanAsset) bool { return asset.PackID == ModelPackID && asset.AssetID == firstSelection[0] }) {
				return errors.New("preserve accepted model binding across selection-less resume")
			}
			if !slices.ContainsFunc(run.Plan.Assets, func(asset PlanAsset) bool { return asset.PackID == ModelPackID && asset.AssetID == resumeSelection[0] }) {
				return errors.New("accepted resumed plan lost the newly selected model")
			}
			return errors.New("verified resumed model selection")
		}
		return nil
	}))

	first, err := s.StartJob(context.Background(), project.ID, StartJobRequest{ModelAssetIDs: firstSelection})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, first.ID); !firstFailed || !strings.Contains(finished.Error, "preserve model selection resume fixture") {
		t.Fatalf("initial model selection fixture failed: firstFailed=%v job=%+v", firstFailed, finished)
	}

	selectionlessResume, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, selectionlessResume.ID); selectionlessResume.ResumeFrom != first.ID || !strings.Contains(finished.Error, "preserve accepted model binding") {
		t.Fatalf("accepted model binding did not survive a selection-less resume: resume_from=%q job=%+v", selectionlessResume.ResumeFrom, finished)
	}

	resumed, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Resume: true, ModelAssetIDs: resumeSelection})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, resumed.ID)
	if resumed.ResumeFrom != selectionlessResume.ID || planningRounds != 2 || finished.Error != "verified resumed model selection" {
		t.Fatalf("selected model was not enforced on resume: resume_from=%q rounds=%d job=%+v", resumed.ResumeFrom, planningRounds, finished)
	}
	s.mu.RLock()
	_, retained := s.planConstraints[resumed.ID]
	s.mu.RUnlock()
	if retained {
		t.Fatal("completed job retained its selected plan constraints")
	}
}

func TestCompactSelectionErrorsUseTheSharedCorrectionBudget(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	presentation := &Presentation{Effects: []string{"pickup-glow"}}
	planningRounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("selection-invalid design entered building")
		}
		planningRounds++
		for attempt := 0; attempt < 3; attempt++ {
			err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"Explore the courtyard","features":["Collect tokens"]}`))
			var detail *DesignValidationError
			if !errors.As(err, &detail) || len(detail.Issues) != 1 || detail.Issues[0].Path != "design.presentation" || detail.RemainingAttempts != 2-attempt {
				return fmt.Errorf("selection error did not consume attempt %d correctly: detail=%+v err=%v", attempt+1, detail, err)
			}
			if s.PlanningComplete(run.Job.ID) != (attempt == 2) {
				return fmt.Errorf("planning completion after invalid selection attempt %d is wrong", attempt+1)
			}
		}
		if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"presentation":{"effects":["pickup-glow"]}}`)); err == nil || !strings.Contains(err.Error(), "correction limit") {
			return fmt.Errorf("fourth selection attempt was not rejected: %v", err)
		}
		return nil
	}))

	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Presentation: presentation})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if planningRounds != 1 || finished.Status != "failed" || !strings.Contains(finished.Error, "design.presentation: retain the user-selected presentation IDs") || strings.Contains(finished.Error, "correction limit") {
		t.Fatalf("selection failure lost its specific error or exceeded the shared budget: rounds=%d job=%+v", planningRounds, finished)
	}
}
