package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProtectFilesCommand refuses unrestricted process execution while protected user
// files exist. Native file APIs retain their own narrower source-aware policy.
func ProtectFilesCommand(cmd *exec.Cmd, roots []string, contexts ...context.Context) (*exec.Cmd, error) {
	active := []string{}
	for _, root := range roots {
		if _, err := os.Lstat(root); err == nil {
			active = append(active, root)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check protected notes: %w", err)
		}
	}
	if len(active) == 0 {
		return cmd, nil
	}
	protected, err := protectPlatformCommand(cmd, active)
	if err != nil {
		return nil, err
	}
	if protected != cmd && len(contexts) > 0 && contexts[0] != nil {
		bound := exec.CommandContext(contexts[0], protected.Path, protected.Args[1:]...)
		bound.Dir = protected.Dir
		bound.Env = protected.Env
		bound.Stdin = protected.Stdin
		bound.Stdout = protected.Stdout
		bound.Stderr = protected.Stderr
		bound.SysProcAttr = protected.SysProcAttr
		protected = bound
	}
	return protected, nil
}

// ProtectedFilePath includes ancestors only for operations that mutate directories.
func ProtectedFilePath(path string, roots []string, parents bool) bool {
	for _, root := range roots {
		if pathsOverlap(path, root) {
			rel, err := filepath.Rel(canonicalProtectedPath(root), canonicalProtectedPath(path))
			if parents || err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
				return true
			}
		}
	}
	return false
}
func canonicalProtectedPath(path string) string {
	path, _ = filepath.Abs(path)
	var tail []string
	current := path
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return strings.ToLower(filepath.Clean(resolved))
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
	return strings.ToLower(filepath.Clean(path))
}
func pathsOverlap(a, b string) bool {
	a, b = canonicalProtectedPath(a), canonicalProtectedPath(b)
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator)) || a == string(filepath.Separator)
}
