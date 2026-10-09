//go:build !windows

package localwiki

import "path/filepath"

// resolveExistingDir returns the real location of an existing directory with
// every symbolic link resolved.
func resolveExistingDir(dir string) (string, error) {
	return filepath.EvalSymlinks(dir)
}
