package gamemaker

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Operating-system errors embed the resolved location. Agents, Studio and the
// ledger only ever receive project-relative paths.

// fileError reports a failed project file operation by its relative path. It
// stays a PathError, so os.IsNotExist and errors.Is keep working for callers.
func fileError(action, rel string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		err = linkErr.Err
	}
	return &fs.PathError{Op: action, Path: rel, Err: err}
}

// RedactHostPaths removes this service's storage locations from a message. It
// is the last boundary before text reaches a model, a client or a job record.
func (s *Service) RedactHostPaths(message string) string {
	if s == nil || message == "" {
		return message
	}
	roots := []string{s.stagingDir, s.blobDir}
	for _, candidate := range []string{s.opts.WorkspacePath, filepath.Dir(s.opts.DBPath)} {
		roots = append(roots, candidate)
		if abs, err := filepath.Abs(candidate); err == nil {
			roots = append(roots, abs)
		}
	}
	for _, root := range append([]string(nil), roots...) {
		if abs, err := filepath.Abs(root); err == nil {
			roots = append(roots, abs)
		}
	}
	// Replace nested locations before the directories that contain them.
	sort.SliceStable(roots, func(i, j int) bool { return len(roots[i]) > len(roots[j]) })
	seen := map[string]bool{}
	for _, root := range roots {
		root = strings.TrimRight(strings.TrimSpace(root), `/\`)
		key := strings.ToLower(filepath.ToSlash(root))
		// Never treat a filesystem or drive root as a storage location.
		if len(key) < 4 || seen[key] || filepath.VolumeName(root)+string(os.PathSeparator) == root+string(os.PathSeparator) {
			continue
		}
		seen[key] = true
		parts := strings.FieldsFunc(root, func(r rune) bool { return r == '/' || r == '\\' })
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		pattern := strings.Join(parts, `[\\/]+`)
		if strings.HasPrefix(root, "/") {
			pattern = `/+` + pattern
		}
		expression, err := regexp.Compile(`(?i)` + pattern + `(?:[\\/]+job_[0-9a-f]+)?[\\/]*`)
		if err != nil {
			continue
		}
		message = expression.ReplaceAllString(message, "")
	}
	return message
}
