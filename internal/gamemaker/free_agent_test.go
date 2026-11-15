package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type freeAgentRunner func(context.Context, JobRun) error

func (f freeAgentRunner) RunGameMakerJob(ctx context.Context, run JobRun) error { return f(ctx, run) }

func TestFreeAgentPublishRestoreExport(t *testing.T) {
	for _, template := range []string{"html", "minimal", "three", "voxel"} {
		t.Run(template, func(t *testing.T) {
			s := newTestService(t)
			s.opts.MaxProjectBytes = 100 << 20
			s.opts.MaxFilesPerProject = 250
			s.opts.JobTimeout = time.Minute
			s.SetSkillStatus(nil, false)
			project := createTestProject(t, s, "2d")
			s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage != "building" {
					return fmt.Errorf("unexpected phase %s", run.Stage)
				}
				if template != "html" && run.Job.BaseRevision == 0 {
					if err := s.UseTemplate(ctx, run.Job.ID, template); err != nil {
						return err
					}
				}
				page, err := s.ReadJobFile(ctx, run.Job.ID, "index.html")
				if err != nil {
					return err
				}
				if _, err = s.WriteJobFileChecked(ctx, run.Job.ID, "index.html", page+"\n<!-- edited -->", sourceHash(page)); err != nil {
					return err
				}
				s.mu.RLock()
				check := s.previewCheck
				s.mu.RUnlock()
				if check != nil {
					return fmt.Errorf("saving unexpectedly built the project")
				}
				before, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
				result := s.FinishJob(ctx, run.Job.ID)
				if !result.OK {
					return fmt.Errorf("finish: %+v", result.Diagnostics)
				}
				after, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
				if before != after {
					return fmt.Errorf("build rewrote source")
				}
				// Runtime errors are advice, never a publication gate.
				s.mu.Lock()
				result.check.Diagnostics = append(result.check.Diagnostics, Diagnostic{Level: "runtime", Message: "visible runtime error"})
				s.mu.Unlock()
				return nil
			}))
			for i := 1; i <= 2; i++ {
				job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "edit freely"})
				if err != nil {
					t.Fatal(err)
				}
				job = waitJob(t, s, job.ID)
				if job.Status != "ready" || job.ResultRevision != int64(i) {
					t.Fatalf("job: %+v", job)
				}
				published, _ := s.GetProject(context.Background(), project.ID)
				if template == "voxel" && (published.Dimension != "3d" || published.Variant != "voxel") {
					t.Fatalf("template publication metadata: %+v", published)
				}
			}
			if _, err := s.RestoreRevision(context.Background(), project.ID, 1); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if _, err := s.WriteExport(context.Background(), project.ID, &output); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, file := range archive.File {
				if file.Name == "index.html" {
					found = true
				}
				if strings.HasPrefix(file.Name, ".") {
					t.Fatalf("private export: %s", file.Name)
				}
			}
			if !found {
				t.Fatal("missing entry")
			}
		})
	}
}

func TestFreeAgentDraftAndCompilerFailures(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", "const broken = ;", ""); err != nil {
			return err
		}
		result := s.FinishJob(ctx, run.Job.ID)
		if result.OK || s.JobFinished(run.Job.ID) || len(result.Diagnostics) == 0 || result.Diagnostics[0].File != "src/main.ts" || result.Diagnostics[0].Line != 1 {
			return fmt.Errorf("bad compiler result: %+v", result)
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "styles.css", "body { color: red }", ""); err != nil {
			return err
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "styles.css", "body{}", "stale"); err == nil {
			return fmt.Errorf("accepted stale overwrite")
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "../outside.txt", "bad", ""); err == nil {
			return fmt.Errorf("accepted escaped path")
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "create"})
	if err != nil {
		t.Fatal(err)
	}
	job = waitJob(t, s, job.ID)
	if job.Status != "failed" || job.ResultRevision != 0 {
		t.Fatalf("unexpected publication: %+v", job)
	}
	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		if run.Job.ResumeFrom == "" {
			return fmt.Errorf("draft not restored")
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", "console.log('ready');", ""); err != nil {
			return err
		}
		if result := s.FinishJob(ctx, run.Job.ID); !result.OK {
			return fmt.Errorf("finish: %+v", result.Diagnostics)
		}
		return nil
	}))
	job, err = s.StartJob(context.Background(), project.ID, StartJobRequest{Resume: true, Prompt: "continue"})
	if err != nil {
		t.Fatal(err)
	}
	if job = waitJob(t, s, job.ID); job.Status != "ready" {
		t.Fatalf("resume: %+v", job)
	}
}

func TestFreeAgentBrowserPreservesState(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 for Chromium")
	}
	s := newTestService(t)
	s.opts.JobTimeout = 3 * time.Minute
	project := createTestProject(t, s, "2d")
	var snapshotDir string
	s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
		page := `<!doctype html><html><body style="background:#156;color:white;font:40px sans-serif"><h1 id="count">0</h1><script>window.count=0;addEventListener('keydown',()=>document.getElementById('count').textContent=String(++window.count)); console.error('first-build-only'); Promise.reject(new Error('promise-feedback'));</script><img src="missing.png"></body></html>`
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "index.html", page, ""); err != nil {
			return err
		}
		opened, err := s.AgentPreview(ctx, run.Job.ID, "open", nil)
		if err != nil {
			return err
		}
		firstID := opened["build_id"]
		if firstID == nil {
			return fmt.Errorf("open did not build: %+v", opened)
		}
		for i := 1; i <= 12; i++ {
			_, err = s.AgentPreview(ctx, run.Job.ID, "input", []PreviewInput{{Type: "key_press", Key: "a"}, {Type: "wait", Milliseconds: 30}})
			if err != nil {
				return err
			}
			shot, err := s.AgentPreview(ctx, run.Job.ID, "screenshot", nil)
			if err != nil {
				return err
			}
			if shot["build_id"] != firstID {
				return fmt.Errorf("screenshot rebuilt the game")
			}
			captures := s.TakeAgentCaptures(run.Job.ID)
			if len(captures) != 1 || !strings.HasPrefix(captures[0].Image, "data:image/jpeg;base64,") {
				return fmt.Errorf("missing pixels")
			}
		}
		s.previewMu.Lock()
		p := s.agentPreviews[run.Job.ID]
		snapshotDir = p.directory
		observed, err := p.page.Eval(`() => window.count`)
		s.previewMu.Unlock()
		if err != nil {
			return err
		}
		if observed.Value.Int() != 12 {
			return fmt.Errorf("state reset: %v", observed.Value)
		}
		if _, err := s.ReadJobFile(ctx, run.Job.ID, ".aurago/conversation.json"); err == nil {
			return fmt.Errorf("private metadata exposed")
		}
		if _, err := s.AgentCaptureImage(ctx, "another-project", run.Job.ID, "capture"); err == nil {
			return fmt.Errorf("capture escaped its project")
		}
		logs, err := s.AgentPreview(ctx, run.Job.ID, "logs", nil)
		if err != nil {
			return err
		}
		text := fmt.Sprint(logs["diagnostics"])
		if !strings.Contains(text, "first-build-only") || !strings.Contains(text, "promise-feedback") || !strings.Contains(text, "missing.png") {
			return fmt.Errorf("missing browser feedback: %s", text)
		}
		if _, err = s.WriteJobFileChecked(ctx, run.Job.ID, "index.html", "<!doctype html><html><body>Second build</body></html>", ""); err != nil {
			return err
		}
		reloaded, err := s.AgentPreview(ctx, run.Job.ID, "reload", nil)
		if err != nil {
			return err
		}
		if reloaded["build_id"] == firstID {
			return fmt.Errorf("reload reused build ID")
		}
		logs, err = s.AgentPreview(ctx, run.Job.ID, "logs", nil)
		if err != nil {
			return err
		}
		if strings.Contains(fmt.Sprint(logs), "first-build-only") {
			return fmt.Errorf("old errors attributed to new build")
		}
		s.previewMu.Lock()
		p = s.agentPreviews[run.Job.ID]
		_ = p.browser.Close()
		s.previewMu.Unlock()
		if _, err := s.AgentPreview(ctx, run.Job.ID, "screenshot", nil); err == nil {
			return fmt.Errorf("dead browser did not return a tool error")
		}
		if _, err := s.AgentPreview(ctx, run.Job.ID, "open", nil); err != nil {
			return fmt.Errorf("browser retry: %w", err)
		}
		if result := s.FinishJob(ctx, run.Job.ID); !result.OK {
			return fmt.Errorf("finish: %+v", result.Diagnostics)
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "exercise HTML preview"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		current, err := s.GetJob(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !activeJobStatus(current.Status) {
			job = current
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if job.Status != "ready" {
		t.Fatalf("browser job: %+v", job)
	}
	s.closeAgentPreview(job.ID)
	if snapshotDir != "" {
		if _, err := os.Stat(filepath.Clean(snapshotDir)); !os.IsNotExist(err) {
			t.Fatalf("snapshot survived: %v", err)
		}
	}
}

func TestFreeAgentFailuresPreserveRevisionAndDraft(t *testing.T) {
	for _, failure := range []string{"cancel", "budget", "storage", "completion_storage"} {
		t.Run(failure, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
				if r := s.FinishJob(ctx, run.Job.ID); !r.OK {
					return fmt.Errorf("build failed: %+v", r.Diagnostics)
				}
				return nil
			}))
			first, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "initial"})
			if err != nil {
				t.Fatal(err)
			}
			if first = waitJob(t, s, first.ID); first.Status != "ready" {
				t.Fatalf("initial: %+v", first)
			}
			if result := s.BuildJob(context.Background(), first.ID); result.OK {
				t.Fatal("completed job accepted a build")
			}
			s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
				if err := s.WriteJobFile(ctx, run.Job.ID, "index.html", "<!doctype html><p>retained-draft</p>"); err != nil {
					return err
				}
				switch failure {
				case "cancel":
					return s.CancelJob(ctx, run.Job.ID)
				case "budget":
					return context.DeadlineExceeded
				case "storage", "completion_storage":
					trigger := `CREATE TRIGGER fail_publish BEFORE INSERT ON gm_revisions BEGIN SELECT RAISE(ABORT, 'fixture storage failure'); END`
					if failure == "completion_storage" {
						trigger = `CREATE TRIGGER fail_publish BEFORE UPDATE OF status ON gm_jobs WHEN NEW.status='ready' BEGIN SELECT RAISE(ABORT, 'fixture completion failure'); END`
					}
					if _, err := s.db.Exec(trigger); err != nil {
						return err
					}
					if r := s.FinishJob(ctx, run.Job.ID); !r.OK {
						return fmt.Errorf("build failed: %+v", r.Diagnostics)
					}
				}
				return nil
			}))
			failed, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "edit"})
			if err != nil {
				t.Fatal(err)
			}
			failed = waitJob(t, s, failed.ID)
			current, _ := s.GetProject(context.Background(), project.ID)
			if failed.Status == "ready" || current.CurrentRevision != 1 {
				t.Fatalf("failure published: %+v / %+v", failed, current)
			}
			if failure == "storage" || failure == "completion_storage" {
				if _, err := s.db.Exec(`DROP TRIGGER fail_publish`); err != nil {
					t.Fatal(err)
				}
			}
			s.SetRunner(freeAgentRunner(func(ctx context.Context, run JobRun) error {
				content, err := s.ReadJobFile(ctx, run.Job.ID, "index.html")
				if err != nil {
					return err
				}
				if !strings.Contains(content, "retained-draft") {
					return fmt.Errorf("draft was lost")
				}
				if r := s.FinishJob(ctx, run.Job.ID); !r.OK {
					return fmt.Errorf("build failed")
				}
				return nil
			}))
			resumed, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Resume: true})
			if err != nil {
				t.Fatal(err)
			}
			if resumed = waitJob(t, s, resumed.ID); resumed.Status != "ready" || resumed.ResultRevision != 2 {
				t.Fatalf("resume: %+v", resumed)
			}
		})
	}
}

func TestFreeAgentHonorsConfiguredDeadline(t *testing.T) {
	s := newTestService(t)
	s.opts.JobTimeout = 20 * time.Millisecond
	ctx, cancel := s.jobContext(Job{}, Project{})
	defer cancel()
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Fatal(ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("job budget was extended")
	}
}
