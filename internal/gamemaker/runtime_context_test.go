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

func TestRuntimeContextRejectsRetainedDescriptorOnChangedHelper(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		original, err := s.ReadJobFile(ctx, run.Job.ID, "src/common.ts")
		if err != nil {
			return err
		}
		changed := strings.Replace(original, "damagePlayer(", "authoredDamage(", 1)
		if changed == original {
			return errors.New("missing damagePlayer fixture")
		}
		for _, tc := range []struct {
			name, source string
			verified     bool
		}{
			{"original", original, true},
			{"CRLF", strings.ReplaceAll(strings.ReplaceAll(original, "\r\n", "\n"), "\n", "\r\n"), true},
			{"changed method, retained marker", changed, false},
			{"unknown version", strings.Replace(original, "phaser-2", "phaser-999", 1), false},
			{"missing descriptor", "export class GameScene { setup() {} }", false},
		} {
			if err := s.writeJobFile(ctx, run.Job.ID, "src/common.ts", tc.source); err != nil {
				return err
			}
			runtime := s.RuntimeContext(ctx, run.Job.ID)
			_, hasAPI := runtime["api_reference"]
			if hasAPI != tc.verified || !tc.verified && (runtime["api"] != nil || runtime["version"] != "legacy/custom") {
				t.Errorf("%s: helper verification mismatch: version=%v, hasAPI=%v", tc.name, runtime["version"], hasAPI)
			}
			files := runtime["files"].([]map[string]any)
			if files[0]["sha256"] != sourceHash(tc.source) || len(files[0]["hooks"].([]SourceMatch)) == 0 {
				t.Errorf("%s: current helper hash/hook locations missing", tc.name)
			}
			if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/common.ts"); got != tc.source {
				t.Errorf("%s: inspection changed source", tc.name)
			}
		}
		return errors.New("helper verification complete")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "helper verification complete" {
		t.Fatal(done.Error)
	}
}

func TestVerifiedTemplateDescriptorCoversEveryEngine(t *testing.T) {
	s := newTestService(t)
	for _, project := range []Project{{Dimension: "2d"}, {Dimension: "3d"}, {Dimension: "3d", Variant: "voxel"}} {
		plan, err := s.planFromDesign(context.Background(), "", project, ExampleGameDesign(project))
		if err != nil {
			t.Fatal(err)
		}
		files, err := gameTemplateSources(plan)
		if err != nil {
			t.Fatal(err)
		}
		common := string(files["common.ts"])
		for _, source := range []string{common, strings.ReplaceAll(strings.ReplaceAll(common, "\r\n", "\n"), "\n", "\r\n")} {
			descriptor := verifiedTemplateDescriptor(&plan, source)
			version, _ := descriptor["version"].(string)
			if runtimeReferences[version].Text == "" {
				t.Errorf("%s/%s has no verified reference", project.Dimension, project.Variant)
			}
		}
		if verifiedTemplateDescriptor(&plan, common+"\n// custom") != nil || verifiedTemplateDescriptor(nil, common) != nil {
			t.Errorf("%s/%s trusted an unverified helper", project.Dimension, project.Variant)
		}
	}
}
