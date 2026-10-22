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

// A comment-only or body-only edit keeps the documented API; adding or changing
// an exported symbol or a hook signature still drops to legacy/custom.
func TestRuntimeContextToleratesEditsToVerifiedHelper(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		if run.Stage != "building" {
			return errors.New("unexpected stage")
		}
		plan, err := s.GetPlan(ctx, run.Job.ID)
		if err != nil || plan == nil {
			return errors.New("accepted plan is unavailable")
		}
		sources, err := gameTemplateSources(*plan)
		if err != nil {
			return err
		}
		base := string(sources["common.ts"])
		renameCodeHook := func(source string) string {
			lines := strings.Split(source, "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
					continue
				}
				if strings.Contains(line, "step(") {
					lines[i] = strings.Replace(line, "step(", "stepFixed(", 1)
					return strings.Join(lines, "\n")
				}
			}
			return source
		}
		changedHook := renameCodeHook(base)
		if changedHook == base {
			t.Fatal("fixture has no code-level step hook")
		}
		changedExport := strings.Replace(base, "export class ", "export class Renamed", 1)
		if changedExport == base {
			t.Fatal("fixture has no exported class")
		}
		for _, tc := range []struct {
			name          string
			source        string
			wantReference bool
			wantWarning   bool
		}{
			{"exact", base, true, false},
			{"comment", "// reviewed helper edit\n" + base, true, true},
			{"body", base + "\nconst helperNote = 1;\n", true, true},
			{"new export", base + "\nexport const extraHelper = 1;\n", false, false},
			{"changed hook", changedHook, false, false},
			{"changed export", changedExport, false, false},
			{"custom", "export const custom = true;", false, false},
		} {
			if err := s.writeJobFile(ctx, run.Job.ID, "src/common.ts", tc.source); err != nil {
				return err
			}
			runtime := s.RuntimeContext(ctx, run.Job.ID)
			_, hasReference := runtime["api_reference"]
			warnings, _ := runtime["helper_warnings"].([]map[string]any)
			if hasReference != tc.wantReference || (len(warnings) > 0) != tc.wantWarning {
				t.Errorf("%s: reference=%v warnings=%d", tc.name, hasReference, len(warnings))
			}
			if tc.wantWarning && len(warnings) == 1 {
				if warnings[0]["kind"] != "comments_or_bodies_changed" || warnings[0]["path"] != "src/common.ts" {
					t.Errorf("%s: unexpected helper warning %+v", tc.name, warnings[0])
				}
			}
		}
		return errors.New("helper tolerance verified")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "helper tolerance verified" {
		t.Fatal(done.Error)
	}
}
