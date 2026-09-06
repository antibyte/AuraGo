package gamemaker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPreviewValidationRejectsStaleReportsAndReadyBeforeError(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "current", JobID: "job"}
	s.previewCheck = check
	s.activeJobID = "job"
	s.tokens["current-token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "current", ExpiresAt: time.Now().Add(time.Minute)}
	s.tokens["old-token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "old", ExpiresAt: time.Now().Add(time.Minute)}
	if err := s.ReportPreview("another-project", PreviewReport{Token: "current-token", Type: "ready"}); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("cross-project report: %v", err)
	}
	for _, kind := range []string{"ready", "runtime_error"} {
		if err := s.ReportPreview("project", PreviewReport{Token: "old-token", Type: kind, Message: "old error"}); err != nil {
			t.Fatal(err)
		}
	}
	if !check.ReadyAt.IsZero() || len(check.Diagnostics) != 0 {
		t.Fatal("stale iframe affected the current build")
	}
	if got := s.waitForPreview(context.Background(), check, time.Millisecond); got.OK || got.RuntimeStatus != "unavailable" {
		t.Fatalf("missing browser passed: %+v", got)
	}
	_ = s.ReportPreview("project", PreviewReport{Token: "current-token", Type: "ready"})
	if !check.ReadyAt.IsZero() {
		t.Fatal("game-authored readiness without a visible canvas passed validation")
	}
	_ = s.ReportPreview("project", PreviewReport{Token: "current-token", Type: "ready", CanvasVisible: true})
	const message = "Uncaught TypeError: this.scale.setSize is not a function"
	for range 30 {
		_ = s.ReportPreview("project", PreviewReport{Token: "current-token", Type: "runtime_error", Message: message})
	}
	if got := s.waitForPreview(context.Background(), check, time.Second); got.OK || got.RuntimeStatus != "failed" || len(got.Diagnostics) != 1 || got.Diagnostics[0].Message != message {
		t.Fatalf("ready suppressed runtime error or duplicates were retained: %+v", got)
	}
	if got := boundedPreviewDiagnostics([]Diagnostic{{Message: strings.Repeat("ö", 1001)}}); len([]rune(got[0].Message)) != 1000 {
		t.Fatal("diagnostics are not bounded as valid UTF-8")
	}
}

func TestPreviewBootObservesErrorsBeforeGameScripts(t *testing.T) {
	html := string(injectPreviewBoot([]byte(`<html><head><script src="game.js"></script></head><body></body></html>`)))
	if strings.Index(html, previewBootMarker) > strings.Index(html, `src="game.js"`) {
		t.Fatal("runtime listeners were injected after game code")
	}
	for _, marker := range []string{`window.addEventListener("error"`, `window.addEventListener("unhandledrejection"`, `"resource_error"`} {
		if !strings.Contains(html, marker) {
			t.Fatalf("missing browser diagnostic capture: %s", marker)
		}
	}
}

func TestPreviewValidationWaitsBeyondFirstSpawnTimer(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "spawn", JobID: "job", ReadyAt: time.Now().Add(-1500 * time.Millisecond)}
	s.previewCheck = check
	s.activeJobID = "job"
	s.tokens["spawn-token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: check.ID, ExpiresAt: time.Now().Add(time.Minute)}
	if got := s.waitForPreview(context.Background(), check, 30*time.Millisecond); got.OK {
		t.Fatal("startup passed before delayed spawning could be checked")
	}
	const message = "Uncaught TypeError: Phaser.Math.pick is not a function"
	if err := s.ReportPreview("project", PreviewReport{Token: "spawn-token", Type: "runtime_error", Message: message}); err != nil {
		t.Fatal(err)
	}
	if got := s.waitForPreview(context.Background(), check, time.Second); got.OK || got.RuntimeStatus != "failed" || got.Diagnostics[0].Message != message {
		t.Fatalf("delayed spawn error was lost: %+v", got)
	}
}

func TestRuntimeErrorReachesRepairRunnerBeforePublication(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	const runtimeError = "Uncaught TypeError: this.scale.setSize is not a function"
	runs := 0
	s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
		runs++
		if runs == 1 {
			if len(run.Diagnostics) != 1 || run.Diagnostics[0].Message != "previous gameplay error" {
				t.Errorf("change request lost preview diagnostics: %+v", run.Diagnostics)
			}
			return nil
		}
		if len(run.Diagnostics) != 1 || run.Diagnostics[0].Message != runtimeError || !strings.Contains(run.Job.Prompt, "Keep the snake controls") {
			t.Errorf("repair lost diagnostics or original request: %+v", run)
		}
		current, _ := s.GetProject(ctx, project.ID)
		if current.CurrentRevision != 0 {
			t.Error("broken build was published before repair")
		}
		return nil
	}})
	events, unsubscribe := s.Subscribe(project.ID)
	defer unsubscribe()
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Keep the snake controls", PreviewDiagnostics: []Diagnostic{{Message: "previous gameplay error"}}})
	if err != nil {
		t.Fatal(err)
	}
	// Inject the screenshot's error through the same grant-bound report path
	// used by the Studio, immediately after the first compiled preview.
	for {
		select {
		case event := <-events:
			if event.Type != "preview_reload" {
				continue
			}
			grant, err := s.CreatePreviewGrant(project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.ReportPreview(project.ID, PreviewReport{Token: grant.Token, Type: "runtime_error", Message: runtimeError}); err != nil {
				t.Fatal(err)
			}
			finished := waitJob(t, s, job.ID)
			if finished.Status != "ready" || runs != 2 {
				t.Fatalf("repair did not complete: job=%+v runs=%d", finished, runs)
			}
			return
		case <-time.After(5 * time.Second):
			t.Fatal("no compiled preview received")
		}
	}
}
