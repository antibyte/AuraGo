//go:build !windows

package tools

import "path/filepath"

func resolveExistingToolPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
