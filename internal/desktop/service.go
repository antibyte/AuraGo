package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 4

var desktopIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)
var stagedDeletePattern = regexp.MustCompile(`^.+\.delete-\d{14}\.\d{9}$`)

// desktopMutationMu serializes file mutations across all Service instances
// sharing the same workspace directory.  Kept at package level so that
// multiple Service objects (e.g. server singleton + tool singleton) cannot
// corrupt the workspace with concurrent writes.
// Read operations may use RLock for concurrent reads; write operations use Lock.
var desktopMutationMu sync.RWMutex

const (
	desktopCopyMaxDepth   = 64
	desktopCopyMaxEntries = 10000
	desktopCopyMaxBytes   = int64(512 * 1024 * 1024)
)

type desktopCopyStats struct {
	entries int
	bytes   int64
}

type mediaMount struct {
	Name      string
	Dir       string
	WebPrefix string
	Kind      string
}

// Service owns the virtual desktop workspace and registry database.
type Service struct {
	mu                  sync.Mutex
	cfg                 Config
	db                  *sql.DB
	mediaRegistryDB     *sql.DB
	imageGalleryDB      *sql.DB
	codeContainer       *CodeContainerService
	closed              bool
	bootstrapCache      BootstrapPayload
	bootstrapCacheMu    sync.RWMutex
	bootstrapCacheValid bool
}

// FileWriteState is the current target state observed while holding the shared desktop mutation lock.
type FileWriteState struct {
	Data   []byte
	Entry  FileEntry
	Exists bool
}

// FileWritePrecondition can reject a write after observing the current target state under the shared desktop mutation lock.
type FileWritePrecondition func(FileWriteState) error

// NewService creates a desktop service. Call Init before using it.
func NewService(cfg Config) (*Service, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{cfg: normalized, codeContainer: NewCodeContainerService(normalized, nil)}, nil
}

func normalizeConfig(cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		return cfg, fmt.Errorf("workspace directory is required")
	}
	if strings.TrimSpace(cfg.DBPath) == "" {
		return cfg, fmt.Errorf("desktop database path is required")
	}
	workspaceDir, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return cfg, fmt.Errorf("resolve workspace directory: %w", err)
	}
	dbPath, err := filepath.Abs(cfg.DBPath)
	if err != nil {
		return cfg, fmt.Errorf("resolve desktop database path: %w", err)
	}
	cfg.WorkspaceDir = filepath.Clean(workspaceDir)
	cfg.DBPath = filepath.Clean(dbPath)
	if strings.TrimSpace(cfg.DataDir) == "" {
		cfg.DataDir = filepath.Join(filepath.Dir(cfg.DBPath), "data")
	}
	dataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return cfg, fmt.Errorf("resolve data directory: %w", err)
	}
	cfg.DataDir = filepath.Clean(dataDir)
	if strings.TrimSpace(cfg.DocumentDir) == "" {
		cfg.DocumentDir = filepath.Join(cfg.DataDir, "documents")
	}
	documentDir, err := filepath.Abs(cfg.DocumentDir)
	if err != nil {
		return cfg, fmt.Errorf("resolve document directory: %w", err)
	}
	cfg.DocumentDir = filepath.Clean(documentDir)
	if strings.TrimSpace(cfg.MediaRegistryPath) == "" {
		cfg.MediaRegistryPath = filepath.Join(cfg.DataDir, "media_registry.db")
	}
	mediaRegistryPath, err := filepath.Abs(cfg.MediaRegistryPath)
	if err != nil {
		return cfg, fmt.Errorf("resolve media registry database path: %w", err)
	}
	cfg.MediaRegistryPath = filepath.Clean(mediaRegistryPath)
	if strings.TrimSpace(cfg.ImageGalleryPath) == "" {
		cfg.ImageGalleryPath = filepath.Join(cfg.DataDir, "image_gallery.db")
	}
	imageGalleryPath, err := filepath.Abs(cfg.ImageGalleryPath)
	if err != nil {
		return cfg, fmt.Errorf("resolve image gallery database path: %w", err)
	}
	cfg.ImageGalleryPath = filepath.Clean(imageGalleryPath)
	if cfg.MaxFileSizeMB <= 0 {
		cfg.MaxFileSizeMB = 50
	}
	if cfg.ControlLevel == "" {
		cfg.ControlLevel = ControlConfirmDestructive
	}
	if cfg.MaxWSClients <= 0 {
		cfg.MaxWSClients = 8
	}
	return cfg, nil
}

// Config returns a copy of the service configuration.
func (s *Service) Config() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Init creates workspace folders and opens the desktop registry database.
func (s *Service) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("desktop service is closed")
	}
	s.invalidateBootstrapCache()
	var createdDirs []string
	ensureDir := func(path string) error {
		_, err := os.Stat(path)
		exists := err == nil
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		_ = os.Chmod(path, 0o700)
		if !exists {
			createdDirs = append(createdDirs, path)
		}
		return nil
	}
	if err := ensureDir(s.cfg.WorkspaceDir); err != nil {
		return fmt.Errorf("create desktop workspace: %w", err)
	}
	for _, dir := range DefaultDirectories() {
		if isMediaMountName(dir) {
			continue
		}
		if err := ensureDir(filepath.Join(s.cfg.WorkspaceDir, dir)); err != nil {
			return fmt.Errorf("create desktop directory %s: %w", dir, err)
		}
	}
	for _, mount := range s.mediaMountsLocked() {
		if err := ensureDir(mount.Dir); err != nil {
			return fmt.Errorf("create desktop media mount %s: %w", mount.Name, err)
		}
	}
	if err := ensureDir(filepath.Dir(s.cfg.DBPath)); err != nil {
		return fmt.Errorf("create desktop database directory: %w", err)
	}
	cleanup := func() {
		for i := len(createdDirs) - 1; i >= 0; i-- {
			_ = os.RemoveAll(createdDirs[i])
		}
	}
	if s.db == nil {
		db, err := sql.Open("sqlite", s.cfg.DBPath)
		if err != nil {
			cleanup()
			return fmt.Errorf("open desktop database: %w", err)
		}
		s.db = db
	}
	if err := s.migrateLocked(ctx); err != nil {
		cleanup()
		return err
	}
	s.cleanupStaleDeletes()
	return nil
}

func (s *Service) cleanupStaleDeletes() {
	entries, err := os.ReadDir(s.cfg.WorkspaceDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if stagedDeletePattern.MatchString(e.Name()) {
			_ = os.RemoveAll(filepath.Join(s.cfg.WorkspaceDir, e.Name()))
		}
	}
}

// Close closes the desktop registry database and stops any managed Code Studio container.
// It always attempts to close all resources even if an earlier step fails.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.invalidateBootstrapCache()

	var closeErr error

	// Stop Code Studio container first (best effort, with timeout).
	if s.codeContainer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.codeContainer.Stop(ctx); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("code container stop: %w", err))
		}
		cancel()
	}

	// Close DBs in a consistent order. Always attempt every close.
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("desktop registry db: %w", err))
		}
		s.db = nil
	}
	if s.mediaRegistryDB != nil {
		if err := s.mediaRegistryDB.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("media registry db: %w", err))
		}
		s.mediaRegistryDB = nil
	}
	if s.imageGalleryDB != nil {
		if err := s.imageGalleryDB.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("image gallery db: %w", err))
		}
		s.imageGalleryDB = nil
	}

	return closeErr
}

// CodeContainer returns the lazy Code Studio container service.
func (s *Service) CodeContainer() *CodeContainerService {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.codeContainer
}

// DB returns the underlying SQLite database handle.
func (s *Service) DB() *sql.DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db
}

// getDB returns the SQLite database handle. Callers must ensure the service
// is initialized (via ensureReady) before calling this method. The DB handle
// is effectively immutable after Init(), so this does not acquire the mutex.
func (s *Service) getDB() *sql.DB {
	return s.db
}

func (s *Service) ensureReady(ctx context.Context) error {
	s.mu.Lock()
	ready := s.db != nil && !s.closed
	s.mu.Unlock()
	if ready {
		return nil
	}
	return s.Init(ctx)
}

func (s *Service) migrateLocked(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS desktop_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_apps (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			icon TEXT NOT NULL,
			entry TEXT NOT NULL,
			manifest_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_app_visibility (
			app_id TEXT PRIMARY KEY,
			dock_visible INTEGER NOT NULL DEFAULT 1,
			start_visible INTEGER NOT NULL DEFAULT 1,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_widgets (
			id TEXT PRIMARY KEY,
			app_id TEXT,
			title TEXT NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			w INTEGER NOT NULL,
			h INTEGER NOT NULL,
			config_json TEXT NOT NULL,
			widget_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_shortcuts (
			id TEXT PRIMARY KEY,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			icon TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			action TEXT NOT NULL,
			target TEXT,
			source TEXT,
			details_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`INSERT INTO desktop_meta(key, value) VALUES('schema_version', '4')
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate desktop database: %w", err)
		}
	}
	if err := s.ensureColumnLocked(ctx, "desktop_widgets", "widget_json", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := s.ensureColumnLocked(ctx, "desktop_widgets", "visible", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumnLocked(ctx, "desktop_widgets", "builtin", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.seedBuiltinWidgetsLocked(ctx); err != nil {
		return err
	}
	if err := s.seedDesktopShortcutsLocked(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) seedDesktopShortcutsLocked(ctx context.Context) error {
	var seeded string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM desktop_meta WHERE key = 'desktop_shortcuts_seeded'`).Scan(&seeded)
	if err == nil && seeded == "true" {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read desktop shortcuts seed state: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	defaults := []Shortcut{
		{ID: "app-files", TargetType: ShortcutTargetApp, TargetID: "files", Name: "Files", Icon: "folder"},
		{ID: "dir-Trash", TargetType: ShortcutTargetDirectory, Path: "Trash", Name: "Trash", Icon: "trash"},
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin desktop shortcuts seed: %w", err)
	}
	defer tx.Rollback()
	for _, shortcut := range defaults {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO desktop_shortcuts(id, target_type, target_id, path, name, icon, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			shortcut.ID, shortcut.TargetType, shortcut.TargetID, shortcut.Path, shortcut.Name, shortcut.Icon, now, now); err != nil {
			return fmt.Errorf("seed desktop shortcut %s: %w", shortcut.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO desktop_meta(key, value) VALUES('desktop_shortcuts_seeded', 'true')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		return fmt.Errorf("mark desktop shortcuts seeded: %w", err)
	}
	return tx.Commit()
}

func (s *Service) seedBuiltinWidgetsLocked(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	defaults := []Widget{
		{ID: "builtin-analog-clock", Title: "Analog Clock", Icon: "calculator", Type: "builtin", Runtime: BuiltinRuntime, X: 0, Y: 0, W: 2, H: 2, Visible: true, Builtin: true},
		{ID: "builtin-quickchat", Title: "Quick Chat", Icon: "chat", Type: "builtin", Runtime: BuiltinRuntime, X: 0, Y: 0, W: 2, H: 2, Visible: true, Builtin: true},
		{ID: "builtin-weather", Title: "Weather", Icon: "weather", Type: "builtin", Runtime: BuiltinRuntime, X: 0, Y: 0, W: 310, H: 220, Visible: true, Builtin: true},
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin desktop builtin widgets seed: %w", err)
	}
	defer tx.Rollback()
	for _, widget := range defaults {
		widgetJSON, _ := json.Marshal(widget)
		configJSON, _ := json.Marshal(widget.Config)
		if _, err := tx.ExecContext(ctx, `INSERT INTO desktop_widgets(id, app_id, title, x, y, w, h, config_json, widget_json, visible, builtin, created_at, updated_at)
			VALUES(?, '', ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			widget.ID, widget.Title, widget.X, widget.Y, widget.W, widget.H, string(configJSON), string(widgetJSON), now, now); err != nil {
			return fmt.Errorf("seed desktop builtin widget %s: %w", widget.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO desktop_meta(key, value) VALUES('desktop_builtin_widgets_seeded', 'true')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		return fmt.Errorf("mark desktop builtin widgets seeded: %w", err)
	}
	return tx.Commit()
}

func (s *Service) ensureColumnLocked(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect desktop table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan desktop table %s: %w", table, err)
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect desktop table %s: %w", table, err)
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add desktop column %s.%s: %w", table, column, err)
	}
	return nil
}

func isMediaMountName(name string) bool {
	for _, mountName := range []string{"Music", "Photos", "Videos", "AuraGo Documents"} {
		if strings.EqualFold(name, mountName) {
			return true
		}
	}
	return false
}

func (s *Service) mediaMountsLocked() []mediaMount {
	cfg := s.cfg
	return []mediaMount{
		{Name: "Music", Dir: filepath.Join(cfg.DataDir, "audio"), WebPrefix: "/files/audio/", Kind: "audio"},
		{Name: "Photos", Dir: filepath.Join(cfg.DataDir, "generated_images"), WebPrefix: "/files/generated_images/", Kind: "image"},
		{Name: "Videos", Dir: filepath.Join(cfg.DataDir, "generated_videos"), WebPrefix: "/files/generated_videos/", Kind: "video"},
		{Name: "AuraGo Documents", Dir: cfg.DocumentDir, WebPrefix: "/files/documents/", Kind: "document"},
	}
}

func (s *Service) mediaMounts() []mediaMount {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mediaMount(nil), s.mediaMountsLocked()...)
}

// Bootstrap returns all state needed to render the virtual desktop shell.
func (s *Service) Bootstrap(ctx context.Context) (BootstrapPayload, error) {
	s.bootstrapCacheMu.RLock()
	if s.bootstrapCacheValid {
		payload := s.bootstrapCache
		s.bootstrapCacheMu.RUnlock()
		return payload, nil
	}
	s.bootstrapCacheMu.RUnlock()

	if err := s.ensureReady(ctx); err != nil {
		return BootstrapPayload{}, err
	}
	apps, err := s.listApps(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	for i := range apps {
		apps[i] = s.validateGeneratedAppEntry(ctx, apps[i])
	}
	appVisibility, err := s.listAppVisibility(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	widgets, err := s.listWidgets(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	shortcuts, err := s.listShortcuts(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	cfg := s.Config()
	var visibleWidgets []Widget
	for _, w := range widgets {
		if w.Visible {
			visibleWidgets = append(visibleWidgets, w)
		}
	}
	payload := BootstrapPayload{
		Enabled:            cfg.Enabled,
		ReadOnly:           cfg.ReadOnly,
		AllowAgentControl:  cfg.AllowAgentControl,
		AllowGeneratedApps: cfg.AllowGeneratedApps,
		AllowPythonJobs:    cfg.AllowPythonJobs,
		ControlLevel:       cfg.ControlLevel,
		Workspace: WorkspaceInfo{
			Root:        "/",
			Directories: DefaultDirectories(),
			MaxFileSize: int64(cfg.MaxFileSizeMB) * 1024 * 1024,
		},
		BuiltinApps:   applyAppVisibility(BuiltinApps(), true, appVisibility),
		InstalledApps: applyAppVisibility(apps, false, appVisibility),
		Shortcuts:     shortcuts,
		Widgets:       visibleWidgets,
		AllWidgets:    widgets,
		Settings:      settings,
		IconCatalog:   DesktopIconCatalog(settings),
	}

	s.bootstrapCacheMu.Lock()
	s.bootstrapCache = payload
	s.bootstrapCacheValid = true
	s.bootstrapCacheMu.Unlock()
	return payload, nil
}

func (s *Service) invalidateBootstrapCache() {
	s.bootstrapCacheMu.Lock()
	s.bootstrapCacheValid = false
	s.bootstrapCacheMu.Unlock()
}

// ResolvePath returns an absolute path that is guaranteed to remain inside the workspace.
func (s *Service) ResolvePath(rawPath string) (string, error) {
	cfg := s.Config()
	cleaned := cleanDesktopPath(rawPath)
	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Join(cfg.WorkspaceDir, cleaned)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve desktop path: %w", err)
	}
	rootAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve desktop root: %w", err)
	}
	if !isWithinPath(rootAbs, candidateAbs) {
		return "", fmt.Errorf("desktop path escapes workspace")
	}
	if err := validatePathComponentsWithinRoot(rootAbs, candidateAbs, "workspace"); err != nil {
		return "", err
	}
	return candidateAbs, nil
}

func (s *Service) resolveRenamePath(rawPath string) (string, error) {
	cfg := s.Config()
	cleaned := cleanDesktopPath(rawPath)
	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Join(cfg.WorkspaceDir, cleaned)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve desktop path: %w", err)
	}
	rootAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve desktop root: %w", err)
	}
	if !isWithinPath(rootAbs, candidateAbs) {
		return "", fmt.Errorf("desktop path escapes workspace")
	}
	parentAbs := filepath.Dir(candidateAbs)
	if candidateAbs == rootAbs {
		parentAbs = rootAbs
	}
	if err := validatePathComponentsWithinRoot(rootAbs, parentAbs, "workspace"); err != nil {
		return "", err
	}
	return candidateAbs, nil
}

func cleanDesktopPath(rawPath string) string {
	p := strings.TrimSpace(rawPath)
	if p == "" || p == "/" || p == `\` {
		return "."
	}
	p = strings.ReplaceAll(p, "\\", "/")
	return filepath.Clean(filepath.FromSlash(p))
}

func cleanDesktopPathSlash(rawPath string) string {
	p := strings.TrimSpace(rawPath)
	if p == "" || p == "/" || p == `\` {
		return "."
	}
	p = strings.ReplaceAll(p, "\\", "/")
	return slashpath.Clean(p)
}

func desktopPathHasParentSegment(rawPath string) bool {
	p := strings.ReplaceAll(strings.TrimSpace(rawPath), "\\", "/")
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func isStandaloneWidgetHTMLPath(rawPath string) bool {
	p := filepath.ToSlash(cleanDesktopPath(rawPath))
	return strings.EqualFold(slashpath.Dir(p), "Widgets") && strings.EqualFold(slashpath.Ext(p), ".html")
}

func isWithinPath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func validatePathComponentsWithinRoot(rootAbs, candidateAbs, label string) error {
	rootAbs, err := filepath.Abs(rootAbs)
	if err != nil {
		return fmt.Errorf("resolve desktop %s root: %w", label, err)
	}
	candidateAbs, err = filepath.Abs(candidateAbs)
	if err != nil {
		return fmt.Errorf("resolve desktop %s path: %w", label, err)
	}
	if !isWithinPath(rootAbs, candidateAbs) {
		return fmt.Errorf("desktop path escapes %s", label)
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return fmt.Errorf("resolve desktop %s relative path: %w", label, err)
	}
	if rel == "." {
		return nil
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("inspect desktop %s path: %w", label, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(current)
		if err != nil {
			return fmt.Errorf("read desktop %s symlink: %w", label, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolve desktop %s symlink: %w", label, err)
		}
		if !isWithinPath(rootAbs, targetAbs) {
			return fmt.Errorf("desktop path follows symlink outside %s", label)
		}
		evaluated, err := filepath.EvalSymlinks(targetAbs)
		if err != nil {
			return fmt.Errorf("desktop path follows invalid symlink in %s: %w", label, err)
		}
		if !isWithinPath(rootAbs, evaluated) {
			return fmt.Errorf("desktop path follows symlink outside %s", label)
		}
		current = evaluated
	}
	return nil
}

func (s *Service) relativePath(absPath string) string {
	cfg := s.Config()
	rel, err := filepath.Rel(cfg.WorkspaceDir, absPath)
	if err != nil {
		return filepath.ToSlash(filepath.Base(absPath))
	}
	return filepath.ToSlash(rel)
}

func (s *Service) workspaceFileEntry(path string, info os.FileInfo) FileEntry {
	return FileEntry{
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
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func normalizeDesktopRuntime(runtime string) string {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return AuraDesktopRuntime
	}
	return runtime
}

func normalizeDesktopPermissions(permissions []string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		permission = strings.ToLower(strings.TrimSpace(permission))
		if permission == "" {
			continue
		}
		if !allowedDesktopPermission(permission) {
			return nil, fmt.Errorf("unsupported desktop permission %q", permission)
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		result = append(result, permission)
	}
	return result, nil
}

func allowedDesktopPermission(permission string) bool {
	switch permission {
	case "apps:open",
		"files:read",
		"files:write",
		"filesystem:read",
		"filesystem:write",
		"notifications",
		"widgets:write":
		return true
	default:
		return false
	}
}
