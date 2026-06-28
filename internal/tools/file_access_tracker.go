package tools

import "sync"

// FileAccessTracker receives successful filesystem-style file accesses.
type FileAccessTracker func(workspaceDir, path, kind string)

var (
	fileAccessTrackerMu sync.RWMutex
	fileAccessTracker   FileAccessTracker
)

// SetFileAccessTracker installs the package-level access callback used by services.
func SetFileAccessTracker(fn FileAccessTracker) {
	fileAccessTrackerMu.Lock()
	fileAccessTracker = fn
	fileAccessTrackerMu.Unlock()
}

func trackFileAccess(workspaceDir, path, kind string) {
	fileAccessTrackerMu.RLock()
	fn := fileAccessTracker
	fileAccessTrackerMu.RUnlock()
	if fn == nil || path == "" {
		return
	}
	fn(workspaceDir, path, kind)
}
