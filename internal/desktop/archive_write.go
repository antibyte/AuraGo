package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

const archiveExpansionLimit = int64(500 << 20)

// ArchiveEntryLimit bounds desktop archive entries across listing and extraction.
const ArchiveEntryLimit = 10000

func validArchivePath(name string) bool {
	name = strings.TrimSuffix(name, "/")
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, ". ") != part {
			return false
		}
	}
	return true
}

// CreateArchive snapshots rooted regular files before publishing the complete ZIP.
func (s *Service) CreateArchive(ctx context.Context, paths []string, dest, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	root, err := os.OpenRoot(s.Config().WorkspaceDir)
	if err != nil {
		return err
	}
	defer root.Close()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	count := 0
	remaining := archiveExpansionLimit
	seen := map[string]bool{}
	for _, raw := range paths {
		resolved, err := s.resolveWorkspacePathNoSymlinks(raw, false)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.Config().WorkspaceDir, resolved)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		base := path.Dir(rel)
		err = fs.WalkDir(root.FS(), rel, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("archive source is not a regular file")
			}
			zipName := strings.TrimPrefix(name, base+"/")
			if base == "." {
				zipName = name
			}
			if !validArchivePath(zipName) || seen[zipName] {
				return fmt.Errorf("invalid or duplicate archive path")
			}
			seen[zipName] = true
			count++
			if count > ArchiveEntryLimit || info.Size() > remaining {
				return fmt.Errorf("archive exceeds limits")
			}
			stream, err := root.Open(name)
			if err != nil {
				return err
			}
			opened, err := stream.Stat()
			if err != nil || !opened.Mode().IsRegular() {
				stream.Close()
				return fmt.Errorf("archive source changed")
			}
			output, err := writer.Create(zipName)
			if err != nil {
				stream.Close()
				return err
			}
			n, copyErr := io.Copy(output, io.LimitReader(stream, remaining+1))
			closeErr := stream.Close()
			remaining -= n
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if remaining < 0 {
				return fmt.Errorf("archive exceeds limits")
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	err = s.WriteFileBytes(ctx, dest, buffer.Bytes(), source)
	return err
}

// ExtractArchive validates the entire expanded package before any destination write.
func (s *Service) ExtractArchive(ctx context.Context, archive, dest, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	src, err := s.resolveWorkspacePathNoSymlinks(archive, false)
	if err != nil {
		return err
	}
	dst, err := s.resolveWorkspacePathNoSymlinks(dest, true)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.Config().WorkspaceDir)
	if err != nil {
		return err
	}
	defer root.Close()
	rel, _ := filepath.Rel(s.Config().WorkspaceDir, src)
	input, err := root.Open(rel)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("archive is not a regular file")
	}
	reader, err := zip.NewReader(input, info.Size())
	if err != nil {
		return err
	}
	if len(reader.File) > ArchiveEntryLimit {
		return fmt.Errorf("archive contains too many entries")
	}
	type preparedEntry struct {
		path      string
		data      []byte
		directory bool
	}
	prepared := make([]preparedEntry, 0, len(reader.File))
	remaining := archiveExpansionLimit
	seen := map[string]bool{}
	maxBytes := int64(s.Config().MaxFileSizeMB) << 20
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !validArchivePath(file.Name) || (!file.Mode().IsRegular() && !file.FileInfo().IsDir()) {
			return fmt.Errorf("invalid archive entry")
		}
		key := file.Name
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			return fmt.Errorf("duplicate archive entry")
		}
		seen[key] = true
		target := filepath.Join(dst, filepath.FromSlash(file.Name))
		if !isWithinPath(dst, target) {
			return fmt.Errorf("archive path escapes destination")
		}
		entry := preparedEntry{path: target, directory: file.FileInfo().IsDir()}
		if !entry.directory {
			budget := min(remaining, maxBytes)
			if file.UncompressedSize64 > uint64(budget) {
				return fmt.Errorf("archive expanded size exceeds limit")
			}
			stream, err := file.Open()
			if err != nil {
				return err
			}
			data, readErr := io.ReadAll(io.LimitReader(stream, budget+1))
			closeErr := stream.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if int64(len(data)) > budget {
				return fmt.Errorf("archive expanded size exceeds limit")
			}
			entry.data = data
			remaining -= int64(len(data))
		}
		prepared = append(prepared, entry)
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	defer s.invalidateListCache()
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	// Preflight every destination and policy before publishing the first file.
	planned := make(map[string]bool, len(prepared))
	for _, entry := range prepared {
		key := filepath.Clean(entry.path)
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if existingDirectory, exists := planned[key]; exists {
			if existingDirectory != entry.directory {
				return fmt.Errorf("archive destination type conflict")
			}
			return fmt.Errorf("archive destination path collision")
		}
		planned[key] = entry.directory
	}
	for _, entry := range prepared {
		for parent := filepath.Dir(entry.path); isWithinPath(dst, parent); parent = filepath.Dir(parent) {
			key := filepath.Clean(parent)
			if runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			if directory, exists := planned[key]; exists && !directory {
				return fmt.Errorf("archive file conflicts with a directory")
			}
			if info, err := os.Lstat(parent); err == nil && !info.IsDir() {
				return fmt.Errorf("archive parent is not a directory")
			} else if err != nil && !os.IsNotExist(err) {
				return err
			}
			if parent == filepath.Clean(dst) || parent == filepath.Dir(parent) {
				break
			}
		}
		if info, err := os.Lstat(entry.path); err == nil && info.IsDir() != entry.directory {
			return fmt.Errorf("archive destination type conflict")
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if _, err := s.resolveWorkspacePathNoSymlinks(entry.path, true); err != nil {
			return err
		}
		if err := s.guardNoteWrite(entry.path, source, entry.data); err != nil {
			return err
		}
	}
	for _, entry := range prepared {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.directory {
			rel, _ := filepath.Rel(s.Config().WorkspaceDir, entry.path)
			if err := root.MkdirAll(rel, 0700); err != nil {
				return err
			}
			continue
		}
		if _, err := s.writeFileBytesLocked(ctx, entry.path, entry.data, source, nil, maxBytes); err != nil {
			return err
		}
	}
	return nil
}
