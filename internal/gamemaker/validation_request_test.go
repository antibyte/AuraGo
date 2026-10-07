package gamemaker

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestValidationRequestErrorsKeepPreviewBudgetAndRound(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			service := newTestService(t)
			project := createTestProject(t, service, dimension)
			service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
				if run.Stage != "building" {
					return nil
				}
				if dimension == "3d" {
					source, err := service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
					if err != nil {
						return err
					}
					if _, err := service.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", source+"\nexport const customRule = 1;", sourceHash(source)); err != nil {
						return err
					}
				}
				build := service.BuildJob(ctx, run.Job.ID)
				if !build.OK {
					return fmt.Errorf("initial build: %s", diagnosticsText(build.Diagnostics))
				}
				stop := service.StopAfterValidation(run.Job.ID, true)
				remaining := service.RemainingRepairs(run.Job.ID)
				service.mu.RLock()
				preview, previous := service.previewCheck, service.lastValidation[run.Job.ID]
				service.mu.RUnlock()
				requests := []struct {
					scope string
					ids   []string
				}{
					{"invalid", nil},
					{"full", []string{""}},
					{"full", []string{"required_end", "required_end"}},
					{"startup", []string{"required_end"}},
					{"full", []string{"unknown_check"}},
				}
				if dimension == "3d" {
					requests = append(requests, struct {
						scope string
						ids   []string
					}{"full", nil})
				}
				for _, request := range requests {
					result := service.ValidateJobScope(ctx, run.Job.ID, request.scope, request.ids...)
					if result.OK || len(result.Diagnostics) != 1 || result.Diagnostics[0].Level != "request" {
						return fmt.Errorf("%s/%v: expected request rejection, got %+v", request.scope, request.ids, result)
					}
					if next := ValidationNextAction(result); !strings.Contains(next, "Correct the validation request") || strings.Contains(next, "preview must stay open") {
						return fmt.Errorf("incorrect request recovery: %s", next)
					}
					service.mu.RLock()
					unchanged := service.previewCheck == preview && service.lastValidation[run.Job.ID] == previous
					service.mu.RUnlock()
					if !unchanged || stop() || service.RemainingRepairs(run.Job.ID) != remaining {
						return fmt.Errorf("rejected request changed the preview, validation, budget or round")
					}
				}
				// A corrected request still requires fresh browser evidence and ends
				// the repair round only after an actual validation.
				result := service.ValidateJobScope(ctx, run.Job.ID, "startup")
				if !result.OK || !stop() {
					return fmt.Errorf("corrected request did not validate and end the round: %+v", result)
				}
				return nil
			}})
			job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if finished := waitJob(t, service, job.ID); finished.Status != "ready" {
				t.Fatalf("job failed after request correction: %+v", finished)
			}
		})
	}
}

func TestFreeCodeThreePlanningDoesNotPromiseScenarioOnlyGameplay(t *testing.T) {
	note := BaseChecks("3d")["three"]["startup"]
	for _, want := range []string{"gameplay unverified", "3D scene data", "scenarios alone do not enable"} {
		if !strings.Contains(note, want) {
			t.Errorf("planning help %q must disclose %q", note, want)
		}
	}
	plan := &GamePlan{SchemaVersion: 4, Template: "three", Scenarios: []GameScenario{{ID: "custom"}}}
	if sceneBackedCurrentAt(t.TempDir(), plan) {
		t.Fatal("declared scenarios unexpectedly enabled free-code Three gameplay")
	}
}
