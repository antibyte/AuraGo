package decisions

import (
	"context"
	"time"
)

type runtimeCleanup struct {
	done chan struct{}
	err  error
}

func (manager *Manager) lockLifecycle(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if manager.lifecycle.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitRuntimeCleanup(ctx context.Context, task *runtimeCleanup) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return NewError(CodeCancelled, cancellationCause(ctx))
	case <-task.done:
		return task.err
	}
}

// The manager retains failed sessions and the admission barrier until a later
// Stop confirms teardown. Caller cancellation never owns this cleanup budget.
func (manager *Manager) startCleanupLocked(lifecycleHeld bool) *runtimeCleanup {
	if manager.cleanupTask != nil {
		return manager.cleanupTask
	}
	task := &runtimeCleanup{done: make(chan struct{})}
	manager.cleanupTask = task
	manager.signalCleanupChangedLocked()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		defer cancel()
		var err error
		if !lifecycleHeld {
			err = manager.lockLifecycle(ctx)
			if err == nil {
				defer manager.lifecycle.Unlock()
			}
		}
		for {
			if err == nil {
				err = manager.drainSessions(ctx)
			}
			manager.mu.Lock()
			if err != nil || len(manager.pendingSessions) == 0 {
				break
			}
			manager.mu.Unlock()
		}
		if err != nil {
			task.err = NewError(CodeRuntimeUnavailable, err)
			manager.status.State = "stopped_with_error"
			manager.status.Code = CodeRuntimeUnavailable
			manager.status.ErrorMessage = CodeRuntimeUnavailable
		} else if manager.cleanupPending == 0 && manager.session == nil && manager.operationCancel == nil && manager.status.State != "error" {
			manager.status.State, manager.status.Code = "stopped", ""
			if !manager.cfg.Enabled {
				manager.status.State, manager.status.Code = "disabled", CodeServiceDisabled
			}
			if manager.closed {
				manager.status.State = "shutdown"
			}
			manager.status.ErrorMessage = ""
			manager.status.Deployment = manager.cfg.Deployment
			manager.status.Runtime.Deployment = manager.cfg.Deployment
		}
		manager.cleanupTask = nil
		manager.signalCleanupChangedLocked()
		close(task.done)
		manager.mu.Unlock()
	}()
	return task
}

func (manager *Manager) drainSessions(ctx context.Context) error {
	for {
		manager.mu.RLock()
		if len(manager.pendingSessions) == 0 {
			manager.mu.RUnlock()
			return nil
		}
		session := manager.pendingSessions[0]
		manager.mu.RUnlock()
		if err := session.Close(ctx); err != nil {
			return err
		}
		manager.mu.Lock()
		manager.pendingSessions[0] = nil
		manager.pendingSessions = manager.pendingSessions[1:]
		manager.mu.Unlock()
	}
}

func (manager *Manager) closeCandidateSession(ctx context.Context, session runtimeSession) error {
	err := session.Close(ctx)
	if err != nil {
		manager.mu.Lock()
		manager.pendingSessions = append(manager.pendingSessions, session)
		manager.clearActiveRuntimeLocked()
		manager.status.State, manager.status.Code = "stopped_with_error", CodeRuntimeUnavailable
		manager.signalCleanupChangedLocked()
		manager.mu.Unlock()
	}
	return err
}

func (manager *Manager) clearActiveRuntimeLocked() {
	manager.status.Ready = false
	manager.status.CPUFallbackActive = false
	manager.status.CPUFallbackReason = ""
	manager.status.DeviceAttested = false
	manager.status.GPUOffloadVerified = false
	manager.status.Device, manager.status.Backend, manager.status.ExecutionMode = "", "", ""
	manager.status.Runtime.DeviceAttested = false
	manager.status.Runtime.GPUOffloadVerified = false
	manager.status.Runtime.Device, manager.status.Runtime.Backend = "", ""
	manager.activeDeployment, manager.activeDockerHost = "", ""
	manager.activeIsDocker = false
	manager.activeDevice = runtimeDevice{}
}
