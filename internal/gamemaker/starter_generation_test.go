package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestStarterGenerationRequiresFreshExactTemplate(t *testing.T) {
	for _, tc := range []struct {
		dimension, base string
		ready           bool
	}{
		{"2d", "platformer", true}, {"3d", "fps", true},
		{"2d", "minimal", false}, {"3d", "three", false},
	} {
		t.Run(tc.base, func(t *testing.T) {
			s := newTestService(t)
			p := createTestProject(t, s, tc.dimension)
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					return s.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Test game","features":["distinct requested rules"]}`, tc.base)))
				}
				ready, err := s.StarterGenerationReady(ctx, run.Job)
				if err != nil || ready != tc.ready {
					t.Errorf("initial routing = %v, %v; want %v", ready, err, tc.ready)
				}
				if tc.ready {
					for _, path := range []string{"src/main.ts", "src/common.ts", "src/scene.json", "src/mechanics.json"} {
						stage, _ := s.JobDirectory(run.Job.ID)
						full := filepath.Join(stage, filepath.FromSlash(path))
						before, err := os.ReadFile(full)
						if err != nil {
							return err
						}
						if err = os.WriteFile(full, append(append([]byte(nil), before...), '\n'), 0o600); err != nil {
							return err
						}
						ready, err = s.StarterGenerationReady(ctx, run.Job)
						if err != nil || ready {
							t.Errorf("modified %s selected direct generation: %v, %v", path, ready, err)
						}
						if err = os.WriteFile(full, before, 0o600); err != nil {
							return err
						}
					}
					resumed := run.Job
					resumed.ResumeFrom = "server-owned-prior-job"
					if ready, err := s.StarterGenerationReady(ctx, resumed); err != nil || ready {
						t.Errorf("continuation selected direct generation: %v, %v", ready, err)
					}
				}
				return errors.New("routing verified")
			}))
			job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "routing verified" {
				t.Fatalf("routing fixture: %+v", done)
			}
		})
	}
}
