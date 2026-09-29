package gamemaker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// driveJobWithReports plays the Studio parent: it answers every new validation
// with a ready report plus the observations chosen by the test.
func driveJobWithReports(t *testing.T, service *Service, job Job, observe func([]GameScenario) []GameObservation) Job {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		current, err := service.GetJob(context.Background(), job.ID)
		service.mu.RLock()
		validationID := ""
		if service.previewCheck != nil {
			validationID = service.previewCheck.ID
		}
		service.mu.RUnlock()
		if err == nil && validationID != "" && validationID != last {
			if grant, grantErr := service.CreatePreviewGrant(current.ProjectID); grantErr == nil {
				_ = service.ReportPreview(current.ProjectID, PreviewReport{Token: grant.Token, Type: "ready", CanvasVisible: true})
				if len(grant.Scenarios) > 0 {
					_ = service.ReportPreview(current.ProjectID, PreviewReport{Token: grant.Token, Type: "gameplay", Observations: observe(grant.Scenarios)})
				}
				last = grant.ValidationID
			}
		}
		if err == nil && !activeJobStatus(current.Status) {
			return current
		}
		time.Sleep(10 * time.Millisecond)
	}
	current, _ := service.GetJob(context.Background(), job.ID)
	t.Fatalf("job %s did not finish: status=%s phase=%s error=%s", job.ID, current.Status, current.Phase, current.Error)
	return Job{}
}

func missingTargetObservations(scenarios []GameScenario, checkID string) []GameObservation {
	observations := successfulObservationFixture(scenarios)
	for i := range observations {
		if observations[i].ID != checkID {
			continue
		}
		observations[i].After = map[string]float64{}
		for key, value := range observations[i].Before {
			observations[i].After[key] = value
		}
		for run := range observations[i].TargetRuns {
			observations[i].TargetRuns[run].Samples = 0
			observations[i].TargetRuns[run].Inputs = 0
			observations[i].TargetRuns[run].Contacts = 0
			observations[i].TargetRuns[run].Effects = 0
			observations[i].TargetRuns[run].Reason = "no_target"
		}
	}
	return observations
}

// A required check whose target does not exist in the running game is a source
// defect the agent can repair. It must reach the bounded repair loop instead of
// failing the whole job as if the browser had been unavailable.
func TestMissingTargetStartsBoundedRepair(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var mu sync.Mutex
	repairs := 0
	var repair JobRun
	var repairResult BuildResult
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage == "repair" {
			mu.Lock()
			repairs++
			repair = run
			if run.Result != nil {
				// The orchestrator reuses this value for the next validation.
				repairResult = *run.Result
			}
			mu.Unlock()
			source, err := service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
			if err != nil {
				return err
			}
			return service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", source+"\n// bound the item role to a real object\n")
		}
		result := service.BuildJob(ctx, run.Job.ID)
		if !result.OK {
			return errors.New(diagnosticsText(result.Diagnostics))
		}
		return nil
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := driveJobWithReports(t, service, job, func(scenarios []GameScenario) []GameObservation {
		mu.Lock()
		repaired := repairs > 0
		mu.Unlock()
		if repaired {
			return successfulObservationFixture(scenarios)
		}
		return missingTargetObservations(scenarios, "required_rules")
	})
	mu.Lock()
	defer mu.Unlock()
	if repairs != 1 {
		t.Fatalf("repair rounds = %d, want 1; job=%+v", repairs, finished)
	}
	if finished.Status != "ready" || finished.ResultRevision != 1 {
		t.Fatalf("repaired game was not published: %+v", finished)
	}
	found := false
	for _, check := range repair.Checks {
		if check.ID == "required_rules" {
			found = check.Status == "unavailable" && check.Repairable && strings.Contains(check.Observed, "no_target")
		}
	}
	if !found {
		t.Fatalf("repair run lost the missing-target evidence: %+v", repair.Checks)
	}
	if !repairResult.Repairable || repairResult.OK || repairResult.GameplayStatus != "unavailable" {
		t.Fatalf("repair result = %+v", repairResult)
	}
}

// The shared budget still bounds repairs of unverifiable checks.
func TestMissingTargetRepairSharesBudget(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var mu sync.Mutex
	repairs := 0
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage == "repair" {
			mu.Lock()
			repairs++
			attempt := repairs
			mu.Unlock()
			source, err := service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
			if err != nil {
				return err
			}
			return service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", source+"\n// attempt "+string(rune('0'+attempt))+"\n")
		}
		result := service.BuildJob(ctx, run.Job.ID)
		if !result.OK {
			return errors.New(diagnosticsText(result.Diagnostics))
		}
		return nil
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := driveJobWithReports(t, service, job, func(scenarios []GameScenario) []GameObservation {
		return missingTargetObservations(scenarios, "required_rules")
	})
	mu.Lock()
	defer mu.Unlock()
	if repairs != 3 {
		t.Fatalf("repair rounds = %d, want the shared limit of 3", repairs)
	}
	if finished.Status != "failed" || !strings.Contains(finished.Error, "required_rules") || !strings.Contains(finished.Error, "no_target") {
		t.Fatalf("job = %+v", finished)
	}
	if current, _ := service.GetProject(context.Background(), project.ID); current.CurrentRevision != 0 {
		t.Fatalf("unverified game was published: %+v", current)
	}
}

// Missing harness evidence is not a source defect: no model call can repair it.
func TestMissingObservationStillEndsWithoutRepair(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var mu sync.Mutex
	repairs := 0
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage == "repair" {
			mu.Lock()
			repairs++
			mu.Unlock()
			return nil
		}
		result := service.BuildJob(ctx, run.Job.ID)
		if !result.OK {
			return errors.New(diagnosticsText(result.Diagnostics))
		}
		return nil
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := driveJobWithReports(t, service, job, func(scenarios []GameScenario) []GameObservation {
		observations := successfulObservationFixture(scenarios)
		kept := observations[:0]
		for _, observation := range observations {
			if observation.ID != "required_timed" {
				kept = append(kept, observation)
			}
		}
		return kept
	})
	mu.Lock()
	defer mu.Unlock()
	if repairs != 0 || finished.Status != "failed" {
		t.Fatalf("missing observation started repairs=%d, job=%+v", repairs, finished)
	}
}

func TestRepairableCheckClassification(t *testing.T) {
	scenario := GameScenario{ID: "required_rules", Metric: "pickup_events", Compare: "increased", Steps: []GameTestStep{{Action: "target", Mode: "reach", Target: "item", MS: 4000}}}
	observation := func(run TargetRun, before, after float64) []GameObservation {
		return []GameObservation{{ID: scenario.ID, Before: map[string]float64{"pickup_events": before}, After: map[string]float64{"pickup_events": after}, TargetRuns: []TargetRun{run}}}
	}
	for name, test := range map[string]struct {
		observations []GameObservation
		status       string
		repairable   bool
	}{
		"missing target":      {observation(TargetRun{Target: "item", Mode: "reach", Reason: "no_target"}, 0, 0), "unavailable", true},
		"blocked route":       {observation(TargetRun{Target: "item", Mode: "reach", Samples: 40, Inputs: 40, Reason: "blocked"}, 0, 0), "unavailable", true},
		"inactive target":     {observation(TargetRun{Target: "item", Mode: "reach", Samples: 4, Inputs: 0, Reason: "inactive"}, 0, 0), "unavailable", true},
		"counter only":        {observation(TargetRun{Target: "item", Mode: "reach", Samples: 40, Inputs: 40, Contacts: 1, Effects: 0, Reason: "complete"}, 0, 1), "unavailable", true},
		"observed effect":     {observation(TargetRun{Target: "item", Mode: "reach", Samples: 40, Inputs: 40, Contacts: 1, Effects: 1, Reason: "complete"}, 0, 1), "passed", false},
		"no observation":      {nil, "unavailable", false},
		"mismatched evidence": {observation(TargetRun{Target: "enemy", Mode: "reach", Samples: 40, Inputs: 40, Contacts: 1, Effects: 1, Reason: "complete"}, 0, 1), "unavailable", false},
	} {
		t.Run(name, func(t *testing.T) {
			checks := compareGameObservations([]GameScenario{scenario}, test.observations)
			if len(checks) != 1 || checks[0].Status != test.status || checks[0].Repairable != test.repairable {
				t.Fatalf("checks = %+v", checks)
			}
		})
	}
}
