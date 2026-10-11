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
		onDisk, err := appPathOnDisk(root, rel)
		if err != nil {
			return nil, "", s.internalError("read file", err)
		}
		if onDisk != rel {
			return nil, "", notFound
		}
		data, err := rootReadFile(root, rel, MaxAppFileBytes)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", notFound
		}
		if errors.Is(err, errNotRegularFile) {
			return nil, "", notRegularError(rel)
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

// notRegularError refuses a path below src/app that holds a directory, link
// or device instead of a file (only possible by hand). Nothing is removed or
// replaced there.
func notRegularError(rel string) error {
	return NewError(CodeBadRequest, "this path is not a regular file", map[string]any{"path": rel, "reason": "not_regular"})
}

// appPathOnDisk resolves rel segment by segment below src/app with the exact
// letter case. It returns "" when rel does not exist, rel when every segment
// exists exactly, and otherwise the existing path prefix whose last segment
// differs from rel only by letter case. On case-insensitive file systems
// (Windows, macOS) opening src/app/APP_MAIN.cpp finds app_main.cpp, so every
// read, write and delete of an app path checks the case first.
func appPathOnDisk(root *os.Root, rel string) (string, error) {
	rest, ok := strings.CutPrefix(rel, AppDir+"/")
	if !ok {
		return "", fmt.Errorf("%s is not below %s", rel, AppDir)
	}
	segments := strings.Split(rest, "/")
	dir := AppDir
	for i, seg := range segments {
		entries, err := fs.ReadDir(root.FS(), dir)
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		var exact, folded fs.DirEntry
		for _, e := range entries {
			if e.Name() == seg {
				exact = e
				break
			}
			if folded == nil && strings.EqualFold(e.Name(), seg) {
				folded = e
			}
		}
		switch {
		case exact != nil:
			dir += "/" + seg
			if i < len(segments)-1 && !exact.IsDir() {
				return "", nil // a file where rel needs a directory
			}
		case folded != nil:
			return dir + "/" + folded.Name(), nil
		default:
			return "", nil
		}
	}
	return dir, nil
}

// appFileState is what an editable path holds on disk.
type appFileState struct {
	exists  bool   // an entry with exactly this path, letter case included
	alias   string // an existing file or folder whose path differs only by letter case
	regular bool   // the entry is a regular file
	size    int64  // size of a regular file
	sha     string // "" for files above MaxAppFileBytes (ListFiles and ReadFile never expose one) and other entries
}

// currentAppFile reports what rel holds on disk. A file above
// MaxAppFileBytes (put there by hand) exists without a hash, so only an
// unconditional write or delete replaces it.
func currentAppFile(root *os.Root, rel string) (appFileState, error) {
	onDisk, err := appPathOnDisk(root, rel)
	if err != nil || onDisk == "" {
		return appFileState{}, err
	}
	if onDisk != rel {
		return appFileState{alias: onDisk}, nil
	}
	info, err := root.Lstat(filepath.FromSlash(rel))
	if errors.Is(err, fs.ErrNotExist) {
		return appFileState{}, nil
	}
	if err != nil {
		return appFileState{}, err
	}
	st := appFileState{exists: true, regular: info.Mode().IsRegular()}
	if !st.regular {
		return st, nil
	}
	st.size = info.Size()
	if st.size <= MaxAppFileBytes {
		data, err := rootReadFile(root, rel, MaxAppFileBytes)
		switch {
		case err == nil:
			st.size, st.sha = int64(len(data)), sha256Hex(data)
		case !errors.Is(err, errFileTooLarge):
			return appFileState{}, err
		}
	}
	return st, nil
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
	cur, err := currentAppFile(root, rel)
	if err != nil {
		return "", s.internalError("write file", err)
	}
	if cur.alias != "" {
		return "", NewError(CodeFileConflict, "a file or folder with the same name in different letter case exists",
			map[string]any{"path": rel, "existing": cur.alias, "sha256": ""})
	}
	if cur.exists && !cur.regular {
		return "", notRegularError(rel)
	}
	switch {
	case ifSHA == "*" && cur.exists, ifSHA != "" && ifSHA != "*" && (!cur.exists || ifSHA != cur.sha):
		return "", NewError(CodeFileConflict, "the file was changed in the meantime", map[string]any{"path": rel, "sha256": cur.sha})
	}
	if !cur.exists && len(files)+1 > MaxAppFiles {
		return "", NewError(CodeFileLimit, fmt.Sprintf("a project may have at most %d source files", MaxAppFiles),
			map[string]any{"limit": "files", "max": MaxAppFiles, "path": rel})
	}
	if total-cur.size+int64(len(data)) > MaxAppTotalBytes {
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
	cur, err := currentAppFile(root, rel)
	if err != nil {
		return s.internalError("delete file", err)
	}
	if !cur.exists {
		return NewError(CodeNotFound, "file not found", map[string]any{"path": rel})
	}
	if !cur.regular {
		return notRegularError(rel)
	}
	if ifSHA != "" && ifSHA != "*" && ifSHA != cur.sha {
		return NewError(CodeFileConflict, "the file was changed in the meantime", map[string]any{"path": rel, "sha256": cur.sha})
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
// app_changed_since_job so the next agent job learns about them. The file is
// already published, so a cancelled request still records the change.
func (s *Service) touchAppFiles(ctx context.Context, id, origin string) error {
	ctx = context.WithoutCancel(ctx)
	now := formatTime(s.now())
	if origin == OriginUser {
		_, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET app_changed_since_job=1, updated_at=? WHERE id=?`, now, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET updated_at=? WHERE id=?`, now, id)
	return err
}
