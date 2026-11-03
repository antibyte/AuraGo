package tools

import (
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestBackgroundTaskManagerProcessExitedScrubsBeforeTailAndPersistence(t *testing.T) {
	logSecret := "process-tail-boundary-secret-" + strings.Repeat("x", 2100)
	errorSecret := "process-error-secret-sentinel"
	releaseLog := security.RegisterScopedSensitiveExact(logSecret)
	defer releaseLog()
	releaseError := security.RegisterScopedSensitiveExact(errorSecret)
	defer releaseError()
	for _, prompt := range []string{"", "summarize the process result"} {
		t.Run(prompt, func(t *testing.T) {
			mgr := NewBackgroundTaskManager(tempSystemTaskDir(t), testBackgroundTaskLogger())
			t.Cleanup(func() { _ = mgr.Close() })
			registry := NewProcessRegistry(testBackgroundTaskLogger())
			info := &ProcessInfo{PID: 9876, State: ProcessStateCrashed, ExitCode: 2, ErrorReason: "failure " + errorSecret}
			_, _ = info.Write([]byte("build output " + logSecret + "\ncompiler error: missing header"))
			registry.Register(info)
			mgr.SetProcessRegistry(registry)
			var executed string
			mgr.SetLoopbackExecutor(func(_, _, payload string, _ time.Duration) error {
				executed = payload
				return nil
			})
			task, err := mgr.ScheduleWaitForEvent(WaitForEventTaskPayload{
				EventType: "process_exited", PID: info.PID, TaskPrompt: prompt,
			}, BackgroundTaskScheduleOptions{Source: "wait_for_event"})
			if err != nil {
				t.Fatal(err)
			}
			mgr.processDueTasks()
			var stored []*BackgroundTask
			if _, err := mgr.store.load(systemTaskNamespaceBackground, &stored); err != nil || len(stored) != 1 {
				t.Fatalf("load persisted task: count=%d err=%v", len(stored), err)
			}
			visible := executed + string(stored[0].Payload) + stored[0].Result + stored[0].LastError
			for _, leaked := range []string{errorSecret, strings.Repeat("x", 500)} {
				if strings.Contains(visible, leaked) {
					t.Fatal("process secret reached the persisted task or follow-up")
				}
			}
			if !strings.Contains(visible, "compiler error: missing header") || !strings.Contains(visible, "exit_code=2") {
				t.Fatalf("task %s lost useful process details", task.ID)
			}
			if !strings.Contains(info.ReadOutput(), logSecret) {
				t.Fatal("internal raw process output changed")
			}
		})
	}
}
