package decisions

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
)

func TestCleanupBudgetIncludesLifecycleLock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager := readyManagerForTest(t, validTestSession())
		manager.lifecycle.Lock()
		started := time.Now()
		err := manager.Shutdown(context.Background())
		manager.lifecycle.Unlock()
		if ErrorCode(err) != CodeRuntimeUnavailable || time.Since(started) != 10*time.Second || !manager.cleanupIsPending() {
			t.Fatalf("unbounded/lost shutdown: %v after %v", err, time.Since(started))
		}
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		if manager.Status().State != "shutdown" || manager.cleanupIsPending() {
			t.Fatal("shutdown retry did not finish")
		}
	})
}

func TestStopCallerCancellationDoesNotDropCleanup(t *testing.T) {
	for _, lockHeld := range []bool{false, true} {
		t.Run(map[bool]string{false: "closing", true: "waiting for lifecycle"}[lockHeld], func(t *testing.T) {
			session := &lifecycleRaceBlockingSession{testSession: validTestSession(), closeEntered: make(chan struct{}), releaseClose: make(chan struct{})}
			manager := readyManagerForTest(t, session.testSession)
			manager.session = session
			if lockHeld {
				manager.lifecycle.Lock()
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := manager.Stop(ctx); ErrorCode(err) != CodeCancelled || !errors.Is(err, context.Canceled) {
				t.Fatalf("Stop: %v", err)
			}
			if status := manager.Status(); status.Ready || status.DeviceAttested || status.Runtime.DeviceAttested || status.GPUOffloadVerified || status.Runtime.GPUOffloadVerified {
				t.Fatalf("revoked attestation: %+v", status)
			}
			if lockHeld {
				manager.lifecycle.Unlock()
			}
			select {
			case <-session.closeEntered:
			case <-time.After(time.Second):
				t.Fatal("manager cleanup did not continue")
			}
			if err := manager.Start(t.Context()); ErrorCode(err) != CodeLifecycleBusy {
				t.Fatalf("premature start: %v", err)
			}
			close(session.releaseClose)
			if err := manager.Stop(t.Context()); err != nil {
				t.Fatal(err)
			}
			if manager.cleanupIsPending() || manager.Status().State != "stopped" {
				t.Fatal("cleanup barrier did not settle")
			}
		})
	}
}

type retryCloseSession struct {
	*testSession
	calls int
}

func (session *retryCloseSession) Close(ctx context.Context) error {
	session.calls++
	if session.calls == 1 {
		return errors.New("temporary teardown failure")
	}
	return session.testSession.Close(ctx)
}

func TestFailedCleanupRetainsSessionUntilRepeatedStop(t *testing.T) {
	session := &retryCloseSession{testSession: validTestSession()}
	manager := readyManagerForTest(t, session.testSession)
	manager.session = session
	cleanup, err := manager.PrepareReconfigure(config.DecisionsConfig{Enabled: false, Deployment: "native"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(t.Context()); ErrorCode(err) != CodeRuntimeUnavailable {
		t.Fatalf("cleanup: %v", err)
	}
	if !manager.cleanupIsPending() || manager.Status().State != "stopped_with_error" {
		t.Fatal("failed teardown lost its session/barrier")
	}
	if _, err := manager.PrepareReconfigure(config.DecisionsConfig{Enabled: true, Deployment: "native"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(t.Context()); ErrorCode(err) != CodeLifecycleBusy {
		t.Fatalf("premature start: %v", err)
	}
	if err := manager.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if session.calls != 2 || manager.cleanupIsPending() {
		t.Fatalf("retry calls=%d", session.calls)
	}
}

func TestBusyCannotOverwriteCleanupStatus(t *testing.T) {
	manager := readyManagerForTest(t, validTestSession())
	manager.operationCancel = func() {}
	manager.status.State, manager.status.Code = "stopped_with_error", CodeRuntimeUnavailable
	manager.pendingSessions = []runtimeSession{manager.session}
	manager.session = nil
	manager.clearActiveRuntimeLocked()
	manager.setRuntimeError(NewError(CodeRuntimeAttestationFailed, nil))
	manager.setInstallError(NewError(CodeLifecycleBusy, nil))
	manager.publishLifecycleTransitionFailure(NewError(CodeLifecycleBusy, nil))
	if status := manager.Status(); status.State != "stopped_with_error" || status.Code != CodeRuntimeUnavailable {
		t.Fatalf("cleanup status overwritten: %+v", status)
	}
	manager.pendingSessions = nil
	manager.setRuntimeError(NewError(CodeLifecycleBusy, nil))
	manager.setInstallError(NewError(CodeLifecycleBusy, nil))
	if status := manager.Status(); status.State != "stopped_with_error" || status.Code != CodeRuntimeUnavailable {
		t.Fatalf("busy published an error: %+v", status)
	}
}
