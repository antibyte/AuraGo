package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"aurago/internal/fileutil"
)

// WriteFileStreamConditional stages a bounded binary file without buffering it.
// Callers authorize the operation and supply their own explicit size budget.
// Preconditions receive Version instead of Data. A nil precondition is create-only.
// Notes and widget HTML retain their dedicated content-validating write paths.
func (s *Service) WriteFileStreamConditional(ctx context.Context, rawPath string, reader io.Reader, maxBytes int64, source string, precondition FileWritePrecondition) (FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return FileEntry{}, err
	}
	if reader == nil || maxBytes <= 0 || maxBytes > 8<<30 {
		return FileEntry{}, fmt.Errorf("invalid desktop stream size budget")
	}
	if s.Config().ReadOnly {
		return FileEntry{}, fmt.Errorf("virtual desktop is read-only")
	}
	if _, _, _, mounted, err := s.resolveMediaMount(rawPath); err != nil {
		return FileEntry{}, err
	} else if mounted {
		return FileEntry{}, fmt.Errorf("desktop media mounts are read-only for file writes")
	}
	path, err := s.resolveWorkspacePathNoSymlinks(rawPath, true)
	if err != nil {
		return FileEntry{}, err
	}
	if NotesPath(s.relativePath(path), true) || isStandaloneWidgetHTMLPath(rawPath) {
		return FileEntry{}, fmt.Errorf("this desktop file requires a content-validating write")
	}
	rootPath, err := filepath.Abs(s.Config().WorkspaceDir)
	if err != nil {
		return FileEntry{}, fmt.Errorf("resolve desktop root: %w", err)
	}
	rel, err := filepath.Rel(rootPath, path)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return FileEntry{}, fmt.Errorf("invalid desktop stream path")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return FileEntry{}, fmt.Errorf("open desktop root: %w", err)
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(rel), 0700); err != nil {
		return FileEntry{}, fmt.Errorf("create desktop stream directory: %w", err)
	}
	temp := filepath.Join(filepath.Dir(rel), ".aurago-write-"+rand.Text())
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return FileEntry{}, fmt.Errorf("stage desktop stream: %w", err)
	}
	defer root.Remove(temp)
	n, writeErr := io.Copy(out, io.LimitReader(desktopContextReader{ctx, reader}, maxBytes+1))
	if writeErr == nil && n > maxBytes {
		writeErr = fmt.Errorf("desktop stream exceeds size budget")
	}
	if writeErr == nil {
		writeErr = out.Sync()
	}
	info, statErr := out.Stat()
	closeErr := out.Close()
	if writeErr != nil {
		return FileEntry{}, fmt.Errorf("write desktop stream: %w", writeErr)
	}
	if statErr != nil {
		return FileEntry{}, fmt.Errorf("stat desktop stream: %w", statErr)
	}
	if closeErr != nil {
		return FileEntry{}, fmt.Errorf("close desktop stream: %w", closeErr)
	}

	// Preparation and slow source I/O never hold the shared mutation lock.
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return FileEntry{}, err
	}
	if s.Config().ReadOnly {
		return FileEntry{}, fmt.Errorf("virtual desktop is read-only")
	}
	if err := validateNoSymlinkComponents(rootPath, path, true); err != nil {
		return FileEntry{}, err
	}
	state, err := s.streamFileState(ctx, root, rel, path, maxBytes)
	if err != nil {
		return FileEntry{}, err
	}
	if precondition != nil {
		if err := precondition(state); err != nil {
			return FileEntry{}, err
		}
	} else if state.Exists {
		return FileEntry{}, &PathConflict{Path: s.relativePath(path), Directory: state.Entry.Type == "directory"}
	}
	if err := fileutil.RenameRootContext(ctx, root, temp, rel); err != nil {
		return FileEntry{}, fmt.Errorf("publish desktop stream: %w", err)
	}
	entry := s.workspaceFileEntry(path, info)
	s.invalidateListCache()
	s.invalidateBootstrapCacheForFileMutation(entry.Path)
	_ = s.Audit(ctx, "write_file", entry.Path, map[string]interface{}{"bytes": n}, source)
	return entry, nil
}

func (s *Service) streamFileState(ctx context.Context, root *os.Root, rel, path string, maxBytes int64) (FileWriteState, error) {
	info, err := root.Lstat(rel)
	if os.IsNotExist(err) {
		return FileWriteState{}, nil
	}
	if err != nil {
		return FileWriteState{}, fmt.Errorf("stat desktop target: %w", err)
	}
	if !info.Mode().IsRegular() {
		return FileWriteState{}, fmt.Errorf("desktop target must be a regular file")
	}
	if info.Size() > maxBytes {
		return FileWriteState{}, fmt.Errorf("desktop target exceeds size budget")
	}
	in, err := root.Open(rel)
	if err != nil {
		return FileWriteState{}, fmt.Errorf("open desktop target: %w", err)
	}
	defer in.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(desktopContextReader{ctx, in}, maxBytes+1))
	if err != nil {
		return FileWriteState{}, fmt.Errorf("hash desktop target: %w", err)
	}
	if n > maxBytes {
		return FileWriteState{}, fmt.Errorf("desktop target exceeds size budget")
	}
	return FileWriteState{Exists: true, Entry: s.workspaceFileEntry(path, info), Version: fmt.Sprintf("\"%x\"", hash.Sum(nil))}, nil
}
