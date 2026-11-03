package desktop

import (
	"archive/zip"
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

// CreateArchive snapshots rooted regular files before publishing one archive.
// ZIP is the default. tar, tar.gz, tar.xz and tar.zst follow the destination
// name. Other archive extensions are refused instead of being written as ZIP.
func (s *Service) CreateArchive(ctx context.Context, paths []string, dest, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	kind := archiveKindForCreate(dest)
	if !kind.creatable() {
		return fmt.Errorf("creating %s archives is not supported", kind)
	}
	root, err := os.OpenRoot(s.Config().WorkspaceDir)
	if err != nil {
		return err
	}
	defer root.Close()
	files, err := s.collectArchiveSources(ctx, root, paths)
	if err != nil {
		return err
	}
	var payload []byte
	switch kind {
	case archiveTar, archiveTarGz, archiveTarXz, archiveTarZst:
		payload, err = encodeTar(files, kind)
	default:
		payload, err = encodeZip(files)
	}
	if err != nil {
		return err
	}
	return s.WriteFileBytes(ctx, dest, payload, source)
}

func (s *Service) collectArchiveSources(ctx context.Context, root *os.Root, paths []string) ([]archiveSourceFile, error) {
	var files []archiveSourceFile
	count := 0
	remaining := archiveExpansionLimit
	seen := map[string]bool{}
	for _, raw := range paths {
		resolved, err := s.resolveWorkspacePathNoSymlinks(raw, false)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(s.Config().WorkspaceDir, resolved)
		if err != nil {
			return nil, err
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
			archiveName := strings.TrimPrefix(name, base+"/")
			if base == "." {
				archiveName = name
			}
			if !validArchivePath(archiveName) || seen[archiveName] {
				return fmt.Errorf("invalid or duplicate archive path")
			}
			seen[archiveName] = true
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
			data, readErr := io.ReadAll(io.LimitReader(stream, remaining+1))
			closeErr := stream.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if int64(len(data)) > remaining {
				return fmt.Errorf("archive exceeds limits")
			}
			remaining -= int64(len(data))
			files = append(files, archiveSourceFile{name: archiveName, data: data})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
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
	kind, err := detectArchiveKind(filepath.Base(src), io.NewSectionReader(input, 0, info.Size()))
	if err != nil {
		return err
	}
	if kind != archiveZip {
		if kind == archiveUnknown {
			return errArchiveUnsupported
		}
		if kind.native() {
			return s.extractNative(ctx, kind, filepath.Base(src), io.NewSectionReader(input, 0, info.Size()), dst, source, root)
		}
		input.Close()
		return s.extractHost(ctx, kind, src, dst, source, root)
	}
	reader, err := zip.NewReader(input, info.Size())
	if err != nil {
		return err
	}
	if len(reader.File) > ArchiveEntryLimit {
		return fmt.Errorf("archive contains too many entries")
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
	return s.publishPreparedArchive(ctx, root, dst, source, prepared, maxBytes)
}

type preparedEntry struct {
	path      string
	data      []byte
	directory bool
}

func (s *Service) publishMembers(ctx context.Context, kind archiveKind, members []archiveMember, dst, source string, root *os.Root) error {
	maxBytes := int64(s.Config().MaxFileSizeMB) << 20
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	prepared, err := stageArchiveMembers(dst, members, kind.filesystem(), maxBytes)
	if err != nil {
		return err
	}
	return s.publishPreparedArchive(ctx, root, dst, source, prepared, maxBytes)
}

func stageArchiveMembers(dst string, members []archiveMember, filesystem bool, maxBytes int64) ([]preparedEntry, error) {
	prepared := make([]preparedEntry, 0, len(members))
	remaining := archiveExpansionLimit
	seen := map[string]bool{}
	for _, member := range members {
		if member.Link || member.Special {
			if filesystem {
				continue
			}
			return nil, fmt.Errorf("invalid archive entry")
		}
		name, skip, err := cleanMemberName(member.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		key := name
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if isDir, exists := seen[key]; exists {
			if isDir && member.IsDir {
				continue
			}
			return nil, fmt.Errorf("duplicate archive entry")
		}
		seen[key] = member.IsDir
		target := filepath.Join(dst, filepath.FromSlash(name))
		if !isWithinPath(dst, target) {
			return nil, fmt.Errorf("archive path escapes destination")
		}
		entry := preparedEntry{path: target, directory: member.IsDir}
		if !member.IsDir {
			budget := min(remaining, maxBytes)
			if member.Size > budget {
				return nil, fmt.Errorf("archive expanded size exceeds limit")
			}
			if member.Read == nil {
				return nil, fmt.Errorf("invalid archive entry")
			}
			data, err := member.Read(budget)
			if err != nil {
				return nil, err
			}
			if int64(len(data)) > budget {
				return nil, fmt.Errorf("archive expanded size exceeds limit")
			}
			entry.data = data
			remaining -= int64(len(data))
		}
		prepared = append(prepared, entry)
	}
	return prepared, nil
}

func (s *Service) publishPreparedArchive(ctx context.Context, root *os.Root, dst, source string, prepared []preparedEntry, maxBytes int64) error {
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	defer s.invalidateListCache()
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
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
