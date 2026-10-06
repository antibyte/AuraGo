//go:build !windows

package server

import (
	"io/fs"
	"path/filepath"
)

// isServableMode reports whether neuteredFileSystem may serve a file of mode
// m: regular files only.
func isServableMode(m fs.FileMode) bool {
	return m.IsRegular()
}

// resolveServedPath returns p with every symlink resolved.
func resolveServedPath(p string) (string, error) {
	return filepath.EvalSymlinks(p)
}
