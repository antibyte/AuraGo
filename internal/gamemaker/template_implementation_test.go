package gamemaker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadOnlyBuildingYieldsToImplementationRepair(t *testing.T) {
	for _, base := range []string{"platformer", "fps", "three"} {
		t.Run(base, func(t *testing.T) {
			s := newTestService(t)
			dimension := "3d"
			if base == "platformer" {
				dimension = "2d"
			}
			project := createTestProject(t, s, dimension)
			repairs := 0
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					return s.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Custom game","features":["custom rules"]}`, base)))
				}
				if run.Stage == "repair" {
					repairs++
					if !strings.Contains(diagnosticsText(run.Diagnostics), "No implementation was written") {
						t.Errorf("missing implementation handoff: %+v", run.Diagnostics)
					}
					return fmt.Errorf("verified implementation handoff")
				}
				past := time.Now().Add(-time.Second)
				if s.StopAfterValidation(run.Job.ID, false)() || s.stopAfterValidation(run.Job.ID, true, past)() {
					t.Error("fresh building or repair lost its budget")
				}
				stop := s.stopAfterValidation(run.Job.ID, false, past)
				if !stop() {
					t.Error("read-only building did not yield after exploration deadline")
				}
				// A real existing revision must never trigger new-starter recovery.
				if _, err := s.db.ExecContext(ctx, "UPDATE gm_jobs SET base_revision=1 WHERE id=?", run.Job.ID); err != nil {
					return err
				}
				if s.stopAfterValidation(run.Job.ID, false, past)() {
					t.Error("existing game lost its editing budget")
				}
				if _, err := s.db.ExecContext(ctx, "UPDATE gm_jobs SET base_revision=0 WHERE id=?", run.Job.ID); err != nil {
					return err
				}
				for _, path := range []string{"src/main.ts", "src/common.ts", "src/scene.json", "src/mechanics.json"} {
					before, err := s.ReadJobFile(ctx, run.Job.ID, path)
					if err != nil {
						continue // Free Three.js has no common helper.
					}
					content := before + "\nexport const customRule = 1;\n"
					if strings.HasSuffix(path, ".json") {
						content = `{"customRule":1}`
					}
					// Exercise starter comparison directly; separate scene tests own schema validation.
					stage, _ := s.JobDirectory(run.Job.ID)
					if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(path)), []byte(content), 0o640); err != nil {
						return err
					}
					if s.stopAfterValidation(run.Job.ID, false, past)() {
						t.Errorf("work in %s was treated as read-only exploration", path)
					}
					if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(path)), []byte(before), 0o640); err != nil {
						return err
					}
				}
				return nil // Orchestrator must still reject the restored unchanged starter.
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "verified implementation handoff" || repairs != 1 || done.ResultRevision != 0 {
				t.Fatalf("unexpected completion: %+v; repairs=%d", done, repairs)
			}
		})
	}
}

func TestStalledImplementationYieldsToValidation(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"platformer","objective":"Collect coins","features":["Custom coins"]}`))
		}
		source, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		// Shorten only the polling interval; the already-running job retains its context.
		s.opts.JobTimeout = 40 * time.Millisecond
		stop := s.stopAfterValidation(run.Job.ID, false, time.Now().Add(-time.Second))
		for i := range 2 {
			if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", source+fmt.Sprintf("\nexport const customRule = %d;", i), ""); err != nil {
				return err
			}
			time.Sleep(20 * time.Millisecond)
			if stop() {
				t.Fatal("productive source changes ended the building round")
			}
		}
		time.Sleep(20 * time.Millisecond)
		if _, err := s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 1, 5); err != nil {
			return err
		}
		if _, err := s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "absent", "replacement", "wrong-hash"); err == nil {
			return fmt.Errorf("invalid edit accepted")
		}
		if !stop() {
			t.Error("reads and failed edits prolonged a stalled implementation")
		}
		if s.stopAfterValidation(run.Job.ID, true, time.Now().Add(-time.Second))() {
			t.Error("repair budget changed")
		}
		return fmt.Errorf("verified stall handoff")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "verified stall handoff" {
		t.Fatal(done.Error)
	}
}

func TestUnimplemented2DTemplatesCannotPublish(t *testing.T) {
	for _, template := range templateNames()[:6] {
		t.Run(template, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			repairs := 0
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					plan := ExampleGamePlan(project)
					plan.Template, plan.Width, plan.Height = template, 1024, 576
					plan.Assets = []PlanAsset{{Role: "tree", PackID: "nature-assets-top-down", Version: "2", AssetID: "pine_tree", Direction: "none", DisplayHeight: 80, Origin: Point{X: .5, Y: .5}, Collider: "circle"}}
					return s.SetPlan(ctx, run.Job.ID, plan)
				}
				if run.Stage == "repair" {
					repairs++
					if !strings.Contains(diagnosticsText(run.Diagnostics), "No implementation was written") {
						t.Errorf("missing implementation diagnostic: %+v", run.Diagnostics)
					}
				}
				// Building injects diagnostics and loads planned assets. Neither is
				// an implementation of the user's requested game.
				result := s.BuildJob(ctx, run.Job.ID)
				if !result.OK {
					return fmt.Errorf("fixture build: %s", diagnosticsText(result.Diagnostics))
				}
				return nil
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "a fps game in the woods"})
			if err != nil {
				t.Fatal(err)
			}
			done := waitJob(t, s, job.ID)
			if done.Status != "failed" || done.ResultRevision != 0 || repairs == 0 || !strings.Contains(done.Error, "No implementation was written") {
				t.Fatalf("unchanged template was accepted: %+v repairs=%d", done, repairs)
			}
		})
	}
}

func TestTemplateImplementationCanLiveInSharedSource(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		if run.Stage == "visual" {
			return nil
		}
		common, err := s.ReadJobFile(ctx, run.Job.ID, "src/common.ts")
		if err != nil {
			return err
		}
		common = strings.Replace(common, "this.state = { score: 0", "this.state = { score: 25", 1)
		return s.WriteJobFile(ctx, run.Job.ID, "src/common.ts", common)
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("shared-source implementation was rejected: %+v", done)
	}
}
