package desktop

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrArchiveEntryNotFound is returned when the requested zip member is missing.
	ErrArchiveEntryNotFound = errors.New("archive entry not found")
	// ErrArchiveEntryUnsafe is returned for zip-slip or otherwise illegal member names.
	ErrArchiveEntryUnsafe = errors.New("illegal archive entry path")
	// ErrArchiveEntryTooLarge is returned when a member exceeds the desktop size cap.
	ErrArchiveEntryTooLarge = errors.New("archive entry exceeds max size")
	// ErrArchiveEntryNotPreviewable is returned for executables and other blocked types.
	ErrArchiveEntryNotPreviewable = errors.New("archive entry type is not previewable")
	// ErrArchiveEntryIsDirectory is returned when the member is a directory.
	ErrArchiveEntryIsDirectory = errors.New("archive entry is a directory")
)

var previewableArchiveExts = map[string]struct{}{
	".txt": {}, ".md": {}, ".log": {}, ".json": {}, ".xml": {}, ".yaml": {}, ".yml": {},
	".toml": {}, ".ini": {}, ".cfg": {}, ".conf": {}, ".csv": {}, ".html": {}, ".htm": {},
	".css": {}, ".js": {}, ".mjs": {}, ".ts": {}, ".tsx": {}, ".jsx": {}, ".go": {},
	".py": {}, ".sh": {}, ".ps1": {}, ".sql": {},
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".bmp": {},
	".svg": {}, ".ico": {}, ".avif": {}, ".tif": {}, ".tiff": {},
	".pdf":  {},
	".docx": {}, ".xlsx": {}, ".xlsm": {},
	".stl": {},
	".mp3": {}, ".mp4": {}, ".m4a": {}, ".webm": {}, ".ogg": {}, ".opus": {},
	".wav": {}, ".flac": {}, ".mkv": {}, ".mov": {},
}

// ArchiveEntry is one previewable zip member and its bytes.
type ArchiveEntry struct {
	Name     string
	Size     int64
	ModTime  time.Time
	MIMEType string
	Data     []byte
}

// NormalizeArchiveEntryName validates a zip member path and returns a clean slash form.
func NormalizeArchiveEntryName(raw string) (string, error) {
	name := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	name = strings.TrimPrefix(name, "./")
	if name == "" {
		return "", ErrArchiveEntryUnsafe
	}
	if path.IsAbs(name) || strings.Contains(name, ":") {
		return "", ErrArchiveEntryUnsafe
	}
	parts := strings.Split(name, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", ErrArchiveEntryUnsafe
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "", ErrArchiveEntryUnsafe
	}
	return strings.Join(clean, "/"), nil
}

// IsPreviewableArchiveEntry reports whether a zip member may be streamed for display.
func IsPreviewableArchiveEntry(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return false
	}
	_, ok := previewableArchiveExts[ext]
	return ok
}

// ReadArchiveEntry reads one previewable file from a workspace zip.
func (s *Service) ReadArchiveEntry(ctx context.Context, rawPath, entryName string) (ArchiveEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return ArchiveEntry{}, err
	}
	normalized, err := NormalizeArchiveEntryName(entryName)
	if err != nil {
		return ArchiveEntry{}, err
	}
	if !IsPreviewableArchiveEntry(normalized) {
		return ArchiveEntry{}, ErrArchiveEntryNotPreviewable
	}

	srcPath, err := s.resolveWorkspacePathNoSymlinks(rawPath, false)
	if err != nil {
		return ArchiveEntry{}, err
	}
	reader, err := zip.OpenReader(srcPath)
	if err != nil {
		return ArchiveEntry{}, fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()

	file, err := findZipFile(reader.File, normalized)
	if err != nil {
		return ArchiveEntry{}, err
	}
	if file.FileInfo().IsDir() || strings.HasSuffix(strings.ReplaceAll(file.Name, "\\", "/"), "/") {
		return ArchiveEntry{}, ErrArchiveEntryIsDirectory
	}

	maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024
	}
	if int64(file.UncompressedSize64) > maxBytes {
		return ArchiveEntry{}, ErrArchiveEntryTooLarge
	}

	rc, err := file.Open()
	if err != nil {
		return ArchiveEntry{}, fmt.Errorf("open zip entry: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return ArchiveEntry{}, fmt.Errorf("read zip entry: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return ArchiveEntry{}, ErrArchiveEntryTooLarge
	}

	modTime := file.Modified
	if modTime.IsZero() {
		modTime = file.FileInfo().ModTime()
	}
	return ArchiveEntry{
		Name:     path.Base(normalized),
		Size:     int64(len(data)),
		ModTime:  modTime,
		MIMEType: MIMETypeForName(normalized),
		Data:     data,
	}, nil
}

func findZipFile(files []*zip.File, want string) (*zip.File, error) {
	for _, file := range files {
		name, err := NormalizeArchiveEntryName(strings.TrimSuffix(file.Name, "/"))
		if err != nil {
			continue
		}
		if name == want {
			return file, nil
		}
	}
	return nil, ErrArchiveEntryNotFound
}
