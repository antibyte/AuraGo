package desktop

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListFiles lists one workspace directory.
func (s *Service) ListFiles(ctx context.Context, rawPath string) ([]FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if mount, dirPath, _, ok, err := s.resolveMediaMount(rawPath); ok || err != nil {
		if err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return nil, fmt.Errorf("list desktop media files: %w", err)
		}
		result := make([]FileEntry, 0, len(entries))
		for _, entry := range entries {
			info, statErr := entry.Info()
			if statErr != nil {
				return nil, fmt.Errorf("stat desktop media file %s: %w", entry.Name(), statErr)
			}
			result = append(result, mediaFileEntry(mount, filepath.Join(dirPath, entry.Name()), info))
		}
		sortFileEntries(result)
		return result, nil
	}
	dirPath, err := s.ResolvePath(rawPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("list desktop files: %w", err)
	}
	result := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil {
			return nil, fmt.Errorf("stat desktop file %s: %w", entry.Name(), statErr)
		}
		itemType := "file"
		if info.IsDir() {
			itemType = "directory"
		}
		result = append(result, FileEntry{
			Name:      entry.Name(),
			Path:      s.relativePath(filepath.Join(dirPath, entry.Name())),
			Type:      itemType,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Modified:  info.ModTime(),
			MIMEType:  MIMETypeForName(entry.Name()),
			MediaKind: desktopMediaKindForName(entry.Name()),
			Mode:      info.Mode().String(),
			Created:   getCreationTime(info),
		})
	}
	if cleanDesktopPathSlash(rawPath) == "." {
		for _, mount := range s.mediaMounts() {
			if info, err := os.Stat(mount.Dir); err == nil {
				result = append(result, FileEntry{
					Name:      mount.Name,
					Path:      mount.Name,
					Type:      "directory",
					Size:      info.Size(),
					ModTime:   info.ModTime(),
					Modified:  info.ModTime(),
					MediaKind: mount.Kind,
					Mount:     mount.Name,
					Mode:      info.Mode().String(),
					Created:   getCreationTime(info),
				})
			}
		}
	}
	sortFileEntries(result)
	return result, nil
}

// ListFilesRecursive lists files below one desktop directory or media mount.
func (s *Service) ListFilesRecursive(ctx context.Context, rawPath string, offset, limit int) ([]FileEntry, bool, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, false, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	var result []FileEntry
	if mount, dirPath, _, ok, err := s.resolveMediaMount(rawPath); ok || err != nil {
		if err != nil {
			return nil, false, err
		}
		if err := filepath.WalkDir(dirPath, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == dirPath || entry.IsDir() {
				return nil
			}
			info, statErr := entry.Info()
			if statErr != nil {
				return fmt.Errorf("stat desktop media file %s: %w", entry.Name(), statErr)
			}
			result = append(result, mediaFileEntry(mount, path, info))
			return nil
		}); err != nil {
			return nil, false, fmt.Errorf("list desktop media files recursively: %w", err)
		}
		sortFileEntriesByNewest(result)
		return pageFileEntries(result, offset, limit)
	}
	dirPath, err := s.ResolvePath(rawPath)
	if err != nil {
		return nil, false, err
	}
	if err := filepath.WalkDir(dirPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dirPath || entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return fmt.Errorf("stat desktop file %s: %w", entry.Name(), statErr)
		}
		result = append(result, FileEntry{
			Name:    entry.Name(),
			Path:    s.relativePath(path),
			Type:    "file",
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Modified: info.ModTime(),
			Mode:    info.Mode().String(),
			Created: getCreationTime(info),
		})
		return nil
	}); err != nil {
		return nil, false, fmt.Errorf("list desktop files recursively: %w", err)
	}
	sortFileEntriesByNewest(result)
	return pageFileEntries(result, offset, limit)
}

func sortFileEntries(result []FileEntry) {
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type == "directory"
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
}

func sortFileEntriesByNewest(result []FileEntry) {
	sort.Slice(result, func(i, j int) bool {
		if !result[i].ModTime.Equal(result[j].ModTime) {
			return result[i].ModTime.After(result[j].ModTime)
		}
		return strings.ToLower(result[i].Path) < strings.ToLower(result[j].Path)
	})
}

func pageFileEntries(result []FileEntry, offset, limit int) ([]FileEntry, bool, error) {
	if offset >= len(result) {
		return []FileEntry{}, false, nil
	}
	if limit == 0 {
		return result[offset:], false, nil
	}
	end := offset + limit
	if end >= len(result) {
		return result[offset:], false, nil
	}
	return result[offset:end], true, nil
}

// ReadFile reads a UTF-8 text file from the workspace.
func (s *Service) ReadFile(ctx context.Context, rawPath string) (string, FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", FileEntry{}, err
	}
	if mount, path, _, ok, err := s.resolveMediaMount(rawPath); ok || err != nil {
		if err != nil {
			return "", FileEntry{}, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", FileEntry{}, fmt.Errorf("stat desktop media file: %w", err)
		}
		if info.IsDir() {
			return "", FileEntry{}, fmt.Errorf("desktop media path is a directory")
		}
		entry := mediaFileEntry(mount, path, info)
		if !isDesktopTextReadable(entry.Name) {
			return "", entry, fmt.Errorf("desktop media file is binary; use web_path or download")
		}
		maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
		if info.Size() > maxBytes {
			return "", FileEntry{}, fmt.Errorf("desktop media file exceeds max size")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", FileEntry{}, fmt.Errorf("read desktop media file: %w", err)
		}
		return string(data), entry, nil
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return "", FileEntry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", FileEntry{}, fmt.Errorf("stat desktop file: %w", err)
	}
	if info.IsDir() {
		return "", FileEntry{}, fmt.Errorf("desktop path is a directory")
	}
	maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
	if info.Size() > maxBytes {
		return "", FileEntry{}, fmt.Errorf("desktop file exceeds max size")
	}
	if !isDesktopTextReadable(path) {
		return "", FileEntry{
			Name:      filepath.Base(path),
			Path:      s.relativePath(path),
			Type:      "file",
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			Modified:  info.ModTime(),
			MIMEType:  MIMETypeForName(path),
			MediaKind: desktopMediaKindForName(path),
			Mode:      info.Mode().String(),
			Created:   getCreationTime(info),
		}, fmt.Errorf("desktop file is binary; use ReadFileBytes or download")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", FileEntry{}, fmt.Errorf("read desktop file: %w", err)
	}
	return string(data), FileEntry{
		Name:    filepath.Base(path),
		Path:    s.relativePath(path),
		Type:    "file",
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Modified: info.ModTime(),
		Mode:    info.Mode().String(),
		Created: getCreationTime(info),
	}, nil
}

// ReadFileBytes reads one binary or text file from the workspace.
func (s *Service) ReadFileBytes(ctx context.Context, rawPath string) ([]byte, FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, FileEntry{}, err
	}
	if mount, path, _, ok, err := s.resolveMediaMount(rawPath); ok || err != nil {
		if err != nil {
			return nil, FileEntry{}, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, FileEntry{}, fmt.Errorf("stat desktop media file: %w", err)
		}
		if info.IsDir() {
			return nil, FileEntry{}, fmt.Errorf("desktop media path is a directory")
		}
		maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
		if maxBytes <= 0 {
			maxBytes = 50 * 1024 * 1024
		}
		if info.Size() > maxBytes {
			return nil, FileEntry{}, fmt.Errorf("desktop media file exceeds max size")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, FileEntry{}, fmt.Errorf("read desktop media file: %w", err)
		}
		return data, mediaFileEntry(mount, path, info), nil
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return nil, FileEntry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, FileEntry{}, fmt.Errorf("stat desktop file: %w", err)
	}
	if info.IsDir() {
		return nil, FileEntry{}, fmt.Errorf("desktop path is a directory")
	}
	maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024
	}
	if info.Size() > maxBytes {
		return nil, FileEntry{}, fmt.Errorf("desktop file exceeds max size")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, FileEntry{}, fmt.Errorf("read desktop file: %w", err)
	}
	return data, FileEntry{
		Name:      filepath.Base(path),
		Path:      s.relativePath(path),
		Type:      "file",
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Modified:  info.ModTime(),
		MIMEType:  MIMETypeForName(path),
		MediaKind: desktopMediaKindForName(path),
		Mode:      info.Mode().String(),
		Created:   getCreationTime(info),
	}, nil
}

// OpenPreviewFile opens a workspace or media file for read-only inline preview.
func (s *Service) OpenPreviewFile(ctx context.Context, rawPath string) (*os.File, FileEntry, string, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, FileEntry{}, "", err
	}
	if mount, path, _, ok, err := s.resolveMediaMount(rawPath); ok || err != nil {
		if err != nil {
			return nil, FileEntry{}, "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, FileEntry{}, "", fmt.Errorf("stat desktop media preview: %w", err)
		}
		if info.IsDir() {
			return nil, FileEntry{}, "", fmt.Errorf("desktop preview path is a directory")
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, FileEntry{}, "", fmt.Errorf("open desktop media preview: %w", err)
		}
		entry := mediaFileEntry(mount, path, info)
		return file, entry, entry.MIMEType, nil
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return nil, FileEntry{}, "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, FileEntry{}, "", fmt.Errorf("stat desktop preview: %w", err)
	}
	if info.IsDir() {
		return nil, FileEntry{}, "", fmt.Errorf("desktop preview path is a directory")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, FileEntry{}, "", fmt.Errorf("open desktop preview: %w", err)
	}
	return file, FileEntry{
		Name:     filepath.Base(path),
		Path:     s.relativePath(path),
		Type:     "file",
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Modified: info.ModTime(),
		MIMEType: MIMETypeForName(path),
		Mode:     info.Mode().String(),
		Created:  getCreationTime(info),
	}, MIMETypeForName(path), nil
}

// WriteFile writes a UTF-8 text file into the workspace.
func (s *Service) WriteFile(ctx context.Context, rawPath, content, source string) error {
	_, err := s.writeFileBytes(ctx, rawPath, []byte(content), source, true, nil)
	return err
}

// WriteFileBytes writes binary or text bytes into the workspace.
func (s *Service) WriteFileBytes(ctx context.Context, rawPath string, content []byte, source string) error {
	_, err := s.WriteFileBytesConditional(ctx, rawPath, content, source, nil)
	return err
}

// WriteFileBytesConditional writes bytes if the optional precondition accepts the current target state.
func (s *Service) WriteFileBytesConditional(ctx context.Context, rawPath string, content []byte, source string, precondition FileWritePrecondition) (FileEntry, error) {
	return s.writeFileBytes(ctx, rawPath, content, source, false, precondition)
}

func (s *Service) writeFileBytes(ctx context.Context, rawPath string, content []byte, source string, textWrite bool, precondition FileWritePrecondition) (FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return FileEntry{}, err
	}
	if s.Config().ReadOnly {
		return FileEntry{}, fmt.Errorf("virtual desktop is read-only")
	}
	maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024
	}
	if int64(len(content)) > maxBytes {
		return FileEntry{}, fmt.Errorf("desktop file exceeds max size")
	}
	if isStandaloneWidgetHTMLPath(rawPath) && strings.TrimSpace(string(content)) == "" {
		return FileEntry{}, fmt.Errorf("desktop widget HTML file must not be empty")
	}
	if _, _, _, ok, err := s.resolveMediaMount(rawPath); err != nil {
		return FileEntry{}, err
	} else if ok {
		if textWrite {
			return FileEntry{}, fmt.Errorf("desktop media mounts are read-only for text writes")
		}
		return FileEntry{}, fmt.Errorf("desktop media mounts are read-only for file writes")
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return FileEntry{}, err
	}

	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()

	if precondition != nil {
		state, err := s.fileWriteStateLocked(path, maxBytes)
		if err != nil {
			return FileEntry{}, err
		}
		if err := precondition(state); err != nil {
			return FileEntry{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FileEntry{}, fmt.Errorf("create desktop file directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return FileEntry{}, fmt.Errorf("write desktop file: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return FileEntry{}, fmt.Errorf("stat written desktop file: %w", err)
	}
	entry := s.workspaceFileEntry(path, info)
	_ = s.Audit(ctx, "write_file", s.relativePath(path), map[string]interface{}{"bytes": len(content)}, source)
	s.invalidateBootstrapCacheForFileMutation(entry.Path)
	return entry, nil
}

func (s *Service) invalidateBootstrapCacheForFileMutation(paths ...string) {
	for _, rawPath := range paths {
		clean := cleanDesktopPathSlash(rawPath)
		if clean == "." || clean == "" {
			s.invalidateBootstrapCache()
			return
		}
		top := clean
		if idx := strings.Index(top, "/"); idx >= 0 {
			top = top[:idx]
		}
		switch strings.ToLower(top) {
		case "apps", "widgets", "desktop":
			s.invalidateBootstrapCache()
			return
		}
	}
}

func (s *Service) fileWriteStateLocked(path string, maxBytes int64) (FileWriteState, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileWriteState{}, nil
		}
		return FileWriteState{}, fmt.Errorf("stat desktop file: %w", err)
	}
	if info.IsDir() {
		return FileWriteState{}, fmt.Errorf("desktop path is a directory")
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return FileWriteState{}, fmt.Errorf("desktop file exceeds max size")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FileWriteState{}, fmt.Errorf("read desktop file: %w", err)
	}
	return FileWriteState{
		Data:   data,
		Entry:  s.workspaceFileEntry(path, info),
		Exists: true,
	}, nil
}

// CreateDirectory creates a workspace directory and any missing parents.
func (s *Service) CreateDirectory(ctx context.Context, rawPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if _, _, _, ok, err := s.resolveMediaMount(rawPath); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("desktop media mount directories are managed by AuraGo")
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create desktop directory: %w", err)
	}
	_ = s.Audit(ctx, "create_directory", s.relativePath(path), map[string]interface{}{}, source)
	s.invalidateBootstrapCacheForFileMutation(s.relativePath(path))
	return nil
}

// MovePath renames or moves a workspace file or directory.
func (s *Service) MovePath(ctx context.Context, oldPath, newPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if fromMount, from, fromRel, fromOK, err := s.resolveMediaMount(oldPath); fromOK || err != nil {
		if err != nil {
			return err
		}
		if strings.TrimSpace(fromRel) == "" {
			return fmt.Errorf("cannot move desktop media mount root")
		}
		toMount, to, toRel, toOK, err := s.resolveMediaMount(newPath)
		if err != nil {
			return err
		}
		if !toOK || !strings.EqualFold(fromMount.Name, toMount.Name) {
			return fmt.Errorf("desktop media paths cannot move across mounts")
		}
		if strings.TrimSpace(toRel) == "" {
			return fmt.Errorf("desktop media destination must be inside the mount")
		}
		if strings.EqualFold(from, to) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return fmt.Errorf("create desktop media destination directory: %w", err)
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("move desktop media path: %w", err)
		}
		s.updateMediaRegistriesAfterMove(ctx, fromMount, from, to)
		_ = s.Audit(ctx, "move_path", mediaDesktopPath(fromMount, fromRel), map[string]interface{}{"new_path": mediaDesktopPath(toMount, toRel)}, source)
		return nil
	}
	if _, _, _, toOK, err := s.resolveMediaMount(newPath); err != nil {
		return err
	} else if toOK {
		return fmt.Errorf("desktop workspace paths cannot move into media mounts")
	}
	from, err := s.ResolvePath(oldPath)
	if err != nil {
		return err
	}
	to, err := s.ResolvePath(newPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(from, to) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("create desktop destination directory: %w", err)
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("move desktop path: %w", err)
	}
	_ = s.Audit(ctx, "move_path", s.relativePath(from), map[string]interface{}{"new_path": s.relativePath(to)}, source)
	s.invalidateBootstrapCacheForFileMutation(s.relativePath(from), s.relativePath(to))
	return nil
}

// CopyPath copies a workspace file or directory to a new location.
func (s *Service) CopyPath(ctx context.Context, srcPath, dstPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	// Handle media mount paths
	if fromMount, from, fromRel, fromOK, err := s.resolveMediaMount(srcPath); fromOK || err != nil {
		if err != nil {
			return err
		}
		if strings.TrimSpace(fromRel) == "" {
			return fmt.Errorf("cannot copy desktop media mount root")
		}
		toMount, to, toRel, toOK, err := s.resolveMediaMount(dstPath)
		if err != nil {
			return err
		}
		if !toOK || !strings.EqualFold(fromMount.Name, toMount.Name) {
			return fmt.Errorf("desktop media paths cannot copy across mounts")
		}
		if strings.TrimSpace(toRel) == "" {
			return fmt.Errorf("desktop media destination must be inside the mount")
		}
		if strings.EqualFold(from, to) {
			return fmt.Errorf("source and destination are the same")
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return fmt.Errorf("create desktop media destination directory: %w", err)
		}
		if err := copyPath(from, to); err != nil {
			return fmt.Errorf("copy desktop media path: %w", err)
		}
		_ = s.Audit(ctx, "copy_path", mediaDesktopPath(fromMount, fromRel), map[string]interface{}{"new_path": mediaDesktopPath(toMount, toRel)}, source)
		return nil
	}
	if _, _, _, toOK, err := s.resolveMediaMount(dstPath); err != nil {
		return err
	} else if toOK {
		return fmt.Errorf("desktop workspace paths cannot copy into media mounts")
	}
	from, err := s.ResolvePath(srcPath)
	if err != nil {
		return err
	}
	to, err := s.ResolvePath(dstPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(from, to) {
		return fmt.Errorf("source and destination are the same")
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("create desktop destination directory: %w", err)
	}
	if err := copyPath(from, to); err != nil {
		return fmt.Errorf("copy desktop path: %w", err)
	}
	_ = s.Audit(ctx, "copy_path", s.relativePath(from), map[string]interface{}{"new_path": s.relativePath(to)}, source)
	s.invalidateBootstrapCacheForFileMutation(s.relativePath(from), s.relativePath(to))
	return nil
}

// copyPath copies a file or directory tree from src to dst.
func copyPath(src, dst string) error {
	stats := &desktopCopyStats{}
	return copyPathLimited(src, dst, 0, stats)
}

func copyPathLimited(src, dst string, depth int, stats *desktopCopyStats) error {
	if depth > desktopCopyMaxDepth {
		return fmt.Errorf("copy exceeds maximum directory depth of %d", desktopCopyMaxDepth)
	}
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("copying symlinks is not supported")
	}
	stats.entries++
	if stats.entries > desktopCopyMaxEntries {
		return fmt.Errorf("copy exceeds maximum entry count of %d", desktopCopyMaxEntries)
	}
	if info.IsDir() {
		return copyDirLimited(src, dst, depth, stats)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("copy source is not a regular file")
	}
	stats.bytes += info.Size()
	if stats.bytes > desktopCopyMaxBytes {
		return fmt.Errorf("copy exceeds maximum size of %d bytes", desktopCopyMaxBytes)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("copy source is not a regular file")
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	_ = os.Chmod(dst, info.Mode())
	_ = os.Chtimes(dst, info.ModTime(), info.ModTime())
	return nil
}

func copyDirLimited(src, dst string, depth int, stats *desktopCopyStats) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := copyPathLimited(srcPath, dstPath, depth+1, stats); err != nil {
			return err
		}
	}
	return nil
}

// DeletePath removes a workspace file or directory tree.
func (s *Service) DeletePath(ctx context.Context, rawPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if mount, path, rel, ok, err := s.resolveMediaMount(rawPath); ok || err != nil {
		if err != nil {
			return err
		}
		if strings.TrimSpace(rel) == "" {
			return fmt.Errorf("cannot delete desktop media mount root")
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete desktop media path: %w", err)
		}
		s.softDeleteMediaRegistries(ctx, mount, path)
		_ = s.Audit(ctx, "delete_path", mediaDesktopPath(mount, rel), map[string]interface{}{}, source)
		return nil
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return err
	}
	if path == s.Config().WorkspaceDir {
		return fmt.Errorf("cannot delete desktop workspace root")
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete desktop path: %w", err)
	}
	_ = s.Audit(ctx, "delete_path", s.relativePath(path), map[string]interface{}{}, source)
	s.invalidateBootstrapCacheForFileMutation(s.relativePath(path))
	return nil
}

// SearchFiles performs a recursive case-insensitive search for files and directories matching the query.
func (s *Service) SearchFiles(ctx context.Context, rawPath, query string) ([]FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	dirPath, err := s.ResolvePath(rawPath)
	if err != nil {
		return nil, err
	}
	searchTerm := strings.ToLower(strings.TrimSpace(query))
	var result []FileEntry
	err = filepath.WalkDir(dirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable files/folders
		}
		if path == dirPath {
			return nil
		}
		name := entry.Name()
		if searchTerm != "" && !strings.Contains(strings.ToLower(name), searchTerm) {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		itemType := "file"
		if info.IsDir() {
			itemType = "directory"
		}
		
		result = append(result, FileEntry{
			Name:      name,
			Path:      s.relativePath(path),
			Type:      itemType,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			MIMEType:  MIMETypeForName(name),
			MediaKind: desktopMediaKindForName(name),
			Mode:      info.Mode().String(),
			Created:   getCreationTime(info),
		})
		
		if len(result) >= 1000 {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil && err != filepath.SkipDir {
		return nil, err
	}
	sortFileEntries(result)
	return result, nil
}

// CreateSymlink creates a symbolic link at linkPath pointing to targetPath.
func (s *Service) CreateSymlink(ctx context.Context, targetPath, linkPath string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()

	resolvedLink, err := s.ResolvePath(linkPath)
	if err != nil {
		return fmt.Errorf("resolve symlink destination: %w", err)
	}

	resolvedTarget, err := s.ResolvePath(targetPath)
	if err != nil {
		return fmt.Errorf("resolve symlink target: %w", err)
	}

	relTarget, err := filepath.Rel(filepath.Dir(resolvedLink), resolvedTarget)
	if err != nil {
		return fmt.Errorf("calculate relative path for symlink: %w", err)
	}

	if _, err := os.Lstat(resolvedLink); err == nil {
		return fmt.Errorf("file or directory already exists at link destination")
	}

	if err := os.Symlink(relTarget, resolvedLink); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	s.invalidateBootstrapCacheForFileMutation(s.relativePath(resolvedLink))
	return nil
}

// GetDirectorySize calculates the recursive size of a directory in bytes.
func (s *Service) GetDirectorySize(ctx context.Context, rawPath string) (int64, error) {
	if err := s.ensureReady(ctx); err != nil {
		return 0, err
	}
	dirPath, err := s.ResolvePath(rawPath)
	if err != nil {
		return 0, err
	}
	var totalSize int64
	err = filepath.WalkDir(dirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable files/folders
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize, err
}


