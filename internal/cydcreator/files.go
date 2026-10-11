package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"aurago/internal/fileutil"
)

// AppDir holds the app sources the user and the agent own.
const AppDir = "src/app"

// FileInfo is one entry of GET /projects/{id}/files.
type FileInfo struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Editable  bool   `json:"editable"`
	Generated bool   `json:"generated"`
}

// listAppFiles lists the editable files below src/app with sizes and hashes
// (hash "" for files above MaxAppFileBytes) and their total size.
func listAppFiles(root *os.Root) ([]FileInfo, int64, error) {
	var out []FileInfo
	var total int64
	err := fs.WalkDir(root.FS(), AppDir, func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && rel == AppDir {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() || !IsEditablePath(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		entry := FileInfo{Path: rel, Size: info.Size(), Editable: true}
		if info.Size() <= MaxAppFileBytes {
			if data, err := rootReadFile(root, rel, MaxAppFileBytes); err == nil {
				entry.SHA256 = sha256Hex(data)
			}
		}
		total += info.Size()
		out = append(out, entry)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, 0, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, total, nil
}

// readAppFiles returns the content of every editable app file. A file above
// MaxAppFileBytes fails with CYD_FILE_LIMIT.
func readAppFiles(root *os.Root) (map[string][]byte, error) {
	list, _, err := listAppFiles(root)
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(list))
	for _, f := range list {
		data, err := rootReadFile(root, f.Path, MaxAppFileBytes)
		if errors.Is(err, errFileTooLarge) {
			return nil, NewError(CodeFileLimit, fmt.Sprintf("%s is larger than %d bytes", f.Path, MaxAppFileBytes),
				map[string]any{"limit": "file_bytes", "max": MaxAppFileBytes, "path": f.Path})
		}
		if err != nil {
			return nil, err
		}
		files[f.Path] = data
	}
	return files, nil
}

// readonlyPathError explains why rel cannot be written.
func readonlyPathError(rel string) error {
	if IsGeneratedPath(rel) || rel == ProjectFile || strings.HasPrefix(rel, "assets/") {
		return NewError(CodeFileReadOnly, "this file is generated from the layout or managed by CYD Creator; change it in the designer",
			map[string]any{"path": rel})
	}
	return NewError(CodeBadRequest, "only .cpp, .h, .hpp and .c files below src/app/ can be written",
		map[string]any{"path": rel})
}

// ListFiles returns the app files plus the generated paths of an on-demand
// generation. When the layout cannot be generated, only app files are listed.
func (s *Service) ListFiles(ctx context.Context, id string) ([]FileInfo, error) {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return nil, s.internalError("list files", err)
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return nil, s.internalError("list files", err)
	}
	defer root.Close()
	files, _, err := listAppFiles(root)
	if err != nil {
		return nil, s.internalError("list files", err)
	}
	if out, err := s.generateFromRoot(row, root); err == nil {
		for rel, data := range out.Files {
			if IsGeneratedPath(rel) {
				files = append(files, FileInfo{Path: rel, Size: int64(len(data)), SHA256: sha256Hex(data), Generated: true})
			}
		}
	} else {
		s.logger.Debug("CYD Creator file list without generated files", "project_id", id, "error", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if files == nil {
		files = []FileInfo{}
	}
	return files, nil
}

// ReadFile returns an app file or a generated file (rendered on demand) and
// its SHA-256.
func (s *Service) ReadFile(ctx context.Context, id, rel string) ([]byte, string, error) {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return nil, "", s.internalError("read file", err)
	}
	notFound := NewError(CodeNotFound, "file not found", map[string]any{"path": rel})
	root, err := s.openProjectRoot(row)
	if err != nil {
		return nil, "", s.internalError("read file", err)
	}
	defer root.Close()
	switch {
	case IsEditablePath(rel):
		data, err := rootReadFile(root, rel, MaxAppFileBytes)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", notFound
		}
		if errors.Is(err, errFileTooLarge) {
			return nil, "", NewError(CodeFileLimit, fmt.Sprintf("%s is larger than %d bytes", rel, MaxAppFileBytes),
				map[string]any{"limit": "file_bytes", "max": MaxAppFileBytes, "path": rel})
		}
		if err != nil {
			return nil, "", s.internalError("read file", err)
		}
		return data, sha256Hex(data), nil
	case IsGeneratedPath(rel):
		out, err := s.generateFromRoot(row, root)
		if err != nil {
			return nil, "", err
		}
		data, ok := out.Files[rel]
		if !ok {
			return nil, "", notFound
		}
		return data, sha256Hex(data), nil
	}
	return nil, "", notFound
}

// currentAppFile returns the content and hash of rel, or exists=false.
func currentAppFile(root *os.Root, rel string) (data []byte, sha string, exists bool, err error) {
	data, err = rootReadFile(root, rel, MaxAppTotalBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	return data, sha256Hex(data), true, nil
}

// WriteFile writes an app file. ifSHA is "*" (create only), the SHA-256 of
// the current content, or "" (unconditional; agent tools). User writes set
// app_changed_since_job.
func (s *Service) WriteFile(ctx context.Context, id, rel string, data []byte, ifSHA string, origin string) (string, error) {
	if !IsEditablePath(rel) {
		return "", readonlyPathError(rel)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", NewError(CodeBadRequest, "source files must be UTF-8 text", map[string]any{"path": rel})
	}
	if len(data) > MaxAppFileBytes {
		return "", NewError(CodeFileLimit, fmt.Sprintf("a source file may have at most %d bytes", MaxAppFileBytes),
			map[string]any{"limit": "file_bytes", "max": MaxAppFileBytes, "path": rel})
	}
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	if err := checkLock(row, origin); err != nil {
		return "", err
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	defer root.Close()
	files, total, err := listAppFiles(root)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	// Check the letter-case clash before looking the file up: on case-insensitive file
	// systems (Windows, macOS) opening src/app/APP_MAIN.cpp finds app_main.cpp, so the
	// write would silently overwrite the other file.
	for _, f := range files {
		if f.Path != rel && strings.EqualFold(f.Path, rel) {
			return "", NewError(CodeFileConflict, "a file with the same name in different letter case exists",
				map[string]any{"path": rel, "existing": f.Path, "sha256": ""})
		}
	}
	old, current, exists, err := currentAppFile(root, rel)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	switch {
	case ifSHA == "*" && exists, ifSHA != "" && ifSHA != "*" && (!exists || ifSHA != current):
		return "", NewError(CodeFileConflict, "the file was changed in the meantime", map[string]any{"path": rel, "sha256": current})
	}
	if !exists && len(files)+1 > MaxAppFiles {
		return "", NewError(CodeFileLimit, fmt.Sprintf("a project may have at most %d source files", MaxAppFiles),
			map[string]any{"limit": "files", "max": MaxAppFiles, "path": rel})
	}
	if total-int64(len(old))+int64(len(data)) > MaxAppTotalBytes {
		return "", NewError(CodeFileLimit, fmt.Sprintf("all source files together may have at most %d bytes", MaxAppTotalBytes),
			map[string]any{"limit": "total_bytes", "max": MaxAppTotalBytes, "path": rel})
	}
	if err := rootWriteFile(ctx, root, rel, data); err != nil {
		return "", s.internalError("write file", err)
	}
	if err := s.touchAppFiles(ctx, row.ID, origin); err != nil {
		return "", s.internalError("write file", err)
	}
	s.Emit(id, EventProjectChanged, jobIDOf(origin), "", projectChange(row.Version, origin, "files"))
	return sha256Hex(data), nil
}

// DeleteFile removes an app file. ifSHA "" or "*" deletes unconditionally.
func (s *Service) DeleteFile(ctx context.Context, id, rel, ifSHA string, origin string) error {
	if !IsEditablePath(rel) {
		return readonlyPathError(rel)
	}
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return s.internalError("delete file", err)
	}
	if err := checkLock(row, origin); err != nil {
		return err
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return s.internalError("delete file", err)
	}
	defer root.Close()
	_, current, exists, err := currentAppFile(root, rel)
	if err != nil {
		return s.internalError("delete file", err)
	}
	if !exists {
		return NewError(CodeNotFound, "file not found", map[string]any{"path": rel})
	}
	if ifSHA != "" && ifSHA != "*" && ifSHA != current {
		return NewError(CodeFileConflict, "the file was changed in the meantime", map[string]any{"path": rel, "sha256": current})
	}
	if err := fileutil.PublishContext(ctx, func() error { return root.Remove(filepath.FromSlash(rel)) }); err != nil {
		return s.internalError("delete file", err)
	}
	for dir := path.Dir(rel); dir != AppDir && strings.HasPrefix(dir, AppDir+"/"); dir = path.Dir(dir) {
		if root.Remove(filepath.FromSlash(dir)) != nil {
			break // not empty
		}
	}
	if err := s.touchAppFiles(ctx, row.ID, origin); err != nil {
		return s.internalError("delete file", err)
	}
	s.Emit(id, EventProjectChanged, jobIDOf(origin), "", projectChange(row.Version, origin, "files"))
	return nil
}

// touchAppFiles records an app file change; user changes set
// app_changed_since_job so the next agent job learns about them.
func (s *Service) touchAppFiles(ctx context.Context, id, origin string) error {
	now := formatTime(s.now())
	if origin == OriginUser {
		_, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET app_changed_since_job=1, updated_at=? WHERE id=?`, now, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET updated_at=? WHERE id=?`, now, id)
	return err
}
