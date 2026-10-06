//go:build !windows

package server

import "path/filepath"

// resolveServedPath returns p with every symlink resolved.
func resolveServedPath(p string) (string, error) {
	return filepath.EvalSymlinks(p)
}
