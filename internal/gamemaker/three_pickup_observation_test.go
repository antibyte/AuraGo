package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Historical fixtures come from the named repository commits, not user games.
func legacyThreePickupFixture(revision string, plan GamePlan) (string, error) {
	data, err := os.ReadFile("testdata/legacy-three-common-" + revision + ".ts")
	if err != nil {
		return "", err
	}
	expected, err := threeTemplateSources(plan)
	if err != nil {
		return "", err
	}
	common := strings.ReplaceAll(string(expected["common.ts"]), "\r\n", "\n")
	const marker = "import mechanicsPlan from './mechanics.json';\n"
	start := strings.Index(common, marker)
	end := strings.Index(common, "const T = A.THREE;")
	if start < 0 || end <= start+len(marker) {
		return "", fmt.Errorf("missing generated model bindings in fixture")
	}
	return strings.Replace(strings.ReplaceAll(string(data), "\r\n", "\n"), "// PLAN_MODEL_IMPORTS\nconst roles: any = {};\n", common[start+len(marker):end], 1), nil
}

func TestUpgradeThreePickupObservation(t *testing.T) {
	s := newTestService(t)
	design := ExampleGameDesign(Project{Dimension: "3d"})
	design.Base = "fps"
	plan, err := s.planFromDesign(context.Background(), "", Project{Dimension: "3d"}, design)
	if err != nil {
		t.Fatal(err)
	}
	for _, revision := range []string{"0f5d4b125", "6040144e3"} {
		legacy, err := legacyThreePickupFixture(revision, plan)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name, source string
			plan         *GamePlan
			upgrade      bool
		}{
			{"known", legacy, &plan, true},
			{"crlf", strings.ReplaceAll(legacy, "\n", "\r\n"), &plan, true},
			{"missing_plan", legacy, nil, false},
			{"different_bindings", strings.Replace(legacy, "scale:1,base:", "scale:2,base:", 1), &plan, false},
			{"custom_collection", strings.Replace(legacy, "o.role==='item'&&d<1.5", "o.role==='item'&&d<2.5", 1), &plan, false},
			{"custom_reset", strings.Replace(legacy, "health=100;ammo=8;", "health=50;ammo=2;", 1), &plan, false},
			{"custom_snapshot", strings.Replace(legacy, legacyThreePickupMetric, "pickup_events:0", 1), &plan, false},
			{"extra_code", legacy + "\nstartAnotherGame();\n", &plan, false},
		} {
			t.Run(revision+"/"+tc.name, func(t *testing.T) {
				got := upgradeThreePickupObservation(tc.source, tc.plan)
				if (got != tc.source) != tc.upgrade {
					t.Fatalf("unexpected upgrade=%v", got != tc.source)
				}
				if tc.upgrade {
					if strings.Count(got, "pickups++") != 2 || !strings.Contains(got, "time=score=hits=pickups=actions=reloads=0") || !strings.Contains(got, "pickup_events:builder?sceneState.pickup_events:pickups") {
						t.Fatal("missing real collection/reset observation")
					}
					// Only the counter declaration, reset, two actual collection
					// branches and snapshot change; all game rules remain intact.
					restored := strings.NewReplacer("hits=0,pickups=0,actions=0", "hits=0,actions=0", "hits=pickups=actions=reloads=0", "hits=actions=reloads=0", "pickups++;", "", "pickup_events:builder?sceneState.pickup_events:pickups", legacyThreePickupMetric).Replace(got)
					if restored != tc.source {
						t.Fatal("instrumentation changed game rules or source layout")
					}
				}
				if upgradeThreePickupObservation(got, tc.plan) != got {
					t.Fatal("instrumentation is not idempotent")
				}
			})
		}
	}
	current, _ := threeTemplateSources(plan)
	if source := string(current["common.ts"]); upgradeThreePickupObservation(source, &plan) != source {
		t.Fatal("current template was changed")
	}
}

func TestLegacyThreePickupBuildAndResume(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "known", true: "authored"}[custom], func(t *testing.T) {
			s := newTestService(t)
			p := createTestProject(t, s, "3d")
			var source, main, previous string
			s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					design := ExampleGameDesign(p)
					design.Base = "fps"
					data, _ := json.Marshal(design)
					return s.SetDesignJSON(ctx, run.Job.ID, data)
				}
				if previous == "" {
					var err error
					source, err = legacyThreePickupFixture("0f5d4b125", *run.Plan)
					if err != nil {
						return err
					}
					if custom {
						source += "\n// Authored helper: preserve for manual repair.\n"
					}
					if err := s.writeJobFile(ctx, run.Job.ID, "src/common.ts", source); err != nil {
						return err
					}
					main, _ = s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
					previous = run.Job.ID
					return errors.New("saved legacy draft")
				}
				if run.Job.ResumeFrom != previous || run.Stage != "building" {
					t.Error("resume did not preserve accepted plan/build phase")
				}
				runtime := s.RuntimeContext(ctx, run.Job.ID)
				_, compatibility := runtime["observation_compatibility"]
				_, warning := runtime["observation_warnings"]
				if compatibility == custom || warning != custom {
					t.Errorf("runtime observation guidance is incorrect: %+v", runtime)
				}
				result := s.BuildJob(ctx, run.Job.ID)
				if !result.OK {
					t.Errorf("legacy build failed: %+v", result.Diagnostics)
				}
				stage, _ := s.JobDirectory(run.Job.ID)
				built, _ := os.ReadFile(filepath.Join(stage, "dist", "game.js"))
				if strings.Contains(string(built), "pickup_events: builder ? sceneState.pickup_events : pickups") == custom {
					t.Error("build instrumentation did not match verified helper")
				}
				for path, want := range map[string]string{"src/common.ts": source, "src/main.ts": main} {
					got, err := s.ReadJobFile(ctx, run.Job.ID, path)
					// Builds can prepend the existing diagnostics prelude to main.
					if path == "src/main.ts" {
						got = strings.TrimPrefix(got, diagnosticsPrelude+"\n")
					}
					if err != nil || got != want {
						t.Errorf("resume/build changed installed %s", path)
					}
				}
				return errors.New("resumed observation verified")
			}))
			for _, message := range []string{"saved legacy draft", "resumed observation verified"} {
				job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{Resume: previous != ""})
				if err != nil {
					t.Fatal(err)
				}
				if got := waitJob(t, s, job.ID); got.Error != message || got.ResultRevision != 0 {
					t.Fatalf("unexpected completion: %+v", got)
				}
				waitContinuationIdle(t, s)
			}
		})
	}
}
