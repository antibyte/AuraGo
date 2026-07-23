package gamemaker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var drivePathPattern = regexp.MustCompile(`^[A-Za-z]:`)

func safeRelativePath(raw string, allowManaged bool) (string, error) {
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: NUL byte", ErrInvalidPath)
	}
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || raw == "." {
		return "", fmt.Errorf("%w: empty path", ErrInvalidPath)
	}
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || drivePathPattern.MatchString(raw) {
		return "", fmt.Errorf("%w: absolute path", ErrInvalidPath)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(raw)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: traversal", ErrInvalidPath)
	}
	first := strings.ToLower(strings.Split(clean, "/")[0])
	if !allowManaged && (first == "vendor" || first == "dist" || strings.HasPrefix(first, ".")) {
		return "", fmt.Errorf("%w: managed path", ErrInvalidPath)
	}
	return clean, nil
}

func secureJoin(root, raw string, allowManaged bool) (string, string, error) {
	rel, err := safeRelativePath(raw, allowManaged)
	if err != nil {
		return "", "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve game maker root: %w", err)
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(rel))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve game maker path: %w", err)
	}
	prefix := rootAbs + string(os.PathSeparator)
	if targetAbs != rootAbs && !strings.HasPrefix(strings.ToLower(targetAbs), strings.ToLower(prefix)) {
		return "", "", fmt.Errorf("%w: escaped root", ErrInvalidPath)
	}
	if err := rejectSymlinkComponents(rootAbs, targetAbs); err != nil {
		return "", "", err
	}
	return targetAbs, rel, nil
}

func rejectSymlinkComponents(rootAbs, targetAbs string) error {
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				return nil
			}
			return fmt.Errorf("inspect game maker path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&fs.ModeIrregular != 0 {
			return fmt.Errorf("%w: symlink or reparse point", ErrInvalidPath)
		}
	}
	return nil
}

func projectKey(slug string) string {
	return filepath.ToSlash(filepath.Join("Games", slug))
}
