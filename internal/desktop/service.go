package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 2

var desktopIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// Service owns the virtual desktop workspace and registry database.
type Service struct {
	mu     sync.Mutex
	cfg    Config
	db     *sql.DB
	closed bool
}

// NewService creates a desktop service. Call Init before using it.
func NewService(cfg Config) (*Service, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{cfg: normalized}, nil
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
	if err := os.MkdirAll(s.cfg.WorkspaceDir, 0o755); err != nil {
		return fmt.Errorf("create desktop workspace: %w", err)
	}
	for _, dir := range DefaultDirectories() {
		if err := os.MkdirAll(filepath.Join(s.cfg.WorkspaceDir, dir), 0o755); err != nil {
			return fmt.Errorf("create desktop directory %s: %w", dir, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.DBPath), 0o755); err != nil {
		return fmt.Errorf("create desktop database directory: %w", err)
	}
	if s.db == nil {
		db, err := sql.Open("sqlite", s.cfg.DBPath)
		if err != nil {
			return fmt.Errorf("open desktop database: %w", err)
		}
		s.db = db
	}
	if err := s.migrateLocked(ctx); err != nil {
		return err
	}
	return nil
}

// Close closes the desktop registry database.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
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
		`CREATE TABLE IF NOT EXISTS desktop_audit (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			action TEXT NOT NULL,
			target TEXT,
			source TEXT,
			details_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`INSERT INTO desktop_meta(key, value) VALUES('schema_version', '2')
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
	return nil
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

// Bootstrap returns all state needed to render the virtual desktop shell.
func (s *Service) Bootstrap(ctx context.Context) (BootstrapPayload, error) {
	if err := s.ensureReady(ctx); err != nil {
		return BootstrapPayload{}, err
	}
	apps, err := s.listApps(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	widgets, err := s.listWidgets(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return BootstrapPayload{}, err
	}
	cfg := s.Config()
	return BootstrapPayload{
		Enabled:            cfg.Enabled,
		ReadOnly:           cfg.ReadOnly,
		AllowAgentControl:  cfg.AllowAgentControl,
		AllowGeneratedApps: cfg.AllowGeneratedApps,
		AllowPythonJobs:    cfg.AllowPythonJobs,
		ControlLevel:       cfg.ControlLevel,
		Workspace: WorkspaceInfo{
			Root:        cfg.WorkspaceDir,
			Directories: DefaultDirectories(),
			MaxFileSize: int64(cfg.MaxFileSizeMB) * 1024 * 1024,
		},
		BuiltinApps:   BuiltinApps(),
		InstalledApps: apps,
		Widgets:       widgets,
		Settings:      settings,
		IconCatalog:   DesktopIconCatalog(settings),
	}, nil
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
	if evaluated, err := filepath.EvalSymlinks(candidateAbs); err == nil && !isWithinPath(rootAbs, evaluated) {
		return "", fmt.Errorf("desktop path follows symlink outside workspace")
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

func (s *Service) relativePath(absPath string) string {
	cfg := s.Config()
	rel, err := filepath.Rel(cfg.WorkspaceDir, absPath)
	if err != nil {
		return filepath.ToSlash(filepath.Base(absPath))
	}
	return filepath.ToSlash(rel)
}

// ListFiles lists one workspace directory.
func (s *Service) ListFiles(ctx context.Context, rawPath string) ([]FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	dirPath, err := s.ResolvePath(rawPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("list desktop files: %w", err)
	}
	result := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil {
			return nil, fmt.Errorf("stat desktop file %s: %w", entry.Name(), statErr)
		}
		itemType := "file"
		if info.IsDir() {
			itemType = "directory"
		}
		result = append(result, FileEntry{
			Name:    entry.Name(),
			Path:    s.relativePath(filepath.Join(dirPath, entry.Name())),
			Type:    itemType,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type == "directory"
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

// ReadFile reads a UTF-8 text file from the workspace.
func (s *Service) ReadFile(ctx context.Context, rawPath string) (string, FileEntry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", FileEntry{}, err
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return "", FileEntry{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", FileEntry{}, fmt.Errorf("stat desktop file: %w", err)
	}
	if info.IsDir() {
		return "", FileEntry{}, fmt.Errorf("desktop path is a directory")
	}
	maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
	if info.Size() > maxBytes {
		return "", FileEntry{}, fmt.Errorf("desktop file exceeds max size")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", FileEntry{}, fmt.Errorf("read desktop file: %w", err)
	}
	return string(data), FileEntry{
		Name:    filepath.Base(path),
		Path:    s.relativePath(path),
		Type:    "file",
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// WriteFile writes a UTF-8 text file into the workspace.
func (s *Service) WriteFile(ctx context.Context, rawPath, content, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	maxBytes := int64(s.Config().MaxFileSizeMB) * 1024 * 1024
	if int64(len([]byte(content))) > maxBytes {
		return fmt.Errorf("desktop file exceeds max size")
	}
	if isStandaloneWidgetHTMLPath(rawPath) && strings.TrimSpace(content) == "" {
		return fmt.Errorf("desktop widget HTML file must not be empty")
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create desktop file directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write desktop file: %w", err)
	}
	_ = s.Audit(ctx, "write_file", s.relativePath(path), map[string]interface{}{"bytes": len([]byte(content))}, source)
	return nil
}

// CreateDirectory creates a workspace directory and any missing parents.
func (s *Service) CreateDirectory(ctx context.Context, rawPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create desktop directory: %w", err)
	}
	_ = s.Audit(ctx, "create_directory", s.relativePath(path), map[string]interface{}{}, source)
	return nil
}

// MovePath renames or moves a workspace file or directory.
func (s *Service) MovePath(ctx context.Context, oldPath, newPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	from, err := s.ResolvePath(oldPath)
	if err != nil {
		return err
	}
	to, err := s.ResolvePath(newPath)
	if err != nil {
		return err
	}
	if strings.EqualFold(from, to) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return fmt.Errorf("create desktop destination directory: %w", err)
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("move desktop path: %w", err)
	}
	_ = s.Audit(ctx, "move_path", s.relativePath(from), map[string]interface{}{"new_path": s.relativePath(to)}, source)
	return nil
}

// DeletePath removes a workspace file or directory tree.
func (s *Service) DeletePath(ctx context.Context, rawPath, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	path, err := s.ResolvePath(rawPath)
	if err != nil {
		return err
	}
	if path == s.Config().WorkspaceDir {
		return fmt.Errorf("cannot delete desktop workspace root")
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete desktop path: %w", err)
	}
	_ = s.Audit(ctx, "delete_path", s.relativePath(path), map[string]interface{}{}, source)
	return nil
}

// InstallApp stores a generated app manifest and writes its files under Apps/<id>.
func (s *Service) InstallApp(ctx context.Context, manifest AppManifest, files map[string]string, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	cfg := s.Config()
	if cfg.ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	if !cfg.AllowGeneratedApps {
		return fmt.Errorf("generated desktop apps are disabled")
	}
	manifest.ID = strings.ToLower(strings.TrimSpace(manifest.ID))
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Icon = strings.TrimSpace(manifest.Icon)
	manifest.Entry = cleanDesktopPath(manifest.Entry)
	manifest.Runtime = normalizeDesktopRuntime(manifest.Runtime)
	manifest.Permissions = normalizeDesktopPermissions(manifest.Permissions)
	if !desktopIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("invalid desktop app id")
	}
	if manifest.Name == "" {
		return fmt.Errorf("desktop app name is required")
	}
	icon, err := NormalizeDesktopIconName(manifest.Icon, "desktop app")
	if err != nil {
		return err
	}
	manifest.Icon = icon
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	if manifest.Entry == "." || strings.HasPrefix(manifest.Entry, "..") || filepath.IsAbs(manifest.Entry) {
		return fmt.Errorf("desktop app entry must be a relative file")
	}
	entryContent, ok := files[manifest.Entry]
	if !ok {
		return fmt.Errorf("desktop app entry file is missing")
	}
	if err := requireNonEmptyDesktopFile("app entry", entryContent); err != nil {
		return err
	}
	baseRel := filepath.ToSlash(filepath.Join("Apps", manifest.ID))
	for rel, content := range files {
		cleanRel := cleanDesktopPath(rel)
		if cleanRel == "." || strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			return fmt.Errorf("desktop app file path escapes app directory")
		}
		if err := s.WriteFile(ctx, filepath.ToSlash(filepath.Join(baseRel, cleanRel)), content, source); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	manifest.CreatedAt = now
	manifest.UpdatedAt = now
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal desktop app manifest: %w", err)
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_apps(id, name, version, icon, entry, manifest_json, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			version = excluded.version,
			icon = excluded.icon,
			entry = excluded.entry,
			manifest_json = excluded.manifest_json,
			updated_at = excluded.updated_at`,
		manifest.ID, manifest.Name, manifest.Version, manifest.Icon, manifest.Entry, string(manifestJSON), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save desktop app manifest: %w", err)
	}
	_ = s.Audit(ctx, "install_app", manifest.ID, manifest, source)
	return nil
}

// UpsertWidget creates or updates one desktop widget.
func (s *Service) UpsertWidget(ctx context.Context, widget Widget, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	widget.ID = strings.ToLower(strings.TrimSpace(widget.ID))
	widget.AppID = strings.ToLower(strings.TrimSpace(widget.AppID))
	widget.Title = strings.TrimSpace(widget.Title)
	widget.Type = strings.ToLower(strings.TrimSpace(widget.Type))
	widget.Icon = strings.TrimSpace(widget.Icon)
	widget.Entry = cleanOptionalDesktopFile(widget.Entry)
	widget.Runtime = normalizeDesktopRuntime(widget.Runtime)
	widget.Permissions = normalizeDesktopPermissions(widget.Permissions)
	if !desktopIDPattern.MatchString(widget.ID) {
		return fmt.Errorf("invalid desktop widget id")
	}
	if widget.AppID != "" && !desktopIDPattern.MatchString(widget.AppID) {
		return fmt.Errorf("invalid desktop widget app_id")
	}
	if widget.Title == "" {
		return fmt.Errorf("desktop widget title is required")
	}
	if widget.Type == "" {
		widget.Type = WidgetTypeCustom
	}
	if widget.Icon == "" {
		widget.Icon = "widgets"
	}
	icon, err := NormalizeDesktopIconName(widget.Icon, "desktop widget")
	if err != nil {
		return err
	}
	widget.Icon = icon
	if widget.Entry != "" {
		if widget.Entry == "." || strings.HasPrefix(widget.Entry, "..") || filepath.IsAbs(widget.Entry) {
			return fmt.Errorf("desktop widget entry must be a relative file")
		}
		if err := s.validateWidgetEntryFile(widget.AppID, widget.Entry); err != nil {
			return err
		}
	}
	if widget.W <= 0 {
		widget.W = 2
	}
	if widget.H <= 0 {
		widget.H = 2
	}
	if widget.Config == nil {
		widget.Config = map[string]interface{}{}
	}
	configJSON, err := json.Marshal(widget.Config)
	if err != nil {
		return fmt.Errorf("marshal desktop widget config: %w", err)
	}
	now := time.Now().UTC()
	widget.CreatedAt = now
	widget.UpdatedAt = now
	widgetJSON, err := json.Marshal(widget)
	if err != nil {
		return fmt.Errorf("marshal desktop widget: %w", err)
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_widgets(id, app_id, title, x, y, w, h, config_json, widget_json, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			app_id = excluded.app_id,
			title = excluded.title,
			x = excluded.x,
			y = excluded.y,
			w = excluded.w,
			h = excluded.h,
			config_json = excluded.config_json,
			widget_json = excluded.widget_json,
			updated_at = excluded.updated_at`,
		widget.ID, widget.AppID, widget.Title, widget.X, widget.Y, widget.W, widget.H, string(configJSON), string(widgetJSON), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save desktop widget: %w", err)
	}
	_ = s.Audit(ctx, "upsert_widget", widget.ID, widget, source)
	return nil
}

// DeleteWidget removes one desktop widget registration.
func (s *Service) DeleteWidget(ctx context.Context, id, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	id = strings.ToLower(strings.TrimSpace(id))
	if !desktopIDPattern.MatchString(id) {
		return fmt.Errorf("invalid desktop widget id")
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	if _, err := db.ExecContext(ctx, `DELETE FROM desktop_widgets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete desktop widget: %w", err)
	}
	_ = s.Audit(ctx, "delete_widget", id, map[string]interface{}{}, source)
	return nil
}

// SetSetting stores one validated desktop setting.
func (s *Service) SetSetting(ctx context.Context, key, value, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if err := validateDesktopSetting(key, value); err != nil {
		return err
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `INSERT INTO desktop_settings(key, value, updated_at)
		VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now); err != nil {
		return fmt.Errorf("save desktop setting: %w", err)
	}
	_ = s.Audit(ctx, "set_setting", key, map[string]interface{}{"value": value}, source)
	return nil
}

// SetSettings stores multiple validated desktop settings atomically.
func (s *Service) SetSettings(ctx context.Context, values map[string]string, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	if len(values) == 0 {
		return nil
	}
	for key, value := range values {
		if err := validateDesktopSetting(strings.TrimSpace(key), strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin desktop settings transaction: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO desktop_settings(key, value, updated_at)
		VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`)
	if err != nil {
		return fmt.Errorf("prepare desktop settings update: %w", err)
	}
	defer stmt.Close()
	for key, value := range values {
		if _, err := stmt.ExecContext(ctx, strings.TrimSpace(key), strings.TrimSpace(value), now); err != nil {
			return fmt.Errorf("save desktop setting %s: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit desktop settings: %w", err)
	}
	_ = s.Audit(ctx, "set_settings", "desktop_settings", values, source)
	return nil
}

func validateDesktopSetting(key, value string) error {
	for _, def := range DesktopSettingDefinitions() {
		if def.Key != key {
			continue
		}
		for _, allowed := range def.Values {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("invalid desktop setting value for %s", key)
	}
	return fmt.Errorf("unsupported desktop setting %s", key)
}

func (s *Service) validateWidgetEntryFile(appID, entry string) error {
	base := "Widgets"
	if appID != "" {
		base = filepath.ToSlash(filepath.Join("Apps", appID))
	}
	path, err := s.ResolvePath(filepath.ToSlash(filepath.Join(base, entry)))
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("desktop widget entry file is missing")
		}
		return fmt.Errorf("read desktop widget entry file: %w", err)
	}
	return requireNonEmptyDesktopFile("widget entry", string(content))
}

func requireNonEmptyDesktopFile(label, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("desktop %s file must not be empty", label)
	}
	return nil
}

// Audit records one desktop operation.
func (s *Service) Audit(ctx context.Context, action, target string, details interface{}, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(source) == "" {
		source = SourceUser
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal desktop audit details: %w", err)
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_audit(action, target, source, details_json, created_at)
		VALUES(?, ?, ?, ?, ?)`, action, target, source, string(detailsJSON), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("write desktop audit: %w", err)
	}
	return nil
}

func (s *Service) listApps(ctx context.Context) ([]AppManifest, error) {
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	rows, err := db.QueryContext(ctx, `SELECT manifest_json, created_at, updated_at FROM desktop_apps ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list desktop apps: %w", err)
	}
	defer rows.Close()
	var apps []AppManifest
	for rows.Next() {
		var manifestJSON, createdAt, updatedAt string
		if err := rows.Scan(&manifestJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan desktop app: %w", err)
		}
		var app AppManifest
		if err := json.Unmarshal([]byte(manifestJSON), &app); err != nil {
			return nil, fmt.Errorf("parse desktop app manifest: %w", err)
		}
		app.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		app.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (s *Service) listWidgets(ctx context.Context) ([]Widget, error) {
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	rows, err := db.QueryContext(ctx, `SELECT id, app_id, title, x, y, w, h, config_json, widget_json, created_at, updated_at FROM desktop_widgets ORDER BY y, x, title COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list desktop widgets: %w", err)
	}
	defer rows.Close()
	var widgets []Widget
	for rows.Next() {
		var widget Widget
		var configJSON, widgetJSON, createdAt, updatedAt string
		if err := rows.Scan(&widget.ID, &widget.AppID, &widget.Title, &widget.X, &widget.Y, &widget.W, &widget.H, &configJSON, &widgetJSON, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan desktop widget: %w", err)
		}
		if strings.TrimSpace(widgetJSON) != "" && strings.TrimSpace(widgetJSON) != "{}" {
			_ = json.Unmarshal([]byte(widgetJSON), &widget)
		}
		if strings.TrimSpace(configJSON) != "" {
			_ = json.Unmarshal([]byte(configJSON), &widget.Config)
		}
		if widget.Type == "" {
			widget.Type = WidgetTypeCustom
		}
		if widget.Icon == "" {
			widget.Icon = "widgets"
		}
		if widget.Runtime == "" {
			widget.Runtime = AuraDesktopRuntime
		}
		widget.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		widget.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		widgets = append(widgets, widget)
	}
	return widgets, rows.Err()
}

func (s *Service) listSettings(ctx context.Context) (map[string]string, error) {
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	rows, err := db.QueryContext(ctx, `SELECT key, value FROM desktop_settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list desktop settings: %w", err)
	}
	defer rows.Close()
	settings := DesktopSettingDefaults()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan desktop setting: %w", err)
		}
		if err := validateDesktopSetting(key, value); err != nil {
			continue
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

func cleanOptionalDesktopFile(rawPath string) string {
	if strings.TrimSpace(rawPath) == "" {
		return ""
	}
	return cleanDesktopPath(rawPath)
}

func normalizeDesktopRuntime(runtime string) string {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return AuraDesktopRuntime
	}
	return runtime
}

func normalizeDesktopPermissions(permissions []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		permission = strings.ToLower(strings.TrimSpace(permission))
		if permission == "" {
			continue
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		result = append(result, permission)
	}
	return result
}
