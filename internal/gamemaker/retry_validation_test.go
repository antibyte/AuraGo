package gamemaker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetryValidatesChangedPublishedGameBeforeModel(t *testing.T) {
	for _, tc := range []struct {
		name, edit string
		validate   bool
		wantReady  bool
	}{
		{name: "changed_retry", edit: "\n// preserved movement-facing shot", validate: true, wantReady: true},
		{name: "ordinary_continuation", edit: "\n// preserved movement-facing shot"},
		{name: "unchanged_retry", validate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			s.SetRunner(testRunner{service: s})
			created, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Build a shooter"})
			if err != nil {
				t.Fatal(err)
			}
			if got := waitJob(t, s, created.ID); got.Status != "ready" || got.ResultRevision != 1 {
				t.Fatalf("published base: %+v", got)
			}
			s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(run.Project))
				}
				stage, err := s.JobDirectory(run.Job.ID)
				if err != nil {
					return err
				}
				path := filepath.Join(stage, "src", "main.ts")
				main, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if err := os.WriteFile(path, append(main, tc.edit...), 0o600); err != nil {
					return err
				}
				return errors.New("provider stopped after preserving draft")
			}))
			failed, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Fix firing direction"})
			if err != nil {
				t.Fatal(err)
			}
			if got := waitJob(t, s, failed.ID); got.Status != "failed" {
				t.Fatalf("draft status: %+v", got)
			}
			var modelCalls int
			s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "building" {
					modelCalls++
					return errors.New("model was invoked")
				}
				return nil
			}))
			retry, err := s.StartJob(context.Background(), project.ID, StartJobRequest{
				Prompt: "Erneut versuchen", Resume: true, ValidateRestoredDraft: tc.validate,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := waitJob(t, s, retry.ID)
			if tc.wantReady {
				if got.Status != "ready" || got.ResultRevision != 2 || modelCalls != 0 {
					t.Fatalf("restored draft was not validated first: job=%+v model_calls=%d", got, modelCalls)
				}
				published, err := os.ReadFile(filepath.Join(s.opts.WorkspacePath, project.ProjectKey, "src", "main.ts"))
				if err != nil || !strings.Contains(string(published), tc.edit) {
					t.Fatalf("corrected source was not published: %v", err)
				}
			} else if got.Status != "failed" || modelCalls != 1 || got.ResultRevision != 0 {
				t.Fatalf("unchanged or ordinary continuation bypassed model: job=%+v model_calls=%d", got, modelCalls)
			}
		})
	}
}
