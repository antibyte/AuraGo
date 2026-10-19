package tools

import (
	"aurago/internal/uid"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrBackgroundTaskPending means the same execution is still running. Polling
// its result is not a new attempt and must not consume the retry allowance.
var ErrBackgroundTaskOutcomeUnknown = errors.New("background task outcome unknown")
var ErrBackgroundTaskBlocked = errors.New("background task requires an explicit configuration rule")

var ErrBackgroundTaskPending = errors.New("background task execution pending")

func (m *BackgroundTaskManager) promptTaskContext(id string) (context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[id]
	if task == nil || task.Status != BackgroundTaskStatusRunning || m.ctx.Err() != nil {
		return nil, false
	}
	if owner := m.taskContexts[id]; owner != nil {
		return owner, true
	}
	if m.taskContexts == nil {
		m.taskContexts = make(map[string]context.Context)
	}
	if m.taskCancels == nil {
		m.taskCancels = make(map[string]context.CancelFunc)
	}
	owner, cancel := context.WithCancel(m.ctx)
	m.taskContexts[id], m.taskCancels[id] = owner, cancel
	return owner, true
}

func (m *BackgroundTaskManager) releaseTaskContextLocked(id string) {
	if cancel := m.taskCancels[id]; cancel != nil {
		cancel()
	}
	delete(m.taskCancels, id)
	delete(m.taskContexts, id)
}

func (m *BackgroundTaskManager) executionContextLocked(executionID string) (context.Context, bool) {
	if executionID == "" || m.ctx.Err() != nil {
		return nil, false
	}
	for id, owner := range m.taskContexts {
		task := m.tasks[id]
		if task != nil && task.ExecutionID == executionID && !backgroundTaskTerminal(task.Status) && owner.Err() == nil {
			return owner, true
		}
	}
	return nil, false
}

// ExecutionContext resolves only an admitted, currently live task execution.
// An arbitrary HTTP execution ID cannot create or choose another task owner.
func (m *BackgroundTaskManager) ExecutionContext(executionID string) (context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.executionContextLocked(executionID)
}

// BeginExecution leases the one server-side cached run until its handler exits.
// The existing manager wait group keeps stores alive while cancellation drains.
func (m *BackgroundTaskManager) BeginExecution(executionID string) (context.Context, func(), bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	owner, ok := m.executionContextLocked(executionID)
	if !ok {
		return nil, nil, false
	}
	m.workerWg.Add(1)
	var once sync.Once
	return owner, func() { once.Do(m.workerWg.Done) }, true
}

func (m *BackgroundTaskManager) promptExecutionID(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok || task.Status != BackgroundTaskStatusRunning {
		return "", fmt.Errorf("background task is not running")
	}
	if task.ExecutionID == "" {
		task.ExecutionID = fmt.Sprintf("%s_run_%s", id, uid.New())
		if err := m.saveLocked(); err != nil {
			task.ExecutionID = ""
			return "", err
		}
	}
	return task.ExecutionID, nil
}

func reconcileInterruptedBackgroundTask(task *BackgroundTask) {
	switch task.Status {
	case BackgroundTaskStatusRunning, BackgroundTaskStatusWaiting, BackgroundTaskStatusQueued:
		if task.ExecutionID != "" {
			// An interrupted process may already have performed external writes.
			// Only an explicit retry may create a new execution identity.
			now := time.Now().UTC()
			task.Status = BackgroundTaskStatusFailed
			task.LastError = "background execution interrupted by restart; check its effects before retrying"
			task.UpdatedAt = now
			task.CompletedAt = &now
		} else if task.Status == BackgroundTaskStatusRunning {
			task.Status = BackgroundTaskStatusQueued
			task.StartedAt = nil
		}
	}
}
