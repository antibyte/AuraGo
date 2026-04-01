package tools

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	isBusy       bool
	muBusy       sync.RWMutex
	busyFilePath string
)

func resolveBusyFilePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	return filepath.Abs(path)
}

// SetBusyFilePath sets the path for the persistent busy flag file.
func SetBusyFilePath(path string) {
	muBusy.Lock()
	defer muBusy.Unlock()
	busyFilePath = path
}

// SetBusy sets the maintenance mode flag.
func SetBusy(busy bool) {
	muBusy.Lock()
	defer muBusy.Unlock()
	isBusy = busy
	slog.Info("Setting busy state", "busy", busy, "has_path", busyFilePath != "")
	if busyFilePath != "" {
		absPath, err := resolveBusyFilePath(busyFilePath)
		if err != nil {
			slog.Error("Failed to resolve maintenance lock path", "path", busyFilePath, "error", err)
			return
		}
		if busy {
			slog.Info("Attempting to write maintenance lock file", "path", absPath)
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				slog.Error("Failed to create parent directory for maintenance lock", "path", filepath.Dir(absPath), "error", err)
			}
			if err := os.WriteFile(absPath, []byte("busy"), 0644); err != nil {
				slog.Error("Failed to write maintenance lock", "path", absPath, "error", err)
			} else {
				slog.Info("Successfully wrote maintenance lock file", "path", absPath)
			}
		} else {
			slog.Info("Attempting to remove maintenance lock file", "path", absPath)
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				slog.Error("Failed to remove maintenance lock", "path", absPath, "error", err)
			} else {
				slog.Info("Successfully removed maintenance lock file (or it didn't exist)", "path", absPath)
			}
		}
	}
}

// IsBusy returns the current maintenance mode status.
func IsBusy() bool {
	muBusy.RLock()
	defer muBusy.RUnlock()
	if isBusy {
		slog.Debug("Maintenance mode detected via in-memory flag")
		return true
	}
	if busyFilePath != "" {
		_, err := os.Stat(busyFilePath)
		if err == nil {
			slog.Debug("Maintenance mode detected via lock file", "path", busyFilePath)
			return true
		}
	}
	return false
}

// GetBusyFilePath returns the current path for the busy flag.
func GetBusyFilePath() string {
	muBusy.RLock()
	defer muBusy.RUnlock()
	return busyFilePath
}
