package tools

import (
	"errors"
	"fmt"
	"time"
)

// ErrBackgroundTaskPending means the same execution is still running. Polling
// its result is not a new attempt and must not consume the retry allowance.
var ErrBackgroundTaskPending = errors.New("background task execution pending")

func (m *BackgroundTaskManager) promptExecutionID(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok || task.Status != BackgroundTaskStatusRunning {
		return "", fmt.Errorf("background task is not running")
	}
	if task.ExecutionID == "" {
		task.ExecutionID = fmt.Sprintf("%s_run_%d", id, time.Now().UnixNano())
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
