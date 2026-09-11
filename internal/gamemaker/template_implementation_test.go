package gamemaker

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

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
