package fileutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
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

// nearestExistingDir walks up from path to the first directory that exists. It only walks past
// entries that are missing, below a non-directory, or not directories themselves; any other
// stat failure (permissions, I/O, invalid path) is returned instead of measuring a wrong volume.
func nearestExistingDir(path string) (string, error) {
	if path == "" {
		return "", errors.New("fileutil: empty path")
	}
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		switch {
		case err == nil:
			if info.IsDir() {
				return current, nil
			}
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		default:
			return "", fmt.Errorf("fileutil: stat %s: %w", current, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("fileutil: no existing directory above %s", path)
		}
		current = parent
	}
}
