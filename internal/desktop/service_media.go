package desktop

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurago/internal/dbutil"
	"aurago/internal/fileutil"
)

const (
	storymakerVideoFolder = "Storymaker"
	// storymakerVideoMargin is the free space a library copy must leave.
	storymakerVideoMargin = int64(64) << 20
)

var storymakerVideoSlug = regexp.MustCompile(`^[A-Za-z0-9-]{1,80}$`)

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

// WriteStorymakerVideo copies r into Videos/Storymaker/<slug>.mp4, or <slug>-2.mp4, <slug>-3.mp4 and so on.
// slug must match ^[A-Za-z0-9-]{1,80}$; the path is built here. It returns the desktop path. Slow source I/O
// never holds the shared mutation lock. RegisterMedia is skipped on purpose.
func (s *Service) WriteStorymakerVideo(ctx context.Context, slug string, r io.Reader, size int64, source string) (string, error) {
	if !storymakerVideoSlug.MatchString(slug) {
		return "", fmt.Errorf("invalid storymaker video name")
	}
	if r == nil || size <= 0 {
		return "", fmt.Errorf("invalid storymaker video size")
	}
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	if s.Config().ReadOnly {
		return "", fmt.Errorf("virtual desktop is read-only")
	}
	mount, _, _, mounted, err := s.resolveMediaMount("Videos/" + storymakerVideoFolder)
	if err != nil {
		return "", err
	}
	if !mounted {
		return "", fmt.Errorf("the Videos media mount is not available")
	}
	free, err := fileutil.FreeDiskBytes(mount.Dir)
	if err != nil {
		return "", fmt.Errorf("check free space: %w", err)
	}
	if free <= size+storymakerVideoMargin {
		return "", fmt.Errorf("not enough free space for the library copy (%d MB free)", free>>20)
	}
	root, err := os.OpenRoot(mount.Dir)
	if err != nil {
		return "", fmt.Errorf("open videos root: %w", err)
	}
	defer root.Close()
	// Only the mount root exists at start, so the first copy creates the folder.
	if err := root.MkdirAll(storymakerVideoFolder, 0o700); err != nil {
		return "", fmt.Errorf("create the storymaker video folder: %w", err)
	}
	temp := filepath.Join(storymakerVideoFolder, ".aurago-write-"+rand.Text())
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("stage the library copy: %w", err)
	}
	defer root.Remove(temp)
	n, writeErr := io.Copy(out, io.LimitReader(desktopContextReader{ctx, r}, size+1))
	if writeErr == nil && n != size {
		writeErr = fmt.Errorf("the source has %d bytes, expected %d", n, size)
	}
	if writeErr == nil {
		writeErr = out.Sync()
	}
	closeErr := out.Close()
	if writeErr != nil {
		return "", fmt.Errorf("write the library copy: %w", writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close the library copy: %w", closeErr)
	}

	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.Config().ReadOnly {
		return "", fmt.Errorf("virtual desktop is read-only")
	}
	name, err := freeStorymakerVideoName(root, slug)
	if err != nil {
		return "", err
	}
	if err := fileutil.RenameRootContext(ctx, root, temp, filepath.Join(storymakerVideoFolder, name)); err != nil {
		return "", fmt.Errorf("publish the library copy: %w", err)
	}
	desktopPath := "Videos/" + storymakerVideoFolder + "/" + name
	s.invalidateListCache()
	_ = s.Audit(ctx, "write_file", desktopPath, map[string]interface{}{"bytes": n}, source)
	return desktopPath, nil
}

// freeStorymakerVideoName finds the first name that is not taken. The caller holds desktopMutationMu.
func freeStorymakerVideoName(root *os.Root, slug string) (string, error) {
	for i := 1; i <= 100000; i++ {
		name := slug + ".mp4"
		if i > 1 {
			name = fmt.Sprintf("%s-%d.mp4", slug, i)
		}
		_, err := root.Lstat(filepath.Join(storymakerVideoFolder, name))
		if os.IsNotExist(err) {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("stat the library copy: %w", err)
		}
	}
	return "", fmt.Errorf("no free library name for %s", slug)
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
	case ".md", ".log", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".csv", ".html", ".htm", ".css", ".js", ".mjs", ".ts", ".tsx", ".jsx", ".go", ".py", ".sh", ".ps1", ".sql", ".svg", ".aurasynth":
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
	db, err := dbutil.Open(dbPath)
	if err != nil {
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
