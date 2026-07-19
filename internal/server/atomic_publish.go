package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var errAtomicPublishTargetExists = errors.New("publish target already exists")

func publishFileNoReplace(tempPath, targetPath string) error {
	tempDir, err := filepath.Abs(filepath.Dir(tempPath))
	if err != nil {
		return fmt.Errorf("resolve temporary directory: %w", err)
	}
	targetDir, err := filepath.Abs(filepath.Dir(targetPath))
	if err != nil {
		return fmt.Errorf("resolve target directory: %w", err)
	}
	if filepath.Clean(tempDir) != filepath.Clean(targetDir) {
		return fmt.Errorf("temporary file and publish target must share a directory")
	}
	if err := os.Link(tempPath, targetPath); err != nil {
		if errors.Is(err, fs.ErrExist) || os.IsExist(err) {
			return fmt.Errorf("%w: %s", errAtomicPublishTargetExists, filepath.Base(targetPath))
		}
		return fmt.Errorf("atomically link published file: %w", err)
	}
	_ = os.Remove(tempPath)
	if directory, openErr := os.Open(targetDir); openErr == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}
