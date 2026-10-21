package tools

import (
	"aurago/internal/fileutil"
	"aurago/internal/security"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxRevisionFileSize  = 500 * 1024
	maxDiffPreviewLength = 3000
)

var revisionExcludedDirs = []string{
	"node_modules", ".next", ".git", "dist", "build", ".output", ".cache", ".temp",
}

func shouldExcludePath(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	if filepath.Base(relPath) == homepageArtifactManifestName || hereNowSensitivePath(relPath) {
		return true
	}
	for _, excluded := range revisionExcludedDirs {
		if strings.Contains(relPath, excluded+"/") || relPath == excluded {
			return true
		}
	}
	return false
}

func hashContent(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

type fileEntry struct {
	Path     string
	Data     []byte
	Size     int64
	Hash     string
	IsBinary bool
	ModTime  time.Time
	Mode     os.FileMode
}

type revisionDelta struct {
	Added    []fileEntry
	Modified []fileEntry
	Deleted  []fileEntry
}

func enumerateProjectFiles(basePath string, logger *slog.Logger) ([]fileEntry, error) {
	root, err := os.OpenRoot(basePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var entries []fileEntry
	contentBytes := 0
	err = filepath.Walk(basePath, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(basePath, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if shouldExcludePath(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported project file %q", rel)
		}
		f, err := root.Open(rel)
		if err != nil {
			return err
		}
		h := sha256.New()
		prefix, err := io.ReadAll(io.LimitReader(io.TeeReader(f, h), maxRevisionFileSize+1))
		if err == nil {
			_, err = io.Copy(h, f)
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		data := prefix
		contentBytes += len(prefix)
		if len(entries) >= 10000 {
			return fmt.Errorf("project exceeds the 10000-file revision inventory limit")
		}
		binary := isBinaryContent(prefix)
		if contentBytes > 64*1024*1024 || len(prefix) > maxRevisionFileSize || security.RedactSensitiveInfo(string(prefix)) != string(prefix) {
			data = nil
		}
		entries = append(entries, fileEntry{Path: rel, Data: data, Size: info.Size(), Hash: hex.EncodeToString(h.Sum(nil)), IsBinary: binary, ModTime: info.ModTime(), Mode: info.Mode().Perm()})
		return nil
	})
	return entries, err
}

func isBinaryContent(data []byte) bool {
	if len(data) < 8000 {
		for _, b := range data {
			if b == 0 {
				return true
			}
		}
		return false
	}
	for i := 0; i < 8000; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func computeDelta(baseline map[string]fileEntry, current []fileEntry) revisionDelta {
	var delta revisionDelta
	currentMap := make(map[string]fileEntry)
	for _, f := range current {
		currentMap[f.Path] = f
	}
	for _, f := range current {
		if base, exists := baseline[f.Path]; exists {
			if base.Hash != f.Hash {
				delta.Modified = append(delta.Modified, f)
			}
		} else {
			delta.Added = append(delta.Added, f)
		}
	}
	for _, f := range baseline {
		if _, exists := currentMap[f.Path]; !exists {
			delta.Deleted = append(delta.Deleted, f)
		}
	}
	return delta
}

func computeDiffText(before, after string, isBinary bool) (beforeText, afterText string) {
	if isBinary {
		return "[binary file]", "[binary file]"
	}
	beforeText = before
	afterText = after
	if len(beforeText) > maxDiffPreviewLength {
		beforeText = beforeText[:maxDiffPreviewLength] + "\n... (truncated)"
	}
	if len(afterText) > maxDiffPreviewLength {
		afterText = afterText[:maxDiffPreviewLength] + "\n... (truncated)"
	}
	return beforeText, afterText
}

type revisionStatus struct {
	ProjectDir    string        `json:"project_dir"`
	HasRevisions  bool          `json:"has_revisions"`
	LatestRevID   int64         `json:"latest_rev_id,omitempty"`
	LatestMessage string        `json:"latest_message,omitempty"`
	UnifiedDelta  revisionDelta `json:"unified_delta"`
}

func computeUnifiedDelta(db *sql.DB, projectDir, basePath string, baseRevID int64, logger *slog.Logger) (*revisionDelta, error) {
	baseline, err := reconstructProjectState(db, projectDir, baseRevID, logger)
	if err != nil {
		return nil, err
	}
	currentFiles, err := enumerateProjectFiles(basePath, logger)
	if err != nil {
		return nil, err
	}
	delta := computeDelta(baseline, currentFiles)
	return &delta, nil
}

func reconstructProjectState(db *sql.DB, projectDir string, baseRevID int64, logger *slog.Logger) (map[string]fileEntry, error) {
	rows, err := db.Query(`SELECT f.path,f.change_type,f.content_after,f.content_hash_after,f.size_after,f.file_mode
        FROM homepage_revision_files f JOIN homepage_revisions r ON r.id=f.revision_id
        WHERE r.project_dir=? AND r.id<=? ORDER BY r.id,f.id`, projectDir, baseRevID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state := make(map[string]fileEntry)
	for rows.Next() {
		var path, change, content, hash string
		var size, mode int64
		if err := rows.Scan(&path, &change, &content, &hash, &size, &mode); err != nil {
			return nil, err
		}
		if shouldExcludePath(path) {
			continue
		}
		if change == "deleted" {
			delete(state, path)
			continue
		}
		state[path] = fileEntry{Path: path, Data: []byte(content), Size: size, Hash: hash, IsBinary: isBinaryContent([]byte(content)), Mode: os.FileMode(mode)}
	}
	return state, rows.Err()
}

func RestoreRevisionFiles(projectDir string, files []HomepageRevisionFile, logger *slog.Logger) (restored []string, warnings []string) {
	root, err := os.OpenRoot(projectDir)
	if err != nil {
		return nil, []string{err.Error()}
	}
	defer root.Close()
	for _, f := range files {
		parent := filepath.Dir(filepath.FromSlash(f.Path))
		for parent != "." {
			if info, err := root.Lstat(parent); err == nil && info.Mode()&os.ModeSymlink != 0 {
				return nil, []string{"restore path traverses a symbolic link"}
			}
			parent = filepath.Dir(parent)
		}
	}
	for _, f := range files {
		if shouldExcludePath(f.Path) || !filepath.IsLocal(filepath.FromSlash(f.Path)) {
			return nil, []string{"unsafe or sensitive restore path"}
		}
		if f.ChangeType != "deleted" && (int64(len(f.ContentAfter)) != f.SizeAfter || hashContent([]byte(f.ContentAfter)) != f.ContentHashAfter) {
			return nil, []string{fmt.Sprintf("incomplete or corrupted revision content for %s", f.Path)}
		}
	}
	for _, f := range files {
		if f.ChangeType == "deleted" {
			err = root.Remove(filepath.FromSlash(f.Path))
			if os.IsNotExist(err) {
				err = nil
			}
		} else {
			err = writeHomepageRevisionFile(context.Background(), root, f.Path, []byte(f.ContentAfter), os.FileMode(f.FileMode))
		}
		if err != nil {
			return restored, []string{fmt.Sprintf("restore %s: %v", f.Path, err)}
		}
		restored = append(restored, f.Path)
	}
	return restored, nil
}

func writeHomepageRevisionFile(ctx context.Context, root *os.Root, path string, data []byte, mode os.FileMode) error {
	path = filepath.FromSlash(path)
	if !filepath.IsLocal(path) {
		return fmt.Errorf("unsafe restore path")
	}
	if err := root.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0644
		if info, err := root.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	}
	temp := filepath.Join(filepath.Dir(path), ".aurago-restore-"+rand.Text())
	f, err := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return fileutil.RenameRootContext(ctx, root, temp, path)
}

func HomepageSaveRevision(cfg HomepageConfig, db *sql.DB, projectDir, message, reason string, logger *slog.Logger) string {
	if db == nil {
		return ErrorJSON("homepage registry DB not initialized")
	}
	if projectDir == "" {
		return ErrorJSON("project_dir is required")
	}
	if err := sanitizeProjectDir(projectDir); err != nil {
		return ErrorJSON(err.Error())
	}
	current, err := enumerateProjectFiles(homepageLocalRoot(cfg, projectDir), logger)
	if err != nil {
		return ErrorJSONf("enumerate project files: %v", err)
	}
	baseline := make(map[string]fileEntry)
	if latest, err := GetLatestHomepageRevision(db, projectDir); err != nil {
		return ErrorJSON(err.Error())
	} else if latest != nil {
		baseline, err = reconstructProjectState(db, projectDir, latest.ID, logger)
		if err != nil {
			return ErrorJSON(err.Error())
		}
	}
	delta := computeDelta(baseline, current)
	changed := len(delta.Added) + len(delta.Modified) + len(delta.Deleted)
	if changed == 0 {
		return `{"status":"ok","message":"no changes since last revision","revision_id":null}`
	}
	var projectID interface{}
	var pid int64
	if proj, err := GetProjectByDir(db, projectDir); err == nil {
		pid = proj.ID
		projectID = pid
	}
	if message == "" {
		message = fmt.Sprintf("Saved %d changed file(s)", changed)
	}
	complete := true
	for _, f := range current {
		if int64(len(f.Data)) != f.Size || hashContent(f.Data) != f.Hash {
			complete = false
		}
	}
	metadata, _ := json.Marshal(map[string]int{"added": len(delta.Added), "modified": len(delta.Modified), "deleted": len(delta.Deleted)})
	tx, err := db.Begin()
	if err != nil {
		return ErrorJSON(err.Error())
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO homepage_revisions (project_id,project_dir,message,reason,author,file_count,restorable,metadata_json)
        VALUES (?,?,?,?,?,?,?,?)`, projectID, projectDir, security.RedactSensitiveInfo(message), security.RedactSensitiveInfo(reason), func() string {
		op := homepageCurrentOperation(cfg)
		if op.Actor != "" {
			return op.Actor
		}
		return "unknown"
	}(), changed, complete, string(metadata))
	if err != nil {
		return ErrorJSON(err.Error())
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ErrorJSON(err.Error())
	}
	for _, group := range []struct {
		kind  string
		files []fileEntry
	}{{"added", delta.Added}, {"modified", delta.Modified}, {"deleted", delta.Deleted}} {
		for _, f := range group.files {
			before := baseline[f.Path]
			after := f
			if group.kind == "deleted" {
				before = f
				after = fileEntry{}
			}
			if _, err := tx.Exec(`INSERT INTO homepage_revision_files
                (revision_id,path,change_type,content_before,content_after,content_hash_before,content_hash_after,size_before,size_after,file_mode)
                VALUES (?,?,?,?,?,?,?,?,?,?)`, id, f.Path, group.kind, string(before.Data), string(after.Data), before.Hash, after.Hash, before.Size, after.Size, int64(f.Mode)); err != nil {
				return ErrorJSON(err.Error())
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return ErrorJSON(err.Error())
	}
	if pid > 0 {
		if err := LogEdit(db, pid, reason); err != nil {
			return ErrorJSONf("revision saved but metadata update failed: %v", err)
		}
	}
	raw, _ := json.Marshal(map[string]interface{}{"status": "ok", "revision_id": id, "message": message, "file_count": changed, "added": len(delta.Added), "modified": len(delta.Modified), "deleted": len(delta.Deleted), "restorable": complete})
	return string(raw)
}

func HomepageListRevisions(db *sql.DB, projectDir string, limit, offset int, logger *slog.Logger) string {
	if db == nil {
		return `{"status":"error","message":"homepage registry DB not initialized"}`
	}
	if projectDir == "" {
		return `{"status":"error","message":"project_dir is required"}`
	}
	if limit <= 0 {
		limit = 20
	}
	revisions, total, err := ListHomepageRevisions(db, projectDir, limit, offset)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	summary := make([]map[string]interface{}, 0, len(revisions))
	for _, r := range revisions {
		summary = append(summary, map[string]interface{}{
			"id":         r.ID,
			"message":    r.Message,
			"reason":     r.Reason,
			"author":     r.Author,
			"file_count": r.FileCount,
			"restorable": r.Restorable,
			"created_at": r.CreatedAt,
		})
	}
	b, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"total":     total,
		"revisions": summary,
	})
	return string(b)
}

func HomepageGetRevision(db *sql.DB, revisionID int64, logger *slog.Logger) string {
	if db == nil {
		return `{"status":"error","message":"homepage registry DB not initialized"}`
	}
	if revisionID <= 0 {
		return `{"status":"error","message":"revision_id is required"}`
	}
	rev, err := GetHomepageRevision(db, revisionID)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	files, err := GetHomepageRevisionFiles(db, revisionID)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	fileList := make([]map[string]interface{}, 0, len(files))
	for _, f := range files {
		fileList = append(fileList, map[string]interface{}{
			"path":        f.Path,
			"change_type": f.ChangeType,
			"size_before": f.SizeBefore,
			"size_after":  f.SizeAfter,
		})
	}
	b, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"revision": rev,
		"files":    fileList,
	})
	return string(b)
}

func HomepageDiffRevision(db *sql.DB, revisionID int64, path string, logger *slog.Logger) string {
	if db == nil {
		return `{"status":"error","message":"homepage registry DB not initialized"}`
	}
	if revisionID <= 0 {
		return `{"status":"error","message":"revision_id is required"}`
	}
	rev, err := GetHomepageRevision(db, revisionID)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	files, err := GetHomepageRevisionFiles(db, revisionID)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	if path != "" {
		var filtered []HomepageRevisionFile
		for _, f := range files {
			if f.Path == path {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}
	safeFiles := files[:0]
	for _, file := range files {
		if !shouldExcludePath(file.Path) {
			safeFiles = append(safeFiles, file)
		}
	}
	files = safeFiles
	diffs := make([]map[string]interface{}, 0, len(files))
	for _, f := range files {
		isBinary := isBinaryContent([]byte(f.ContentBefore)) || isBinaryContent([]byte(f.ContentAfter))
		beforeText, afterText := computeDiffText(security.RedactSensitiveInfo(f.ContentBefore), security.RedactSensitiveInfo(f.ContentAfter), isBinary)
		diffs = append(diffs, map[string]interface{}{
			"path":           f.Path,
			"change_type":    f.ChangeType,
			"content_before": beforeText,
			"content_after":  afterText,
			"is_binary":      isBinary,
			"size_before":    f.SizeBefore,
			"size_after":     f.SizeAfter,
		})
	}
	b, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"revision":   rev,
		"diffs":      diffs,
		"diff_count": len(diffs),
	})
	return string(b)
}

func HomepageRestoreRevision(cfg HomepageConfig, db *sql.DB, revisionID int64, path string, logger *slog.Logger) string {
	if db == nil {
		return ErrorJSON("homepage registry DB not initialized")
	}
	rev, err := GetHomepageRevision(db, revisionID)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	target, err := reconstructProjectState(db, rev.ProjectDir, rev.ID, logger)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	current, err := enumerateProjectFiles(homepageLocalRoot(cfg, rev.ProjectDir), logger)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	present := make(map[string]fileEntry)
	for _, f := range current {
		present[f.Path] = f
	}
	files := []HomepageRevisionFile{}
	for name, f := range target {
		if path != "" && name != path {
			continue
		}
		if now, ok := present[name]; ok && now.Hash == f.Hash {
			continue
		}
		if int64(len(f.Data)) != f.Size || hashContent(f.Data) != f.Hash {
			return ErrorJSONf("revision %d has incomplete or corrupted content for %s; no files changed", revisionID, name)
		}
		files = append(files, HomepageRevisionFile{Path: name, ChangeType: "modified", ContentAfter: string(f.Data), ContentHashAfter: f.Hash, SizeAfter: f.Size, FileMode: int64(f.Mode)})
	}
	latest, err := GetLatestHomepageRevision(db, rev.ProjectDir)
	if err != nil {
		return ErrorJSON(err.Error())
	}
	known := make(map[string]fileEntry)
	if latest != nil {
		known, err = reconstructProjectState(db, rev.ProjectDir, latest.ID, logger)
		if err != nil {
			return ErrorJSON(err.Error())
		}
	}
	for name := range present {
		if path != "" && name != path {
			continue
		}
		if _, ok := target[name]; !ok {
			if _, tracked := known[name]; !tracked {
				continue
			} // Preserve unknown external files.
			files = append(files, HomepageRevisionFile{Path: name, ChangeType: "deleted"})
		}
	}
	if path != "" {
		if !filepath.IsLocal(filepath.FromSlash(path)) || shouldExcludePath(path) {
			return ErrorJSON("unsafe or sensitive restore path")
		}
		if _, ok := target[path]; !ok {
			if _, ok := known[path]; !ok {
				return ErrorJSON("path is not part of the revision history")
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	// Keep a complete preimage even for assets too large for database revisions.
	backup, err := os.MkdirTemp("", "aurago-homepage-restore-")
	if err != nil {
		return ErrorJSON(err.Error())
	}
	failed := false
	defer func() {
		if !failed {
			os.RemoveAll(backup)
		}
	}()
	root, err := os.OpenRoot(homepageLocalRoot(cfg, rev.ProjectDir))
	if err != nil {
		return ErrorJSON(err.Error())
	}
	defer root.Close()
	for _, f := range files {
		if _, ok := present[f.Path]; !ok {
			continue
		}
		src, err := root.Open(filepath.FromSlash(f.Path))
		if err != nil {
			return ErrorJSON(err.Error())
		}
		dest := filepath.Join(backup, filepath.FromSlash(f.Path))
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			src.Close()
			return ErrorJSON(err.Error())
		}
		dst, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			src.Close()
			return ErrorJSON(err.Error())
		}
		_, copyErr := io.Copy(dst, src)
		syncErr := dst.Sync()
		dst.Close()
		src.Close()
		if copyErr != nil {
			return ErrorJSON(copyErr.Error())
		}
		if syncErr != nil {
			return ErrorJSON(syncErr.Error())
		}
	}
	restored, warnings := RestoreRevisionFiles(homepageLocalRoot(cfg, rev.ProjectDir), files, logger)
	failed = len(warnings) > 0
	outcome := "ok"
	if failed {
		outcome = "partial_success"
	}
	restoreReason := fmt.Sprintf("Restore requested revision %d", revisionID)
	if op := homepageCurrentOperation(cfg); op.Reason != "" {
		restoreReason = op.Reason
	}
	saved := SaveHomepageRevisionAndState(cfg, db, rev.ProjectDir, fmt.Sprintf("Restore revision %d", revisionID), restoreReason, "homepage_restore", map[string]interface{}{"restored_revision_id": revisionID}, logger)
	warnings = append(warnings, saved.Warnings...)
	if len(warnings) > 0 {
		failed = true
		outcome = "partial_success"
	}
	raw, _ := json.Marshal(map[string]interface{}{"status": outcome, "revision_id": saved.RevisionID, "restored_revision_id": revisionID, "project_dir": rev.ProjectDir, "restored": restored, "restored_count": len(restored), "warnings": warnings, "retry_safe": !failed, "recovery_backup": func() string {
		if failed {
			return backup
		}
		return ""
	}()})
	return string(raw)
}

func HomepageRevisionStatus(cfg HomepageConfig, db *sql.DB, projectDir string, logger *slog.Logger) string {
	if cfg.WorkspacePath == "" {
		return `{"status":"error","message":"workspace_path not configured"}`
	}
	if db == nil {
		return `{"status":"error","message":"homepage registry DB not initialized"}`
	}
	if projectDir == "" {
		return `{"status":"error","message":"project_dir is required"}`
	}
	basePath := filepath.Join(cfg.WorkspacePath, projectDir)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Sprintf(`{"status":"error","message":"project directory '%s' does not exist"}`, projectDir)
	}
	status := &revisionStatus{ProjectDir: projectDir}
	latest, err := GetLatestHomepageRevision(db, projectDir)
	if err != nil || latest == nil {
		b, _ := json.Marshal(map[string]interface{}{
			"status":        "ok",
			"project_dir":   projectDir,
			"has_revisions": false,
			"message":       "no saved revisions",
		})
		return string(b)
	}
	status.HasRevisions = true
	status.LatestRevID = latest.ID
	status.LatestMessage = latest.Message
	delta, err := computeUnifiedDelta(db, projectDir, basePath, latest.ID, logger)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	status.UnifiedDelta = *delta
	b, _ := json.Marshal(map[string]interface{}{
		"status":         "ok",
		"project_dir":    projectDir,
		"has_revisions":  true,
		"latest_rev_id":  status.LatestRevID,
		"latest_message": status.LatestMessage,
		"added":          len(delta.Added),
		"modified":       len(delta.Modified),
		"deleted":        len(delta.Deleted),
	})
	return string(b)
}
