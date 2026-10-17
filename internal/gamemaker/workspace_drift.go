package gamemaker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const driftListCap = 40

// WorkspaceDrift compares a published revision with the editable workspace.
// Paths are project-relative. Export dot paths such as .aurago/ are omitted.
type WorkspaceDrift struct {
	Revision     int64    `json:"revision"`
	Changed      []string `json:"changed"`
	Added        []string `json:"added"`
	Removed      []string `json:"removed"`
	ChangedCount int      `json:"changed_count"`
	AddedCount   int      `json:"added_count"`
	RemovedCount int      `json:"removed_count"`
	Truncated    bool     `json:"truncated"`
}

func (d WorkspaceDrift) Empty() bool {
	return d.ChangedCount == 0 && d.AddedCount == 0 && d.RemovedCount == 0
}

// PromptView is the untrusted planning context: counts, capped paths and no file contents.
func (d WorkspaceDrift) PromptView() map[string]any {
	return map[string]any{
		"changed_count": d.ChangedCount,
		"added_count":   d.AddedCount,
		"removed_count": d.RemovedCount,
		"changed":       d.Changed,
		"added":         d.Added,
		"removed":       d.Removed,
		"truncated":     d.Truncated,
	}
}

func skippedDriftPath(rel string) bool {
	return strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.")
}

// WorkspaceDrift hashes the workspace without writing blobs. Revision 0 and a
// matching tree both return empty counts. A missing workspace lists the
// published files as removed.
func (s *Service) WorkspaceDrift(ctx context.Context, project Project) (WorkspaceDrift, error) {
	out := WorkspaceDrift{
		Revision: project.CurrentRevision,
		Changed:  []string{},
		Added:    []string{},
		Removed:  []string{},
	}
	if project.CurrentRevision <= 0 {
		return out, nil
	}
	if err := ctx.Err(); err != nil {
		return WorkspaceDrift{}, err
	}
	published, err := s.revisionContentHashes(ctx, project)
	if err != nil {
		return WorkspaceDrift{}, s.redactDriftError(err)
	}
	root := filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	workspace, missing, err := s.hashWorkspaceTree(ctx, root)
	if err != nil {
		return WorkspaceDrift{}, s.redactDriftError(err)
	}
	if missing {
		workspace = map[string]string{}
	}
	var changed, added, removed []string
	for path, hash := range workspace {
		publishedHash, ok := published[path]
		if !ok {
			added = append(added, path)
			continue
		}
		if publishedHash != hash {
			changed = append(changed, path)
		}
	}
	for path := range published {
		if _, ok := workspace[path]; !ok {
			removed = append(removed, path)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	sort.Strings(removed)
	out.ChangedCount = len(changed)
	out.AddedCount = len(added)
	out.RemovedCount = len(removed)
	out.Changed = capDriftPaths(changed)
	out.Added = capDriftPaths(added)
	out.Removed = capDriftPaths(removed)
	out.Truncated = out.ChangedCount > driftListCap || out.AddedCount > driftListCap || out.RemovedCount > driftListCap
	return out, nil
}

func (s *Service) redactDriftError(err error) error {
	if err == nil {
		return nil
	}
	redacted := s.RedactHostPaths(err.Error())
	if errors.Is(err, ErrInvalidPath) {
		return fmt.Errorf("%w: %s", ErrInvalidPath, redacted)
	}
	return errors.New(redacted)
}

func capDriftPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}
	if len(paths) > driftListCap {
		return append([]string(nil), paths[:driftListCap]...)
	}
	return paths
}

func (s *Service) revisionContentHashes(ctx context.Context, project Project) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT f.path, f.content_hash
		FROM gm_revision_files f JOIN gm_revisions r ON r.id=f.revision_id
		WHERE r.project_id=? AND r.number=?`, project.ID, project.CurrentRevision)
	if err != nil {
		return nil, fmt.Errorf("read published revision: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, fmt.Errorf("read published revision file: %w", err)
		}
		rel, err := safeRelativePath(path, true)
		if err != nil || rel != path {
			return nil, fmt.Errorf("%w: non-canonical revision path", ErrInvalidPath)
		}
		if skippedDriftPath(rel) {
			continue
		}
		out[rel] = hash
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read published revision files: %w", err)
	}
	return out, nil
}

func (s *Service) hashWorkspaceTree(ctx context.Context, root string) (map[string]string, bool, error) {
	info, err := os.Lstat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, true, nil
		}
		return nil, false, fmt.Errorf("compare workspace: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: project symlink", ErrInvalidPath)
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("%w: workspace is not a directory", ErrInvalidPath)
	}
	out := map[string]string{}
	var files int
	var total int64
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in project", ErrInvalidPath)
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		canonical, err := safeRelativePath(rel, true)
		if err != nil || canonical != rel {
			return fmt.Errorf("%w: non-canonical workspace path", ErrInvalidPath)
		}
		if info.Size() < 0 {
			return fmt.Errorf("%w: negative file size", ErrInvalidPath)
		}
		files++
		total += info.Size()
		if files > s.opts.MaxFilesPerProject || total > s.opts.MaxProjectBytes {
			return fmt.Errorf("game maker project exceeds configured limits")
		}
		if s.opts.MaxFileBytes > 0 && info.Size() > s.opts.MaxFileBytes {
			return fmt.Errorf("game maker file exceeds configured limit")
		}
		hash, err := hashFileContents(path)
		if err != nil {
			return fileError("compare", rel, err)
		}
		if !skippedDriftPath(rel) {
			out[rel] = hash
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrInvalidPath) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false, err
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			rel, relErr := filepath.Rel(root, pathErr.Path)
			if relErr != nil || rel == "." {
				rel = "workspace"
			}
			return nil, false, fileError("compare", filepath.ToSlash(rel), pathErr.Err)
		}
		return nil, false, fmt.Errorf("compare workspace: %w", err)
	}
	return out, false, nil
}

func hashFileContents(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}
