package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// A stored job whose id is not in the uid format is dropped on load. Trimming deletes previews/<project>/<id>.mp4, so
// an id such as "../../victim" would name a file outside the preview folder. The sentinel sits where that path leads
// (video_studio/victim.mp4) and the oldest stored jobs are the malformed ones, which trimming removes first.
func TestVideoStudioManagerLoadDropsJobsWithMalformedIDs(t *testing.T) {
	dataDir := t.TempDir()
	root := filepath.Join(dataDir, "video_studio")
	if err := os.MkdirAll(filepath.Join(root, "previews"), 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "victim.mp4")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldest := time.Now().Add(-2 * time.Hour).UTC()
	good := &videoStudioJob{ID: uidNew(), ProjectID: lifecycleProjectID, Kind: "range_render", Status: "succeeded", CreatedAt: oldest.Add(time.Hour)}
	state := videoStudioDiskState{Version: 1, Jobs: []*videoStudioJob{good}}
	malformed := []string{"../../victim", "victim", strings.ToUpper(lifecycleProjectID), good.ID + "/.."}
	for i, id := range malformed {
		state.Jobs = append(state.Jobs, &videoStudioJob{ID: id, ProjectID: lifecycleProjectID, Kind: "range_render", Status: "succeeded", CreatedAt: oldest.Add(time.Duration(i) * time.Minute)})
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "jobs.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	manager := newLifecycleVideoStudioManager(t, nil, dataDir)
	t.Cleanup(manager.close)
	if manager.job(good.ID) == nil {
		t.Fatalf("the job with the well-formed id %s was not loaded", good.ID)
	}
	for _, id := range malformed {
		if manager.job(id) != nil {
			t.Errorf("a stored job with the malformed id %q was loaded", id)
		}
	}
	manager.mu.Lock()
	for i := 0; i < videoStudioJobRetention-1; i++ {
		filler := &videoStudioJob{ID: uidNew(), ProjectID: lifecycleProjectID, Kind: "probe", Status: "succeeded", CreatedAt: time.Now().UTC()}
		manager.jobs[filler.ID] = filler
	}
	manager.trimJobsLocked()
	manager.mu.Unlock()
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("trimming reached the file outside the preview folder: %v", err)
	}
	persisted, err := os.ReadFile(filepath.Join(root, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "victim") {
		t.Errorf("the rewritten job state still holds a malformed id: %s", persisted)
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

func TestVideoStudioProjectLocksAreBoundedAndSerializeSameID(t *testing.T) {
	manager := &videoStudioManager{}
	first := manager.projectLock(lifecycleProjectID)
	second := manager.projectLock(lifecycleProjectID)
	if first != second {
		t.Fatal("same project ID did not resolve to the same lock")
	}
	first.Lock()
	acquired := make(chan struct{})
	go func() {
		second.Lock()
		close(acquired)
		second.Unlock()
	}()
	if second.TryLock() {
		second.Unlock()
		first.Unlock()
		t.Fatal("same-project lock allowed concurrent entry")
	}
	first.Unlock()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("same-project waiter did not acquire the lock after release")
	}

	locks := make(map[*sync.Mutex]struct{})
	for i := 1; i <= 4096; i++ {
		projectID := fmt.Sprintf("%08x-e89b-12d3-a456-%012x", i, i)
		locks[manager.projectLock(projectID)] = struct{}{}
	}
	if got := len(locks); got > videoStudioProjectLockStripeCount {
		t.Fatalf("4096 project IDs resolved to %d locks, want at most %d", got, videoStudioProjectLockStripeCount)
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
		{name: "generation project quota", kind: "generate", err: errVideoStudioProjectSizeLimit, status: "failed", code: "project_size_limit", contains: "storage limit"},
		{name: "generation asset quota", kind: "generate", err: errVideoStudioAssetSizeLimit, status: "failed", code: "asset_size_limit", contains: "per-asset size limit"},
		{name: "generation local import", kind: "generate", err: errVideoStudioGenerationImport, status: "failed", code: "generation_import_failed", contains: "could not import"},
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
