package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) resolveMediaMount(rawPath string) (mediaMount, string, string, bool, error) {
	cleanSlash := cleanDesktopPathSlash(rawPath)
	if cleanSlash == "." {
		return mediaMount{}, "", "", false, nil
	}
	parts := strings.Split(cleanSlash, "/")
	mountName := parts[0]
	for _, mount := range s.mediaMounts() {
		if !strings.EqualFold(mount.Name, mountName) {
			continue
		}
		if desktopPathHasParentSegment(rawPath) {
			return mount, "", "", true, fmt.Errorf("desktop media path escapes mount")
		}
		rel := ""
		if len(parts) > 1 {
			rel = strings.Join(parts[1:], "/")
		}
		candidate := mount.Dir
		if rel != "" {
			candidate = filepath.Join(mount.Dir, filepath.FromSlash(rel))
		}
		candidateAbs, err := filepath.Abs(candidate)
		if err != nil {
			return mount, "", "", true, fmt.Errorf("resolve desktop media path: %w", err)
		}
		mountAbs, err := filepath.Abs(mount.Dir)
		if err != nil {
			return mount, "", "", true, fmt.Errorf("resolve desktop media root: %w", err)
		}
		if !isWithinPath(mountAbs, candidateAbs) {
			return mount, "", "", true, fmt.Errorf("desktop media path escapes mount")
		}
		if err := validatePathComponentsWithinRoot(mountAbs, candidateAbs, "media mount"); err != nil {
			return mount, "", "", true, err
		}
		return mount, filepath.Clean(candidateAbs), filepath.ToSlash(rel), true, nil
	}
	return mediaMount{}, "", "", false, nil
}

func mediaRelativePath(mount mediaMount, absPath string) string {
	rel, err := filepath.Rel(mount.Dir, absPath)
	if err != nil {
		return filepath.Base(absPath)
	}
	return filepath.ToSlash(rel)
}

func mediaDesktopPath(mount mediaMount, rel string) string {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return mount.Name
	}
	return mount.Name + "/" + rel
}

func mediaWebPath(mount mediaMount, rel string) string {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return ""
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return mount.WebPrefix + strings.Join(parts, "/")
}

func mediaMIMEType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xlsm":
		return "application/vnd.ms-excel.sheet.macroEnabled.12"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".pdf":
		return "application/pdf"
	}
	if ext == ".mp3" {
		return "audio/mpeg"
	}
	if ext == ".m4a" {
		return "audio/mp4"
	}
	if ext == ".flac" {
		return "audio/flac"
	}
	if ext == ".opus" {
		return "audio/ogg"
	}
	if ext == ".mp4" {
		return "video/mp4"
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		if idx := strings.Index(mt, ";"); idx >= 0 {
			return mt[:idx]
		}
		return mt
	}
	return "application/octet-stream"
}

func desktopMediaKindForName(name string) string {
	mimeType := MIMETypeForName(name)
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "text/"), strings.Contains(mimeType, "json"), strings.Contains(mimeType, "xml"):
		return ""
	case strings.HasPrefix(mimeType, "application/pdf"),
		strings.Contains(mimeType, "wordprocessingml"),
		strings.Contains(mimeType, "spreadsheetml"),
		strings.Contains(mimeType, "presentationml"):
		return "document"
	default:
		return ""
	}
}

func isDesktopTextReadable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".docx", ".xlsx", ".xlsm", ".pptx", ".pdf":
		return false
	}
	mimeType := MIMETypeForName(name)
	if strings.HasPrefix(mimeType, "text/") || strings.Contains(mimeType, "json") || strings.Contains(mimeType, "xml") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".log", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".csv", ".html", ".htm", ".css", ".js", ".mjs", ".ts", ".tsx", ".jsx", ".go", ".py", ".sh", ".ps1", ".sql", ".svg":
		return true
	default:
		return false
	}
}

// MIMETypeForName returns the best-effort MIME type used by desktop file previews.
func MIMETypeForName(name string) string {
	return mediaMIMEType(name)
}

func mediaFileEntry(mount mediaMount, absPath string, info os.FileInfo) FileEntry {
	itemType := "file"
	if info.IsDir() {
		itemType = "directory"
	}
	rel := mediaRelativePath(mount, absPath)
	mediaKind := mount.Kind
	if itemType == "file" {
		if kind := desktopMediaKindForName(info.Name()); kind != "" {
			mediaKind = kind
		}
	}
	entry := FileEntry{
		Name:      info.Name(),
		Path:      mediaDesktopPath(mount, rel),
		Type:      itemType,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Modified:  info.ModTime(),
		MediaKind: mediaKind,
		Mount:     mount.Name,
		Mode:      info.Mode().String(),
		Created:   getCreationTime(info),
	}
	if itemType == "file" {
		entry.WebPath = mediaWebPath(mount, rel)
		entry.MIMEType = mediaMIMEType(info.Name())
	}
	return entry
}

func (s *Service) updateMediaRegistriesAfterMove(ctx context.Context, mount mediaMount, oldAbs, newAbs string) {
	cfg := s.Config()
	oldBase := filepath.Base(oldAbs)
	newBase := filepath.Base(newAbs)
	newWebPath := mediaWebPath(mount, mediaRelativePath(mount, newAbs))
	s.withMediaRegistryDB(ctx, cfg.MediaRegistryPath, func(db *sql.DB) {
		mediaPredicate, mediaArgs := mediaTypeSQLPredicate(mount.Kind)
		args := []interface{}{newBase, newAbs, newWebPath}
		args = append(args, mediaArgs...)
		args = append(args, oldAbs, oldBase)
		_, _ = db.ExecContext(ctx, `UPDATE media_items
			SET filename = ?, file_path = ?, web_path = ?, updated_at = CURRENT_TIMESTAMP
			WHERE deleted = 0 AND (`+mediaPredicate+`) AND (file_path = ? OR filename = ?)`,
			args...)
	})
	if mount.Kind == "image" {
		s.withMediaRegistryDB(ctx, cfg.ImageGalleryPath, func(db *sql.DB) {
			_, _ = db.ExecContext(ctx, "UPDATE generated_images SET filename = ? WHERE filename = ?", newBase, oldBase)
		})
	}
}

func (s *Service) softDeleteMediaRegistries(ctx context.Context, mount mediaMount, absPath string) {
	cfg := s.Config()
	base := filepath.Base(absPath)
	prefix := filepath.Clean(absPath) + string(os.PathSeparator) + "%"
	s.withMediaRegistryDB(ctx, cfg.MediaRegistryPath, func(db *sql.DB) {
		mediaPredicate, mediaArgs := mediaTypeSQLPredicate(mount.Kind)
		args := append([]interface{}{}, mediaArgs...)
		args = append(args, absPath, prefix, base)
		_, _ = db.ExecContext(ctx, `UPDATE media_items
			SET deleted = 1, updated_at = CURRENT_TIMESTAMP
			WHERE deleted = 0 AND (`+mediaPredicate+`) AND (file_path = ? OR file_path LIKE ? OR filename = ?)`,
			args...)
	})
	if mount.Kind == "image" {
		s.withMediaRegistryDB(ctx, cfg.ImageGalleryPath, func(db *sql.DB) {
			_, _ = db.ExecContext(ctx, "DELETE FROM generated_images WHERE filename = ?", base)
		})
	}
}

func (s *Service) withMediaRegistryDB(ctx context.Context, dbPath string, fn func(*sql.DB)) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return
	}
	db, err := s.mediaDB(ctx, dbPath)
	if err != nil {
		slog.Warn("failed to open desktop media registry database", "path", dbPath, "error", err)
		return
	}
	fn(db)
}

func (s *Service) mediaDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	dbPath = filepath.Clean(dbPath)
	s.mu.Lock()
	defer s.mu.Unlock()
	var slot **sql.DB
	switch dbPath {
	case filepath.Clean(s.cfg.MediaRegistryPath):
		slot = &s.mediaRegistryDB
	case filepath.Clean(s.cfg.ImageGalleryPath):
		slot = &s.imageGalleryDB
	default:
		return nil, fmt.Errorf("unknown desktop media registry database")
	}
	if *slot != nil {
		return *slot, nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	*slot = db
	return db, nil
}

func mediaTypeSQLPredicate(kind string) (string, []interface{}) {
	switch kind {
	case "audio":
		return "media_type IN (?, ?)", []interface{}{"audio", "music"}
	case "document":
		return "media_type = ?", []interface{}{"document"}
	case "video":
		return "media_type = ?", []interface{}{"video"}
	default:
		return "media_type = ?", []interface{}{"image"}
	}
}
