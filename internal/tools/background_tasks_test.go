package tools

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testBackgroundTaskLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestBackgroundTaskManagerFollowUpExecutesAndPersists(t *testing.T) {
	dir := t.TempDir()
	mgr := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
	t.Cleanup(func() { _ = mgr.Close() })

	executed := make(chan string, 1)
	mgr.SetLoopbackExecutor(func(prompt string, timeout time.Duration) error {
		executed <- prompt
		return nil
	})

	task, err := mgr.ScheduleFollowUp("continue with diagnostics", BackgroundTaskScheduleOptions{
		Source:      "follow_up",
		Description: "Autonomous follow-up",
	})
	if err != nil {
		t.Fatalf("ScheduleFollowUp: %v", err)
	}

	mgr.processDueTasks()

	select {
	case got := <-executed:
		if got != "continue with diagnostics" {
			t.Fatalf("prompt = %q, want continue with diagnostics", got)
		}
	default:
		t.Fatal("expected follow-up executor to run")
	}

	stored, ok := mgr.GetTask(task.ID)
	if !ok {
		t.Fatalf("task %s not found after execution", task.ID)
	}
	if stored.Status != BackgroundTaskStatusCompleted {
		t.Fatalf("task status = %q, want %q", stored.Status, BackgroundTaskStatusCompleted)
	}

	if _, err := os.Stat(filepath.Join(dir, systemTaskStoreFile)); err != nil {
		t.Fatalf("expected persisted system task sqlite store: %v", err)
	}
	var tasks []*BackgroundTask
	loaded, err := mgr.store.load(systemTaskNamespaceBackground, &tasks)
	if err != nil {
		t.Fatalf("load persisted background tasks: %v", err)
	}
	if !loaded || len(tasks) == 0 {
		t.Fatal("expected background tasks to be persisted in sqlite store")
	}
}

func TestBackgroundTaskManagerWaitForEventFileChanged(t *testing.T) {
	dir := t.TempDir()
	logger := testBackgroundTaskLogger()
	mgr := NewBackgroundTaskManager(dir, logger)
	t.Cleanup(func() { _ = mgr.Close() })

	target := filepath.Join(dir, "watch.txt")
	if err := os.WriteFile(target, []byte("before"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}

	executed := make(chan string, 1)
	mgr.SetLoopbackExecutor(func(prompt string, timeout time.Duration) error {
		executed <- prompt
		return nil
	})

	task, err := mgr.ScheduleWaitForEvent(WaitForEventTaskPayload{
		EventType:           "file_changed",
		TaskPrompt:          "inspect the changed file",
		FilePath:            target,
		PollIntervalSeconds: 1,
		TimeoutSeconds:      30,
	}, BackgroundTaskScheduleOptions{
		Source:      "wait_for_event",
		Description: "Wait for file change",
	})
	if err != nil {
		t.Fatalf("ScheduleWaitForEvent: %v", err)
	}

	mgr.processDueTasks()
	waiting, ok := mgr.GetTask(task.ID)
	if !ok {
		t.Fatalf("task %s missing after first check", task.ID)
	}
	if waiting.Status != BackgroundTaskStatusWaiting {
		t.Fatalf("first status = %q, want %q", waiting.Status, BackgroundTaskStatusWaiting)
	}

	time.Sleep(2 * time.Millisecond)
	if err := os.WriteFile(target, []byte("after"), 0o644); err != nil {
		t.Fatalf("write updated file: %v", err)
	}

	mgr.mu.Lock()
	if internal, ok := mgr.tasks[task.ID]; ok {
		internal.NextAttemptAt = time.Now().UTC()
	}
	mgr.mu.Unlock()
	mgr.processDueTasks()

	select {
	case got := <-executed:
		if got == "" {
			t.Fatal("expected non-empty follow-up prompt after file change")
		}
	default:
		t.Fatal("expected wait_for_event continuation to execute")
	}

	completed, ok := mgr.GetTask(task.ID)
	if !ok {
		t.Fatalf("task %s missing after completion", task.ID)
	}
	if completed.Status != BackgroundTaskStatusCompleted {
		t.Fatalf("final status = %q, want %q", completed.Status, BackgroundTaskStatusCompleted)
	}
}

func TestBackgroundTaskManagerWaitForEventWithoutPromptCompletes(t *testing.T) {
	dir := t.TempDir()
	mgr := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
	t.Cleanup(func() { _ = mgr.Close() })

	target := filepath.Join(dir, "watch-no-prompt.txt")
	task, err := mgr.ScheduleWaitForEvent(WaitForEventTaskPayload{
		EventType:           "file_changed",
		FilePath:            target,
		PollIntervalSeconds: 1,
		TimeoutSeconds:      30,
	}, BackgroundTaskScheduleOptions{Source: "wait_for_event", Description: "Wait without follow-up"})
	if err != nil {
		t.Fatalf("ScheduleWaitForEvent: %v", err)
	}

	if err := os.WriteFile(target, []byte("created"), 0o644); err != nil {
		t.Fatalf("write watched file: %v", err)
	}
	mgr.processDueTasks()

	completed, ok := mgr.GetTask(task.ID)
	if !ok {
		t.Fatalf("task %s missing after completion", task.ID)
	}
	if completed.Status != BackgroundTaskStatusCompleted {
		t.Fatalf("final status = %q, want %q", completed.Status, BackgroundTaskStatusCompleted)
	}
}

func TestBackgroundTaskManagerCheckWaitConditionReadsRegistryWithoutManagerLock(t *testing.T) {
	mgr := NewBackgroundTaskManager(t.TempDir(), testBackgroundTaskLogger())
	t.Cleanup(func() { _ = mgr.Close() })
	registry := NewProcessRegistry(testBackgroundTaskLogger())
	mgr.SetProcessRegistry(registry)

	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	resultCh := make(chan struct {
		met bool
		err error
	}, 1)
	go func() {
		met, _, err := mgr.checkWaitCondition(WaitForEventTaskPayload{EventType: "process_exited", PID: 1234})
		resultCh <- struct {
			met bool
			err error
		}{met: met, err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("unexpected error: %v", result.err)
		}
		if !result.met {
			t.Fatal("expected missing process to be treated as exited")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected checkWaitCondition to read process registry without waiting on manager mutex")
	}
}
