package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type continuationRunner func(context.Context, JobRun) error

func (f continuationRunner) RunGameMakerJob(ctx context.Context, run JobRun) error {
	return f(ctx, run)
}

func waitContinuationIdle(t *testing.T, s *Service) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		s.mu.RLock()
		idle := s.activeJobID == ""
		s.mu.RUnlock()
		if idle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("writer not released")
}

func TestContinuationRestoresPrivateContextAndWorkingCopy(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			s := newTestService(t)
			p := createTestProject(t, s, dimension)
			var firstID string
			payload := json.RawMessage(`[{"role":"assistant","reasoning_content":"keep the unusual mechanic","content":"progress"}]`)
			const source = "// original mechanic must survive\nexport const mechanic = 13;"
			s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(p))
				}
				firstID = run.Job.ID
				if err := os.WriteFile(filepath.Join(s.stagingDir, run.Job.ID, "src", "main.ts"), []byte(source), 0600); err != nil {
					return err
				}
				if err := s.SaveAgentConversation(ctx, run.Job.ID, "provider", "model", payload); err != nil {
					return err
				}
				return errors.New("simulated provider disconnect")
			}))
			job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{Prompt: "Build an unusual runner"})
			if err != nil {
				t.Fatal(err)
			}
			if got := waitJob(t, s, job.ID); got.Status != "failed" {
				t.Fatal(got)
			}
			waitContinuationIdle(t, s)
			// Persist through a service/database reopen, not merely in process memory.
			opts := s.opts
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = NewService(opts)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.SetSkillStatus(nil, true)
			// A failed copy (for example, a newly reduced file limit) must not
			// destroy or hide the last recoverable working copy.
			limit := s.opts.MaxFilesPerProject
			s.opts.MaxFilesPerProject = 1
			copyJob, err := s.StartJob(context.Background(), p.ID, StartJobRequest{Resume: true})
			if err != nil {
				t.Fatal(err)
			}
			copyResult := waitJob(t, s, copyJob.ID)
			waitContinuationIdle(t, s)
			if !strings.Contains(copyResult.Error, "restore game maker working copy") {
				t.Fatal(copyResult.Error)
			}
			s.opts.MaxFilesPerProject = limit
			if data, err := os.ReadFile(filepath.Join(s.stagingDir, firstID, "src", "main.ts")); err != nil || string(data) != source {
				t.Fatal("failed restore destroyed original draft")
			}
			checked := 0
			s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
				if run.Job.ResumeFrom != firstID {
					return errors.New("missing resumed working copy")
				}
				content, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
				if err != nil || content != source {
					return errors.New("source replaced by scaffold/template")
				}
				c, err := s.LoadAgentConversation(ctx, run.Job.ID)
				if err != nil || string(c.Messages) != string(payload) {
					return errors.New("private reasoning lost")
				}
				requests, err := s.PreviousJobRequests(ctx, run.Job.ID)
				if err != nil || len(requests) != 2 || requests[0] != "Build an unusual runner" {
					return errors.New("original task lost")
				}
				checked++
				if run.Stage == "planning" {
					return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(p))
				}
				return errors.New("fixture stopped after resumed building")
			}))
			job, err = s.StartJob(context.Background(), p.ID, StartJobRequest{Resume: true})
			if err != nil {
				t.Fatal(err)
			}
			got := waitJob(t, s, job.ID)
			waitContinuationIdle(t, s)
			if checked != 1 || !strings.Contains(got.Error, "fixture stopped") {
				t.Fatalf("resume failed: checked=%d job=%+v", checked, got)
			}
			messages, _ := s.ListMessages(context.Background(), p.ID)
			for _, m := range messages {
				if strings.Contains(m.Content, "unusual mechanic") {
					t.Fatal("private reasoning leaked to chat")
				}
			}
			if _, err := os.Stat(filepath.Join(s.stagingDir, firstID)); !os.IsNotExist(err) {
				t.Fatal("obsolete draft not removed")
			}
			other := createTestProject(t, s, dimension)
			s.SetRunner(continuationRunner(func(ctx context.Context, run JobRun) error {
				c, err := s.LoadAgentConversation(ctx, run.Job.ID)
				if err != nil || len(c.Messages) != 0 {
					t.Error("cross-project history leak")
				}
				return errors.New("other project")
			}))
			otherJob, err := s.StartJob(context.Background(), other.ID, StartJobRequest{Resume: true})
			if err != nil {
				t.Fatal(err)
			}
			waitJob(t, s, otherJob.ID)
			waitContinuationIdle(t, s)
			if err := s.DeleteProject(context.Background(), p.ID); err != nil {
				t.Fatal(err)
			}
			var remaining int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM gm_agent_context WHERE project_id=?`, p.ID).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatal("project deletion retained private context")
			}
			if _, err := os.Stat(filepath.Join(s.stagingDir, job.ID)); !os.IsNotExist(err) {
				t.Fatal("project deletion retained working copy")
			}
		})
	}
}
