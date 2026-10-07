package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

const lifecycleProjectID = "123e4567-e89b-12d3-a456-426614174000"

func newLifecycleVideoStudioManager(t *testing.T, srv *Server, dataDir string) *videoStudioManager {
	t.Helper()
	if srv == nil {
		srv = &Server{Cfg: &config.Config{}}
	}
	manager, err := newVideoStudioManager(srv, dataDir)
	if err != nil {
		t.Fatalf("newVideoStudioManager: %v", err)
	}
	return manager
}

func TestVideoStudioManagerRestartMarksJobsInterruptedAndPreservesUncertainty(t *testing.T) {
	dataDir := t.TempDir()
	manager := newLifecycleVideoStudioManager(t, nil, dataDir)
	t.Cleanup(manager.close)

	queued, _, err := manager.enqueue(lifecycleProjectID, "render", "render-fingerprint", "queued-key", nil)
	if err != nil {
		t.Fatalf("enqueue queued job: %v", err)
	}
	running, _, err := manager.enqueue(lifecycleProjectID, "generate", "generate-fingerprint", "generate-key", nil)
	if err != nil {
		t.Fatalf("enqueue running job: %v", err)
	}

	manager.mu.Lock()
	manager.jobs[running.ID].Status = "running"
	manager.jobs[running.ID].ExternalStatusUnknown = true
	started := time.Now().UTC()
	manager.jobs[running.ID].StartedAt = &started
	if err := manager.persistLocked(); err != nil {
		manager.mu.Unlock()
		t.Fatalf("persist simulated pre-crash state: %v", err)
	}
	manager.mu.Unlock()

	// Stop only the worker context to model an abrupt process exit; close() would
	// correctly mark jobs as cancelled during an orderly shutdown instead.
	manager.cancel()
	manager.wg.Wait()

	restarted := newLifecycleVideoStudioManager(t, nil, dataDir)
	t.Cleanup(restarted.close)

	queuedAfterRestart := restarted.job(queued.ID)
	if queuedAfterRestart == nil || queuedAfterRestart.Status != "interrupted" {
		t.Fatalf("queued job after restart = %#v, want interrupted", queuedAfterRestart)
	}
	if queuedAfterRestart.Error != "interrupted" || !strings.Contains(queuedAfterRestart.Message, "Start a new job") {
		t.Fatalf("queued restart explanation = (%q, %q), want generic interrupted guidance", queuedAfterRestart.Error, queuedAfterRestart.Message)
	}
	runningAfterRestart := restarted.job(running.ID)
	if runningAfterRestart == nil || runningAfterRestart.Status != "interrupted" || !runningAfterRestart.ExternalStatusUnknown {
		t.Fatalf("uncertain generation after restart = %#v, want interrupted and uncertain", runningAfterRestart)
	}
	if !strings.Contains(runningAfterRestart.Message, "Check the provider") || !strings.Contains(runningAfterRestart.Message, "paid request") {
		t.Fatalf("uncertain generation message = %q, want provider-check guidance", runningAfterRestart.Message)
	}
}

func TestVideoStudioManagerIdempotencyDeduplicatesAndRejectsChangedPayload(t *testing.T) {
	dataDir := t.TempDir()
	manager := newLifecycleVideoStudioManager(t, nil, dataDir)
	t.Cleanup(manager.close)

	first, duplicate, err := manager.enqueue(lifecycleProjectID, "render", "same-fingerprint", "stable-key", nil)
	if err != nil || duplicate {
		t.Fatalf("first enqueue = (%#v, duplicate=%v, err=%v), want a new job", first, duplicate, err)
	}
	manager.cancel()
	manager.wg.Wait()

	restarted := newLifecycleVideoStudioManager(t, nil, dataDir)
	t.Cleanup(restarted.close)
	second, duplicate, err := restarted.enqueue(lifecycleProjectID, "render", "same-fingerprint", "stable-key", nil)
	if err != nil || !duplicate {
		t.Fatalf("repeated enqueue = (%#v, duplicate=%v, err=%v), want deduplicated job", second, duplicate, err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate job id = %q, want original %q", second.ID, first.ID)
	}
	if _, duplicate, err := restarted.enqueue(lifecycleProjectID, "render", "changed-fingerprint", "stable-key", nil); err == nil || duplicate || !strings.Contains(err.Error(), "idempotency_conflict") {
		t.Fatalf("changed payload enqueue = (duplicate=%v, err=%v), want idempotency conflict", duplicate, err)
	}
}

func TestVideoStudioManagerCancellationClosesPublicationGate(t *testing.T) {
	cfg := &config.Config{}
	cfg.VideoStudio.Enabled = true
	cfg.VirtualDesktop.Enabled = true
	srv := &Server{Cfg: cfg}
	manager := newLifecycleVideoStudioManager(t, srv, t.TempDir())
	t.Cleanup(manager.close)

	job, _, err := manager.enqueue(lifecycleProjectID, "render", "fingerprint", "key", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	manager.mu.Lock()
	manager.jobs[job.ID].Status = "running"
	manager.mu.Unlock()
	ctx := manager.publicationContext(context.Background(), job.ID)

	if !manager.cancelJob(job.ID) {
		t.Fatal("cancelJob returned false for running job")
	}
	committed := false
	err = fileutil.PublishContext(ctx, func() error {
		committed = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PublishContext error = %v, want context.Canceled", err)
	}
	if committed {
		t.Fatal("publication commit ran after cancellation")
	}
}

func TestVideoStudioJobFailureClassification(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		err      error
		ctxErr   error
		status   string
		code     string
		contains string
	}{
		{name: "probe error", kind: "probe", err: errors.New("bad media"), status: "failed", code: "probe_failed", contains: "Media inspection"},
		{name: "preview error", kind: "preview", err: errors.New("encoder failure"), status: "failed", code: "preview_failed", contains: "preview"},
		{name: "render error", kind: "render", err: errors.New("encoder failure"), status: "failed", code: "render_failed", contains: "rendered"},
		{name: "generation error", kind: "generate", err: errors.New("provider error"), status: "failed", code: "generation_failed", contains: "provider"},
		{name: "cancelled", kind: "render", err: context.Canceled, status: "cancelled", code: "cancelled", contains: "cancelled"},
		{name: "deadline", kind: "render", ctxErr: context.DeadlineExceeded, status: "failed", code: "timeout", contains: "time limit"},
		{name: "unknown kind", kind: "other", err: errors.New("failure"), status: "failed", code: "job_failed", contains: "job failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctxErr != nil {
				if errors.Is(tt.ctxErr, context.DeadlineExceeded) {
					deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
					ctx = deadlineCtx
					defer cancel()
				} else {
					cancelCtx, cancel := context.WithCancel(ctx)
					ctx = cancelCtx
					cancel()
					defer cancel()
				}
			}
			status, code, message := videoStudioJobFailure(tt.kind, tt.err, ctx)
			if status != tt.status || code != tt.code || !strings.Contains(message, tt.contains) {
				t.Fatalf("videoStudioJobFailure = (%q, %q, %q), want (%q, %q, message containing %q)", status, code, message, tt.status, tt.code, tt.contains)
			}
		})
	}
}

func TestVideoStudioManagerShutdownStopsQueuedJobsAndRejectsEnqueue(t *testing.T) {
	manager := newLifecycleVideoStudioManager(t, nil, t.TempDir())
	job, _, err := manager.enqueue(lifecycleProjectID, "render", "fingerprint", "first-key", nil)
	if err != nil {
		t.Fatalf("enqueue before shutdown: %v", err)
	}

	manager.close()
	if got := manager.job(job.ID); got == nil || got.Status != "cancelled" || got.Error != "server_shutdown" {
		t.Fatalf("job after shutdown = %#v, want server_shutdown cancellation", got)
	}
	if _, duplicate, err := manager.enqueue(lifecycleProjectID, "render", "other", "second-key", nil); err == nil || duplicate || !strings.Contains(err.Error(), "video_studio_shutdown") {
		t.Fatalf("enqueue after shutdown = (duplicate=%v, err=%v), want shutdown rejection", duplicate, err)
	}
}
