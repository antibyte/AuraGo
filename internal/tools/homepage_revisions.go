package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
}

type revisionDelta struct {
	Added    []fileEntry
	Modified []fileEntry
	Deleted  []fileEntry
}

func enumerateProjectFiles(basePath string, logger *slog.Logger) ([]fileEntry, error) {
	var entries []fileEntry
	err := filepath.Walk(basePath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(basePath, p)
		if err != nil {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		if shouldExcludePath(slashRel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		entries = append(entries, fileEntry{
			Path:     slashRel,
			Data:     data,
			Size:     info.Size(),
			Hash:     hashContent(data),
			IsBinary: isBinaryContent(data),
		})
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

func computeUnifiedDelta(db *sql.DB, projectDir string, baseRevID int64, logger *slog.Logger) (*revisionDelta, error) {
	baseline, err := reconstructProjectState(db, projectDir, baseRevID, logger)
	if err != nil {
		return nil, err
	}
	currentFiles, err := enumerateProjectFiles(projectDir, logger)
	if err != nil {
		return nil, err
	}
	delta := computeDelta(baseline, currentFiles)
	return &delta, nil
}

func reconstructProjectState(db *sql.DB, projectDir string, baseRevID int64, logger *slog.Logger) (map[string]fileEntry, error) {
	revisions, _, err := ListHomepageRevisions(db, projectDir, 1000, 0)
	if err != nil {
		return nil, err
	}
	revFiles := make(map[int64][]HomepageRevisionFile)
	for _, rev := range revisions {
		files, err := GetHomepageRevisionFiles(db, rev.ID)
		if err != nil {
			continue
		}
		revFiles[rev.ID] = files
	}
	state := make(map[string]fileEntry)
	for _, rev := range revisions {
		for _, f := range revFiles[rev.ID] {
			switch f.ChangeType {
			case "added", "modified":
				if f.ContentAfter != "" {
					state[f.Path] = fileEntry{
						Path: f.Path, Data: []byte(f.ContentAfter),
						Size: int64(len(f.ContentAfter)), Hash: f.ContentHashAfter,
						IsBinary: isBinaryContent([]byte(f.ContentAfter)),
					}
				}
			case "deleted":
				delete(state, f.Path)
			}
		}
		if rev.ID == baseRevID {
			break
		}
	}
	return state, nil
}

func RestoreRevisionFiles(projectDir string, files []HomepageRevisionFile, logger *slog.Logger) (restored []string, warnings []string) {
	projectDir, _ = filepath.Abs(projectDir)
	for _, f := range files {
		relPath := filepath.FromSlash(f.Path)
		fullPath := filepath.Join(projectDir, relPath)
		cleanFull := filepath.Clean(fullPath)
		if !strings.HasPrefix(cleanFull, projectDir+string(os.PathSeparator)) {
			warnings = append(warnings, fmt.Sprintf("path %q escapes project directory, skipping", f.Path))
			continue
		}
		switch f.ChangeType {
		case "added", "modified":
			if f.ContentAfter == "" {
				warnings = append(warnings, fmt.Sprintf("no content for %s", f.Path))
				continue
			}
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to create dir for %s: %v", f.Path, err))
				continue
			}
			if err := os.WriteFile(fullPath, []byte(f.ContentAfter), 0644); err != nil {
				warnings = append(warnings, fmt.Sprintf("failed to write %s: %v", f.Path, err))
				continue
			}
			restored = append(restored, f.Path)
		case "deleted":
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("failed to delete %s: %v", f.Path, err))
			} else {
				restored = append(restored, f.Path)
			}
		}
	}
	return restored, warnings
}

func HomepageSaveRevision(cfg HomepageConfig, db *sql.DB, projectDir, message, reason string, logger *slog.Logger) string {
	if projectDir == "" {
		return `{"status":"error","message":"project_dir is required"}`
	}
	if err := sanitizeProjectDir(projectDir); err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%v"}`, err)
	}
	if cfg.WorkspacePath == "" {
		return `{"status":"error","message":"workspace_path not configured"}`
	}
	basePath := filepath.Join(cfg.WorkspacePath, projectDir)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Sprintf(`{"status":"error","message":"project directory '%s' does not exist"}`, projectDir)
	}
	projectID := int64(0)
	if db != nil {
		if proj, err := GetProjectByDir(db, projectDir); err == nil {
			projectID = proj.ID
		}
	}
	currentFiles, err := enumerateProjectFiles(basePath, logger)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"failed to enumerate files: %v"}`, err)
	}
	var baseline map[string]fileEntry
	var delta revisionDelta
	if db != nil {
		if latest, err := GetLatestHomepageRevision(db, projectDir); err == nil && latest != nil {
			baseline, _ = reconstructProjectState(db, basePath, latest.ID, logger)
		}
	}
	if baseline == nil {
		baseline = make(map[string]fileEntry)
	}
	delta = computeDelta(baseline, currentFiles)
	totalChanged := len(delta.Added) + len(delta.Modified) + len(delta.Deleted)
	if totalChanged == 0 {
		return fmt.Sprintf(`{"status":"ok","message":"no changes since last revision","revision_id":null}`)
	}
	author := "agent"
	if message == "" {
		message = fmt.Sprintf("Saved %d changed file(s)", totalChanged)
	}
	restorable := true
	metadataJSON := fmt.Sprintf(`{"added":%d,"modified":%d,"deleted":%d}`, len(delta.Added), len(delta.Modified), len(delta.Deleted))
	revID, err := CreateHomepageRevision(db, projectID, projectDir, message, reason, author, totalChanged, restorable, metadataJSON)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"failed to save revision: %v"}`, err)
	}
	for _, f := range delta.Added {
		content := string(f.Data)
		if len(content) > maxRevisionFileSize {
			content = content[:maxRevisionFileSize]
		}
		CreateHomepageRevisionFile(db, revID, f.Path, "added", "", content, "", f.Hash, 0, f.Size)
	}
	for _, f := range delta.Modified {
		contentBefore := ""
		if base, exists := baseline[f.Path]; exists {
			contentBefore = string(base.Data)
			if len(contentBefore) > maxRevisionFileSize {
				contentBefore = contentBefore[:maxRevisionFileSize]
			}
		}
		contentAfter := string(f.Data)
		if len(contentAfter) > maxRevisionFileSize {
			contentAfter = contentAfter[:maxRevisionFileSize]
		}
		CreateHomepageRevisionFile(db, revID, f.Path, "modified", contentBefore, contentAfter, baseline[f.Path].Hash, f.Hash, baseline[f.Path].Size, f.Size)
	}
	for _, f := range delta.Deleted {
		CreateHomepageRevisionFile(db, revID, f.Path, "deleted", string(f.Data), "", f.Hash, "", f.Size, 0)
	}
	if db != nil && projectID > 0 {
		_ = LogEdit(db, projectID, reason)
	}
	b, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"revision_id": revID,
		"message":     message,
		"file_count":  totalChanged,
		"added":       len(delta.Added),
		"modified":    len(delta.Modified),
		"deleted":     len(delta.Deleted),
		"restorable":  restorable,
	})
	return string(b)
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
	diffs := make([]map[string]interface{}, 0, len(files))
	for _, f := range files {
		isBinary := isBinaryContent([]byte(f.ContentBefore)) || isBinaryContent([]byte(f.ContentAfter))
		beforeText, afterText := computeDiffText(f.ContentBefore, f.ContentAfter, isBinary)
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
		return `{"status":"error","message":"homepage registry DB not initialized"}`
	}
	if revisionID <= 0 {
		return `{"status":"error","message":"revision_id is required"}`
	}
	rev, err := GetHomepageRevision(db, revisionID)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
	}
	if !rev.Restorable {
		return `{"status":"error","message":"revision is marked as not restorable"}`
	}
	if cfg.WorkspacePath == "" {
		return `{"status":"error","message":"workspace_path not configured"}`
	}
	projectDir := rev.ProjectDir
	basePath := filepath.Join(cfg.WorkspacePath, projectDir)
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
	restored, warnings := RestoreRevisionFiles(basePath, files, logger)
	b, _ := json.Marshal(map[string]interface{}{
		"status":         "ok",
		"revision_id":    rev.ID,
		"project_dir":    projectDir,
		"restored":       restored,
		"restored_count": len(restored),
		"warnings":       warnings,
	})
	return string(b)
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
	delta, err := computeUnifiedDelta(db, basePath, latest.ID, logger)
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
