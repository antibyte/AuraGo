package tools

import (
	"context"
	"testing"
	"time"
)

func TestBackgroundUnknownAndPolicyBlockedDoNotAutomaticallyRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status string
	}{
		{"unknown", ErrBackgroundTaskOutcomeUnknown, BackgroundTaskStatusFailed},
		{"policy", ErrBackgroundTaskBlocked, BackgroundTaskStatusBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
			defer mgr.Close()
			calls := 0
			mgr.SetLoopbackExecutor(func(string, string, string, time.Duration) error { calls++; return tc.err })
			task, err := mgr.ScheduleFollowUp("guarded task", BackgroundTaskScheduleOptions{MaxRetries: 5})
			if err != nil {
				t.Fatal(err)
			}
			mgr.processDueTasks()
			mgr.processDueTasks()
			stored, _ := mgr.GetTask(task.ID)
			if calls != 1 || stored.Status != tc.status || stored.RetryCount != 0 {
				t.Fatalf("calls=%d status=%s retries=%d", calls, stored.Status, stored.RetryCount)
			}
		})
	}
}

func TestIsolatedBackgroundUsesScopedPermissionsAndTaskCancellation(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{AllowPython: true, AllowFilesystemWrite: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	mgr := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
	defer mgr.Close()
	started := make(chan RuntimePermissions, 1)
	mgr.SetIsolatedCodeAuthorizer(func(context.Context, BackgroundCodeAction) error { return nil })
	mgr.SetIsolatedCodeExecutor(func(ctx context.Context, _, _ string, _ time.Duration) (string, error) {
		perms, _ := EffectiveRuntimePermissions(ctx)
		started <- perms
		<-ctx.Done()
		return "late result", nil
	})
	owner := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowPython: true, AllowFilesystemWrite: false})
	owner = WithBackgroundCodeAction(owner, BackgroundCodeAction{Call: []byte(`{"action":"execute_python","code":"print('local fixture')"}`), SessionID: "isolated-fixture"})
	task, err := mgr.ScheduleIsolatedCodeContext(owner, "print('local fixture')", "python", BackgroundTaskScheduleOptions{ExecutionKind: "python"})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); mgr.processDueTasks() }()
	select {
	case perms := <-started:
		if !perms.AllowPython || perms.AllowFilesystemWrite {
			t.Fatal("scoped permissions were widened")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("isolated executor did not start")
	}
	if !mgr.CancelTask(task.ID) {
		t.Fatal("running task could not be canceled")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("task cancellation did not reach isolated executor")
	}
	stored, _ := mgr.GetTask(task.ID)
	if stored.Status != BackgroundTaskStatusCanceled {
		t.Fatalf("late success overwrote cancellation: %s", stored.Status)
	}
}
