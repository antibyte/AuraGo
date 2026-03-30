package updater

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// UpdateLock is a simple PID-based lock to prevent concurrent updates.
type UpdateLock struct {
	path string
}

// AcquireLock creates a lock file. Returns error if already locked.
func AcquireLock() (*UpdateLock, error) {
	path := fmt.Sprintf("/tmp/.aurago-update-%d.lock", os.Getuid())
	lock := &UpdateLock{path: path}

	// Check existing lock
	if data, err := os.ReadFile(path); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			// Check if process is still running
			if processExists(pid) {
				return nil, fmt.Errorf("update already running (PID %d)", pid)
			}
		}
		// Stale lock — remove it
		os.Remove(path)
	}

	// Write our PID
	pid := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600); err != nil {
		return nil, fmt.Errorf("write lock file: %w", err)
	}
	return lock, nil
}

// Release removes the lock file.
func (l *UpdateLock) Release() {
	os.Remove(l.path)
}

// processExists checks if a process with the given PID is running.
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check existence.
	err = proc.Signal(os.Signal(nil))
	return err == nil
}
