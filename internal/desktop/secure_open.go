package desktop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) resolveWorkspacePathNoSymlinks(rawPath string, allowMissing bool) (string, error) {
	cfg := s.Config()
	cleaned := cleanDesktopPath(rawPath)
	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Join(cfg.WorkspaceDir, cleaned)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve desktop path: %w", err)
	}
	rootAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve desktop root: %w", err)
	}
	if !isWithinPath(rootAbs, candidateAbs) {
		return "", fmt.Errorf("desktop path escapes workspace")
	}
	if err := validateNoSymlinkComponents(rootAbs, candidateAbs, allowMissing); err != nil {
		return "", err
	}
	return candidateAbs, nil
}

func (s *Service) resolveWorkspaceRenameDestinationNoSymlinkParent(rawPath string) (string, error) {
	cfg := s.Config()
	cleaned := cleanDesktopPath(rawPath)
	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Join(cfg.WorkspaceDir, cleaned)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve desktop path: %w", err)
	}
	rootAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve desktop root: %w", err)
	}
	if !isWithinPath(rootAbs, candidateAbs) {
		return "", fmt.Errorf("desktop path escapes workspace")
	}
	if err := validateNoSymlinkComponents(rootAbs, filepath.Dir(candidateAbs), true); err != nil {
		return "", err
	}
	return candidateAbs, nil
}

func validateNoSymlinkComponents(rootAbs, candidateAbs string, allowMissing bool) error {
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return fmt.Errorf("resolve desktop relative path: %w", err)
	}
	if rel == "." {
		return nil
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	current := rootAbs
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				if allowMissing {
					return nil
				}
				return fmt.Errorf("inspect desktop path: %w", err)
			}
			return fmt.Errorf("inspect desktop path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if i == len(parts)-1 {
				return fmt.Errorf("desktop path is a symlink")
			}
			return fmt.Errorf("desktop path contains a symlink")
		}
	}
	return nil
}

func secureOpenWorkspaceRead(path string) (*os.File, os.FileInfo, error) {
	file, err := openFileNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		return nil, nil, fmt.Errorf("desktop path is a symlink")
	}
	return file, info, nil
}

func secureReadWorkspaceFile(path string) ([]byte, os.FileInfo, error) {
	file, info, err := secureOpenWorkspaceRead(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	return data, info, nil
}

func secureWriteWorkspaceFile(path string, content []byte) (os.FileInfo, error) {
	file, err := openFileNoFollow(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	writeErr := func() error {
		if _, err := file.Write(content); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		return nil
	}()
	info, statErr := file.Stat()
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if statErr != nil {
		return nil, statErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	_ = os.Chmod(path, 0o600)
	return info, nil
}
