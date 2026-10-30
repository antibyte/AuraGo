package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackgroundWaitEventFreezesPromptWhileExecutionIsPending(t *testing.T) {
	m := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
	t.Cleanup(func() { _ = m.Close() })
	path := filepath.Join(t.TempDir(), "event.txt")
	if err := os.WriteFile(path, []byte("initial"), 0600); err != nil {
		t.Fatal(err)
	}
	task, err := m.ScheduleWaitForEvent(WaitForEventTaskPayload{EventType: "file_changed", FilePath: path, TaskPrompt: "inspect change"}, BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("first event"), 0600); err != nil {
		t.Fatal(err)
	}
	var prompts, identities []string
	m.SetLoopbackExecutor(func(id, kind, prompt string, timeout time.Duration) error {
		prompts = append(prompts, prompt)
		identities = append(identities, id)
		if len(prompts) == 1 {
			return ErrBackgroundTaskPending
		}
		return nil
	})
	m.processDueTasks()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.tasks[task.ID].NextAttemptAt = time.Now().Add(-time.Second)
	m.mu.Unlock()
	m.processDueTasks()
	done, _ := m.GetTask(task.ID)
	if done.Status != BackgroundTaskStatusCompleted || len(prompts) != 2 || prompts[0] != prompts[1] || identities[0] != identities[1] {
		t.Fatalf("event was re-evaluated: status=%s attempts=%d", done.Status, len(prompts))
	}
}

func TestBackgroundPendingKeepsExecutionIdentityWithoutRetry(t *testing.T) {
	m := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
	t.Cleanup(func() { _ = m.Close() })
	var identities []string
	m.SetLoopbackExecutor(func(id, kind, prompt string, timeout time.Duration) error {
		identities = append(identities, id)
		if kind != BackgroundTaskTypeCronPrompt {
			t.Errorf("type = %s", kind)
		}
		if len(identities) == 1 {
			return ErrBackgroundTaskPending
		}
		return nil
	})
	task, err := m.ScheduleCronPrompt("scheduled task", BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m.processDueTasks()
	waiting, _ := m.GetTask(task.ID)
	if waiting.Status != BackgroundTaskStatusWaiting || waiting.RetryCount != 0 {
		t.Fatalf("pending state: %+v", waiting)
	}
	m.mu.Lock()
	m.tasks[task.ID].NextAttemptAt = time.Now().Add(-time.Second)
	m.mu.Unlock()
	m.processDueTasks()
	done, _ := m.GetTask(task.ID)
	if done.Status != BackgroundTaskStatusCompleted || len(identities) != 2 || identities[0] == "" || identities[0] != identities[1] {
		t.Fatalf("unexpected completion: %+v IDs=%v", done, identities)
	}
}

func TestBackgroundRestartRequiresExplicitRetryForUncertainExecution(t *testing.T) {
	dir := tempSystemTaskDir(t)
	m := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
	m.SetLoopbackExecutor(func(string, string, string, time.Duration) error { return ErrBackgroundTaskPending })
	task, err := m.ScheduleCronPrompt("scheduled task", BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m.processDueTasks()
	before, _ := m.GetTask(task.ID)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
	t.Cleanup(func() { _ = restarted.Close() })
	after, _ := restarted.GetTask(task.ID)
	if after.Status != BackgroundTaskStatusFailed || after.ExecutionID != before.ExecutionID {
		t.Fatalf("unsafe restart: %+v", after)
	}
	if !restarted.RetryTask(task.ID) {
		t.Fatal("explicit retry rejected")
	}
	restarted.SetLoopbackExecutor(func(id, kind, prompt string, timeout time.Duration) error {
		if id == before.ExecutionID {
			t.Error("explicit retry reused previous execution")
		}
		return nil
	})
	restarted.processDueTasks()
	after, _ = restarted.GetTask(task.ID)
	if after.Status != BackgroundTaskStatusCompleted {
		t.Fatalf("explicit retry: %+v", after)
	}
}

func TestBackgroundPromptOwnerSurvivesPendingUntilCancelOrCompletion(t *testing.T) {
	for _, cancelTask := range []bool{true, false} {
		m := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
		var owners []context.Context
		var executionID string
		m.SetLoopbackExecutorContext(func(ctx context.Context, id, _, _ string, _ time.Duration) error {
			owners = append(owners, ctx)
			executionID = id
			if len(owners) == 1 {
				return ErrBackgroundTaskPending
			}
			return nil
		})
		task, err := m.ScheduleFollowUp("local pending owner", BackgroundTaskScheduleOptions{})
		if err != nil {
			t.Fatal(err)
		}
		m.processDueTasks()
		if len(owners) != 1 || owners[0].Err() != nil {
			t.Fatal("202 ended task owner")
		}
		if owner, ok := m.ExecutionContext(executionID); !ok || owner != owners[0] {
			t.Fatal("execution lost its original owner")
		}
		if _, ok := m.ExecutionContext("forged-execution"); ok {
			t.Fatal("unowned execution ID accepted")
		}
		if cancelTask {
			if !m.CancelTask(task.ID) {
				t.Fatal("cancel failed")
			}
		} else {
			m.mu.Lock()
			m.tasks[task.ID].NextAttemptAt = time.Now().Add(-time.Second)
			m.mu.Unlock()
			m.processDueTasks()
			if len(owners) != 2 || owners[0] != owners[1] {
				t.Fatal("poll replaced task owner")
			}
		}
		if owners[0].Err() != context.Canceled {
			t.Fatal("terminal task retained a live owner")
		}
		if _, ok := m.ExecutionContext(executionID); ok {
			t.Fatal("terminal task remained admissible")
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// A task queued from a tainted session hands the flag to the loopback executor;
// a clean task does not, and the flag survives a restart of the manager.
func TestBackgroundPromptCarriesUntrustedProvenance(t *testing.T) {
	for _, untrusted := range []bool{true, false} {
		dir := tempSystemTaskDir(t)
		m := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
		task, err := m.ScheduleFollowUp("check the report", BackgroundTaskScheduleOptions{UntrustedProvenance: untrusted})
		if err != nil {
			t.Fatal(err)
		}
		_ = m.Close()
		restarted := NewBackgroundTaskManager(dir, testBackgroundTaskLogger())
		t.Cleanup(func() { _ = restarted.Close() })
		if stored, ok := restarted.GetTask(task.ID); !ok || stored.UntrustedProvenance != untrusted {
			t.Fatalf("flag lost across restart: %+v", stored)
		}
		var got []bool
		restarted.SetLoopbackExecutorContext(func(ctx context.Context, _, _, _ string, _ time.Duration) error {
			got = append(got, UntrustedProvenanceFromContext(ctx))
			return nil
		})
		restarted.processDueTasks()
		if len(got) != 1 || got[0] != untrusted {
			t.Fatalf("executor saw %v, want [%v]", got, untrusted)
		}
	}
}
