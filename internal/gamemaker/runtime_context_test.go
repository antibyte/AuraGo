package gamemaker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRuntimeContextIdentifiesInstalledPickupCounter(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		if run.Stage != "building" {
			return errors.New("unexpected repair")
		}
		current, err := gameTemplates.ReadFile("templates/three-common.ts")
		if err != nil {
			return err
		}
		legacy := strings.Replace(string(current), "pickup_events:builder?sceneState.pickup_events:pickups", "pickup_events:builder?sceneState.pickup_events:0", 1)
		if legacy == string(current) {
			return errors.New("missing fixture pickup observation")
		}
		for _, tc := range []struct {
			name, source string
			warning      bool
		}{
			{"current", string(current), false},
			{"legacy", legacy, true},
			{"custom", "export const custom = true;", false},
		} {
			if err := s.writeJobFile(ctx, run.Job.ID, "src/common.ts", tc.source); err != nil {
				return err
			}
			runtime := s.RuntimeContext(ctx, run.Job.ID)
			warnings, _ := runtime["observation_warnings"].([]map[string]any)
			if (len(warnings) == 1) != tc.warning {
				t.Errorf("%s: warning not based on the installed helper", tc.name)
			}
			if len(warnings) == 1 && (warnings[0]["path"] != "src/common.ts" || warnings[0]["metric"] != "pickup_events" || warnings[0]["line"].(int) <= 0) {
				t.Errorf("%s: missing exact repair location", tc.name)
			}
			if after, _ := s.ReadJobFile(ctx, run.Job.ID, "src/common.ts"); after != tc.source {
				t.Error("runtime inspection replaced the installed helper")
			}
		}
		return errors.New("runtime pickup warning verified")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "runtime pickup warning verified" {
		t.Fatal(done.Error)
	}
}
