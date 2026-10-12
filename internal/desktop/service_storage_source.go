package desktop

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"aurago/internal/fileutil"
)

// CreateStorageDirectory exclusively creates a transfer directory inside the
// workspace. Existing trees are conflicts, never silently merged.
func (s *Service) CreateStorageDirectory(ctx context.Context, rawPath string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	if _, _, _, mounted, err := s.resolveMediaMount(rawPath); err != nil {
		return err
	} else if mounted {
		return fmt.Errorf("desktop media mounts are read-only")
	}
	p, err := s.resolveWorkspacePathNoSymlinks(rawPath, true)
	if err != nil {
		return err
	}
	if err = s.guardNoteMutation(p, "desktop"); err != nil {
		return err
	}
	rootPath, err := filepath.Abs(s.Config().WorkspaceDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootPath, p)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("invalid storage directory")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = fileutil.PublishContext(ctx, func() error { return root.Mkdir(rel, 0700) }); err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}
	s.invalidateListCache()
	s.invalidateBootstrapCacheForFileMutation(rawPath)
	_ = s.Audit(ctx, "create_directory", rawPath, nil, "desktop")
	return nil
}

// StatStoragePath reads current metadata and a content version without the UI
// listing cache. Transfers must detect external changes as well as app writes.
func (s *Service) StatStoragePath(ctx context.Context, rawPath string, maxBytes int64) (FileEntry, string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return FileEntry{}, "", err
	}
	p, err := s.resolveWorkspacePathNoSymlinks(rawPath, false)
	mount, mediaPath, _, mounted, mediaErr := s.resolveMediaMount(rawPath)
	if mediaErr != nil {
		return FileEntry{}, "", mediaErr
	}
	if mounted {
		p, err = mediaPath, nil
	}
	if err != nil {
		return FileEntry{}, "", err
	}
	in, info, err := secureOpenWorkspaceRead(p)
	if err != nil {
		return FileEntry{}, "", err
	}
	defer in.Close()
	entry := s.workspaceFileEntry(p, info)
	if mounted {
		entry = mediaFileEntry(mount, p, info)
	}
	if info.IsDir() {
		entry.Type, entry.Size = "directory", 0
		return entry, "", nil
	}
	if !info.Mode().IsRegular() || info.Size() > maxBytes {
		return FileEntry{}, "", fmt.Errorf("storage source must be a regular file within the size limit")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(desktopContextReader{ctx, in}, maxBytes+1))
	if err != nil {
		return FileEntry{}, "", err
	}
	after, err := in.Stat()
	if err != nil {
		return FileEntry{}, "", err
	}
	if n != info.Size() || n > maxBytes || after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return FileEntry{}, "", fmt.Errorf("storage source changed while reading")
	}
	return entry, fmt.Sprintf("\"%x\"", h.Sum(nil)), nil
}

// RemoveStorageSource only removes the exact copied file, or an empty directory.
func (s *Service) RemoveStorageSource(ctx context.Context, rawPath, version string, directory bool) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	if _, _, _, mounted, err := s.resolveMediaMount(rawPath); err != nil {
		return err
	} else if mounted {
		return fmt.Errorf("storage move retains media source")
	}
	p, err := s.resolveWorkspacePathNoSymlinks(rawPath, false)
	if err != nil {
		return err
	}
	if err = s.guardNoteMutation(p, "desktop"); err != nil {
		return err
	}
	rootPath, err := filepath.Abs(s.Config().WorkspaceDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootPath, p)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("invalid storage source")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(rel)
	if err != nil {
		return err
	}
	if directory {
		if !info.IsDir() {
			return fmt.Errorf("storage source changed")
		}
	} else {
		if version == "" {
			return fmt.Errorf("storage source version required")
		}
		state, err := s.streamFileState(ctx, root, rel, p, desktopCopyMaxBytes)
		if err != nil {
			return err
		}
		if !state.Exists || state.Version != version {
			return fmt.Errorf("storage source changed")
		}
	}
	if err = fileutil.PublishContext(ctx, func() error { return root.Remove(rel) }); err != nil {
		return fmt.Errorf("remove copied source: %w", err)
	}
	s.invalidateListCache()
	s.invalidateBootstrapCacheForFileMutation(rawPath)
	_ = s.Audit(ctx, "storage_move_source", rawPath, nil, "desktop")
	return nil
}
