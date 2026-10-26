package dockerutil

import (
	"path"
	"strings"
	"sync"
)

// Keep observed paths until process exit: stopped/removed runtimes retain their private volumes.
var rustFSPaths = struct {
	sync.RWMutex
	known map[string]struct{}
}{known: map[string]struct{}{
	"/var/lib/docker/volumes/" + RustFSDataVolumeName + "/_data":       {},
	"/var/lib/docker/volumes/" + RustFSCredentialVolumeName + "/_data": {},
}}

// RegisterRustFSVolumePath accepts only an already ownership-verified Engine mountpoint.
// Mountpoints remain private; this registry exposes only containment decisions.
func RegisterRustFSVolumePath(raw string) {
	value := normalizeRustFSVolumePath(raw)
	if value == "." || value == "/" || value == "" {
		return
	}
	if !strings.HasPrefix(value, "/") && !(len(value) > 2 && value[1] == ':' && value[2] == '/') {
		return
	}
	rustFSPaths.Lock()
	rustFSPaths.known[value] = struct{}{}
	rustFSPaths.Unlock()
}

func normalizeRustFSVolumePath(raw string) string {
	value := path.Clean(NormalizeHostPathForBind(raw))
	if len(value) >= 2 && value[1] == ':' {
		value = strings.ToLower(value)
	}
	return value
}

func rustFSVolumePath(raw string) bool {
	value := normalizeRustFSVolumePath(raw)
	if value == "." || value == "" {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if IsRustFSVolumeName(part) {
			return true
		}
	}
	rustFSPaths.RLock()
	defer rustFSPaths.RUnlock()
	for protected := range rustFSPaths.known {
		if rustFSPathContains(value, protected) || rustFSPathContains(protected, value) {
			return true
		}
	}
	return false
}

func rustFSPathContains(parent, child string) bool {
	if parent == "/" {
		return strings.HasPrefix(child, "/")
	}
	parent = strings.TrimRight(parent, "/")
	return parent == child || strings.HasPrefix(child, parent+"/")
}
