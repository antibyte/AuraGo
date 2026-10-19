package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackgroundAdmissionBoundBatchAndPlannerDedupe(t *testing.T) {
	mgr := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
	defer mgr.Close()
	mgr.SetLimits(BackgroundTaskLimits{MaxActiveTasks: 3, MaxDueStartsPerTick: 2})
	opts := BackgroundTaskScheduleOptions{}
	first, err := mgr.SchedulePlannerPrompt("revision-1", "reminder", opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := mgr.SchedulePlannerPrompt("revision-1", "reminder", opts)
	if err != nil || first.ID != second.ID {
		t.Fatalf("dedupe %v %v", second, err)
	}
	for i := 0; i < 2; i++ {
		if _, err := mgr.ScheduleFollowUp("next", opts); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := mgr.ScheduleFollowUp("beyond cap", opts); err == nil {
		t.Fatal("active capacity exceeded")
	}
	claimed := mgr.claimDueTasks(time.Now().Add(time.Minute))
	if len(claimed) != 2 {
		t.Fatalf("claimed %d, want 2", len(claimed))
	}
	if mgr.Summary().Queued != 1 {
		t.Fatal("unclaimed due task disappeared")
	}
}

func TestBackgroundClaimFailureNeverExecutesAndRetentionKeepsActive(t *testing.T) {
	mgr := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
	defer mgr.Close()
	task, err := mgr.ScheduleFollowUp("must not dispatch", BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mgr.mu.Lock()
	originalStore := mgr.store
	defer originalStore.release()
	mgr.store = nil
	mgr.file = filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(mgr.file, 0700); err != nil {
		t.Fatal(err)
	}
	mgr.mu.Unlock()
	if claimed := mgr.claimDueTasks(time.Now().Add(time.Minute)); len(claimed) != 0 {
		t.Fatal("unpersisted claim was returned")
	}
	stored, _ := mgr.GetTask(task.ID)
	if stored.Status != BackgroundTaskStatusQueued {
		t.Fatal("failed claim was not rolled back")
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	now := time.Now().UTC()
	mgr.limits = BackgroundTaskLimits{MaxTerminalTasks: 1}
	mgr.tasks["old-active"] = &BackgroundTask{ID: "old-active", Status: BackgroundTaskStatusWaiting, UpdatedAt: now.AddDate(-1, 0, 0)}
	mgr.tasks["expired"] = &BackgroundTask{ID: "expired", Status: BackgroundTaskStatusCompleted, UpdatedAt: now.AddDate(0, 0, -31)}
	mgr.tasks["recent-failed"] = &BackgroundTask{ID: "recent-failed", Status: BackgroundTaskStatusFailed, UpdatedAt: now.AddDate(0, 0, -31)}
	mgr.tasks["recent-complete"] = &BackgroundTask{ID: "recent-complete", Status: BackgroundTaskStatusCompleted, UpdatedAt: now}
	mgr.pruneTerminalLocked(now)
	if mgr.tasks["old-active"] == nil || mgr.tasks[task.ID] == nil {
		t.Fatal("active task pruned")
	}
	if mgr.tasks["expired"] != nil || mgr.tasks["recent-failed"] != nil || mgr.tasks["recent-complete"] == nil {
		t.Fatal("terminal retention/cap incorrect")
	}
}

func TestBackgroundCancelPersistenceFailureStopsOwnerAndReportsFailure(t *testing.T) {
	mgr := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
	originalStore, originalFile := mgr.store, mgr.file
	t.Cleanup(func() {
		mgr.mu.Lock()
		if mgr.store == nil {
			mgr.store = originalStore
		}
		mgr.mu.Unlock()
		_ = mgr.Close()
	})
	task, err := mgr.ScheduleFollowUp("cancel before execution", BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(mgr.claimDueTasks(time.Now().Add(time.Minute))) != 1 {
		t.Fatal("task was not claimed")
	}
	owner, ok := mgr.promptTaskContext(task.ID)
	if !ok {
		t.Fatal("task owner missing")
	}
	blockedPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(blockedPath, 0700); err != nil {
		t.Fatal(err)
	}
	mgr.mu.Lock()
	mgr.store, mgr.file = nil, blockedPath
	mgr.mu.Unlock()
	canceled, err := mgr.CancelTaskWithError(task.ID)
	if !canceled || err == nil {
		t.Fatalf("cancel accepted=%v error=%v", canceled, err)
	}
	if owner.Err() != context.Canceled {
		t.Fatal("persistence failure left original owner running")
	}
	stored, _ := mgr.GetTask(task.ID)
	if stored.Status != BackgroundTaskStatusCanceled || stored.LastError != backgroundCancelPersistenceFailed {
		t.Fatalf("cancel failure was not visible: %+v", stored)
	}
	if mgr.CancelTask(task.ID) {
		t.Fatal("legacy wrapper reported durable success on persistence failure")
	}
	mgr.mu.Lock()
	mgr.store, mgr.file = originalStore, originalFile
	mgr.mu.Unlock()
	if canceled, err := mgr.CancelTaskWithError(task.ID); !canceled || err != nil {
		t.Fatalf("cancel persistence could not be retried: %v %v", canceled, err)
	}
	var persisted []*BackgroundTask
	if loaded, err := originalStore.load(systemTaskNamespaceBackground, &persisted); !loaded || err != nil {
		t.Fatalf("read persisted cancel: loaded=%v error=%v", loaded, err)
	}
	if len(persisted) != 1 || persisted[0].Status != BackgroundTaskStatusCanceled || persisted[0].LastError != "" {
		t.Fatalf("cancel was not saved after retry: %+v", persisted)
	}
}
