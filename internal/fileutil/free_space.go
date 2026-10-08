package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FreeDiskBytes returns the bytes available to the current user on the filesystem holding path.
// path may not exist yet; the nearest existing parent directory is measured.
func FreeDiskBytes(path string) (int64, error) {
	dir, err := nearestExistingDir(path)
	if err != nil {
		return 0, err
	}
	free, err := freeDiskBytes(dir)
	if err != nil {
		return 0, fmt.Errorf("fileutil: free space of %s: %w", dir, err)
	}
	return free, nil
}

// nearestExistingDir walks up from path to the first directory that exists.
func nearestExistingDir(path string) (string, error) {
	if path == "" {
		return "", errors.New("fileutil: empty path")
	}
	current := filepath.Clean(path)
	for {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("fileutil: no existing directory above %s", path)
		}
		current = parent
	}
}
