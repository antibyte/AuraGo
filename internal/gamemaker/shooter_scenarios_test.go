package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func blindShooterScenario() GameScenario {
	return GameScenario{ID: "laser_hits", Metric: "hits", Compare: "increased", Steps: []GameTestStep{
		{Action: "wait", MS: 500},
		{Action: "key", Key: "SPACE", MS: 600}, {Action: "key", Key: "SPACE", MS: 600},
		{Action: "key", Key: "SPACE", MS: 600}, {Action: "key", Key: "SPACE", MS: 600},
		{Action: "key", Key: "SPACE", MS: 600}, {Action: "wait", MS: 500},
	}}
}

func TestShooterScenarioInputPreservesRequirementsAndBudget(t *testing.T) {
	for _, version := range []int{1, 3, 4} {
		plan := GamePlan{SchemaVersion: version, Template: "shooter", Scenarios: []GameScenario{blindShooterScenario()}}
		original, _ := json.Marshal(plan)
		checks := gameScenarios(&plan)
		got := checks[len(checks)-1]
		if err := validateScenario(got); err != nil {
			t.Fatal(err)
		}
		if got.ID != "laser_hits" || got.Metric != "hits" || got.Compare != "increased" || got.Value != 0 || got.inputNote == "" {
			t.Fatalf("requirements changed: %+v", got)
		}
		want := []GameTestStep{{Action: "wait", MS: 500}, {Action: "target", Target: "enemy", Mode: "aim", MS: 3500}}
		if !reflect.DeepEqual(got.Steps, want) {
			t.Fatalf("input budget or spawn wait changed: %+v", got.Steps)
		}
		if again := gameScenarios(&plan); !reflect.DeepEqual(checks, again) {
			t.Fatal("repeated builds changed the test fingerprint")
		}
		after, _ := json.Marshal(plan)
		if string(after) != string(original) {
			t.Fatal("accepted plan was mutated")
		}
		for _, id := range []string{"required_input", "required_primary", "required_assets", "required_restart", "required_end", "required_timed", "required_late_events"} {
			found := false
			for _, check := range checks {
				found = found || check.ID == id
			}
			if !found {
				t.Fatalf("schema %d lost %s", version, id)
			}
		}
		// Only physical input/effects may certify the retained hit requirement.
		obs := GameObservation{ID: got.ID, Before: map[string]float64{"hits": 0}, After: map[string]float64{"hits": 1}}
		if result := compareGameObservations([]GameScenario{got}, []GameObservation{obs})[0]; result.Status != "unavailable" || !strings.Contains(result.Observed, "input adapted:") {
			t.Fatalf("missing evidence passed or adaptation was hidden: %+v", result)
		}
		obs.TargetRuns = []TargetRun{{Target: "enemy", Mode: "aim", Samples: 20, Inputs: 4, Contacts: 1, Effects: 1, Reason: "complete"}}
		if result := compareGameObservations([]GameScenario{got}, []GameObservation{obs})[0]; result.Status != "passed" || !reflect.DeepEqual(result.Steps, want) {
			t.Fatalf("real hit or executed input lost: %+v", result)
		}
		obs.After["hits"] = 0
		if result := compareGameObservations([]GameScenario{got}, []GameObservation{obs})[0]; result.Status != "failed" {
			t.Fatalf("contact without a hit did not fail: %+v", result)
		}
		got.Steps[0].MS = 1
		if plan.Scenarios[0].Steps[0].MS != 500 {
			t.Fatal("execution steps alias the stored plan")
		}
	}
}

func TestShooterScenarioInputLeavesCustomContractsIntact(t *testing.T) {
	for _, name := range []string{"scene", "other_base", "primary", "hit_events", "threshold", "target", "pointer", "movement", "other_key", "passive", "short", "too_long", "too_many"} {
		t.Run(name, func(t *testing.T) {
			plan := GamePlan{Template: "shooter"}
			s := blindShooterScenario()
			switch name {
			case "scene":
				plan.Scene = &Scene{}
			case "other_base":
				plan.Template = "fps"
			case "primary":
				s.Metric = "actions"
			case "hit_events":
				s.Metric = "hit_events"
			case "threshold":
				s.Compare, s.Value = "at_least", 5
			case "target":
				s.Steps[1] = GameTestStep{Action: "target", Target: "boss", Mode: "aim", MS: 1000}
			case "pointer":
				s.Steps[1] = GameTestStep{Action: "pointer", X: 100, Y: 120, MS: 500}
			case "movement":
				s.Steps[1].Key = "LEFT"
			case "other_key":
				s.Steps[1].Key = "F"
			case "passive":
				s.Steps = []GameTestStep{{Action: "wait", MS: 4000}}
			case "short":
				s.Steps = []GameTestStep{{Action: "key", Key: "SPACE", MS: 50}}
			case "too_long":
				s.Steps[0].MS = 4000
			case "too_many":
				s.Steps = append(s.Steps, GameTestStep{Action: "observe"}, GameTestStep{Action: "observe"})
			}
			if got := shooterScenarioInput(&plan, s); !reflect.DeepEqual(got, s) {
				t.Fatalf("custom input was guessed: %+v", got)
			}
		})
	}
}

func TestShooterScenarioInputCapsTargetWindow(t *testing.T) {
	s := blindShooterScenario()
	s.Steps = []GameTestStep{{Action: "key", Key: "SPACE", MS: 3000}, {Action: "wait", MS: 3000}}
	got := shooterScenarioInput(&GamePlan{Template: "shooter"}, s)
	if err := validateScenario(got); err != nil {
		t.Fatal(err)
	}
	want := []GameTestStep{{Action: "target", Target: "enemy", Mode: "aim", MS: 4000}, {Action: "wait", MS: 2000}}
	if !reflect.DeepEqual(got.Steps, want) {
		t.Fatalf("per-command/overall budget changed: %+v", got.Steps)
	}
}

func TestShooterScenarioInputOnResumedBuild(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	plan := ExampleGamePlan(project)
	plan.SchemaVersion, plan.Template = 4, "shooter"
	plan.Scenarios = []GameScenario{blindShooterScenario()}
	var originalPlan, originalSource []byte
	stop := errors.New("fixture stopped before publication")
	s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, plan)
		}
		root, err := s.JobDirectory(run.Job.ID)
		if err != nil {
			return err
		}
		// A failed validation has already built the draft and installed the
		// existing diagnostic prelude. Capture that actual resume baseline.
		if build := s.buildJob(ctx, run.Job.ID, "full"); !build.OK {
			return errors.New("initial fixture did not compile")
		}
		originalPlan, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(gamePlanPath)))
		if err != nil {
			return err
		}
		originalSource, err = os.ReadFile(filepath.Join(root, "src/main.ts"))
		if err != nil {
			return err
		}
		return stop
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Build a space shooter"})
	if err != nil {
		t.Fatal(err)
	}
	if got := waitJob(t, s, job.ID); got.Status != "failed" || !strings.Contains(got.Error, stop.Error()) {
		t.Fatal(got)
	}
	waitContinuationIdle(t, s)
	checked := false
	s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" || run.Job.ResumeFrom != job.ID {
			return errors.New("resume replanned instead of retaining the failed draft")
		}
		build := s.buildJob(ctx, run.Job.ID, "full")
		if !build.OK {
			return errors.New("resumed fixture did not compile")
		}
		grant, err := s.CreatePreviewGrant(project.ID)
		if err != nil {
			return err
		}
		if len(grant.Scenarios) == 0 {
			return errors.New("missing build-bound tests")
		}
		last := grant.Scenarios[len(grant.Scenarios)-1]
		if last.ID != "laser_hits" || len(last.Steps) != 2 || last.Steps[1].Mode != "aim" || last.inputNote == "" {
			return errors.New("resumed preview still received blind firing")
		}
		root, err := s.JobDirectory(run.Job.ID)
		if err != nil {
			return err
		}
		for name, original := range map[string][]byte{gamePlanPath: originalPlan, "src/main.ts": originalSource} {
			got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil || !bytes.Equal(got, original) {
				return fmt.Errorf("resumed build changed %s (original=%d, current=%d bytes; read error=%v)", name, len(original), len(got), err)
			}
		}
		checked = true
		return stop
	}))
	resumed, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	got := waitJob(t, s, resumed.ID)
	waitContinuationIdle(t, s)
	if !checked || got.Status != "failed" || !strings.Contains(got.Error, stop.Error()) {
		t.Fatalf("resumed build did not preserve the source and use targeted input: %+v", got)
	}
}
