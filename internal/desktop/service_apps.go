package desktop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	permissions, err := normalizeDesktopPermissions(manifest.Permissions)
	if err != nil {
		return err
	}
	manifest.Permissions = permissions
	if !desktopIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("invalid desktop app id")
	}
	if manifest.Name == "" {
		return fmt.Errorf("desktop app name is required")
	}
	if manifest.Icon == "" {
		manifest.Icon = InferDesktopIconName(manifest.ID, manifest.Name, manifest.Entry, manifest.Description)
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
	db := s.getDB()
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
	s.invalidateBootstrapCache()
	return nil
}

// DeleteApp removes one generated app from the start menu, desktop shortcuts,
// widgets, and workspace app files. Built-in apps are never deleted.
func (s *Service) DeleteApp(ctx context.Context, id, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	id = strings.ToLower(strings.TrimSpace(id))
	if !desktopIDPattern.MatchString(id) {
		return fmt.Errorf("invalid desktop app id")
	}
	for _, app := range BuiltinApps() {
		if app.ID == id {
			return fmt.Errorf("built-in desktop apps cannot be deleted")
		}
	}
	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	appDir, err := s.ResolvePath(filepath.ToSlash(filepath.Join("Apps", id)))
	if err != nil {
		return err
	}
	stagedDir := appDir + ".delete-" + time.Now().UTC().Format("20060102150405.000000000")
	appDirStaged := false
	if _, err := os.Stat(appDir); err == nil {
		if err := os.Rename(appDir, stagedDir); err != nil {
			return fmt.Errorf("stage desktop app files for delete: %w", err)
		}
		appDirStaged = true
		defer func() {
			if appDirStaged {
				_ = os.Rename(stagedDir, appDir)
			}
		}()
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat desktop app files: %w", err)
	}
	db := s.getDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin desktop app delete: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM desktop_apps WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete desktop app manifest: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("desktop app not found")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_shortcuts WHERE target_type = ? AND target_id = ?`, ShortcutTargetApp, id); err != nil {
		return fmt.Errorf("delete desktop app shortcuts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_widgets WHERE app_id = ?`, id); err != nil {
		return fmt.Errorf("delete desktop app widgets: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_app_visibility WHERE app_id = ?`, id); err != nil {
		return fmt.Errorf("delete desktop app visibility: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit desktop app delete: %w", err)
	}
	if appDirStaged {
		appDirStaged = false
	}
	removeTarget := appDir
	if stagedDir != "" {
		removeTarget = stagedDir
	}
	if err := os.RemoveAll(removeTarget); err != nil {
		return fmt.Errorf("delete desktop app files: %w", err)
	}
	_ = s.Audit(ctx, "delete_app", id, map[string]interface{}{}, source)
	s.invalidateBootstrapCache()
	return nil
}

// SetAppVisibility toggles whether an app appears in the dock and start menu.
func (s *Service) SetAppVisibility(ctx context.Context, id string, dockVisible, startVisible *bool, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	id = strings.ToLower(strings.TrimSpace(id))
	if !desktopIDPattern.MatchString(id) {
		return fmt.Errorf("invalid desktop app id")
	}
	if dockVisible == nil && startVisible == nil {
		return fmt.Errorf("dock_visible or start_visible field is required")
	}
	if _, ok, err := s.findApp(ctx, id); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("desktop app not found")
	}
	visibility := defaultAppVisibility()
	db := s.getDB()
	var existingDock, existingStart int
	err := db.QueryRowContext(ctx, `SELECT dock_visible, start_visible FROM desktop_app_visibility WHERE app_id = ?`, id).Scan(&existingDock, &existingStart)
	if err == nil {
		visibility.DockVisible = existingDock != 0
		visibility.StartVisible = existingStart != 0
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("read desktop app visibility: %w", err)
	}
	if dockVisible != nil {
		visibility.DockVisible = *dockVisible
	}
	if startVisible != nil {
		visibility.StartVisible = *startVisible
	}
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_app_visibility(app_id, dock_visible, start_visible, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(app_id) DO UPDATE SET
			dock_visible = excluded.dock_visible,
			start_visible = excluded.start_visible,
			updated_at = excluded.updated_at`,
		id, boolToInt(visibility.DockVisible), boolToInt(visibility.StartVisible), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("update desktop app visibility: %w", err)
	}
	_ = s.Audit(ctx, "set_app_visibility", id, map[string]interface{}{
		"dock_visible":  visibility.DockVisible,
		"start_visible": visibility.StartVisible,
	}, source)
	s.invalidateBootstrapCache()
	return nil
}

func requireNonEmptyDesktopFile(label, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("desktop %s file must not be empty", label)
	}
	return nil
}

func (s *Service) listApps(ctx context.Context) ([]AppManifest, error) {
	db := s.getDB()
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
		app.Builtin = false
		app.Deletable = true
		app.DockVisible = true
		app.StartVisible = true
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (s *Service) findApp(ctx context.Context, id string) (AppManifest, bool, error) {
	for _, app := range BuiltinApps() {
		if app.ID == id {
			return app, true, nil
		}
	}
	apps, err := s.listApps(ctx)
	if err != nil {
		return AppManifest{}, false, err
	}
	for _, app := range apps {
		if app.ID == id {
			return app, true, nil
		}
	}
	return AppManifest{}, false, nil
}

func (s *Service) listAppVisibility(ctx context.Context) (map[string]appVisibility, error) {
	db := s.getDB()
	rows, err := db.QueryContext(ctx, `SELECT app_id, dock_visible, start_visible FROM desktop_app_visibility`)
	if err != nil {
		return nil, fmt.Errorf("list desktop app visibility: %w", err)
	}
	defer rows.Close()
	visibility := map[string]appVisibility{}
	for rows.Next() {
		var id string
		var dockVisible, startVisible int
		if err := rows.Scan(&id, &dockVisible, &startVisible); err != nil {
			return nil, fmt.Errorf("scan desktop app visibility: %w", err)
		}
		visibility[strings.ToLower(id)] = appVisibility{
			DockVisible:  dockVisible != 0,
			StartVisible: startVisible != 0,
		}
	}
	return visibility, rows.Err()
}

type appVisibility struct {
	DockVisible  bool
	StartVisible bool
}

func defaultAppVisibility() appVisibility {
	return appVisibility{DockVisible: true, StartVisible: true}
}

func applyAppVisibility(apps []AppManifest, builtin bool, visibility map[string]appVisibility) []AppManifest {
	out := make([]AppManifest, 0, len(apps))
	for _, app := range apps {
		app.Builtin = builtin
		app.Deletable = !builtin
		v, ok := visibility[strings.ToLower(app.ID)]
		if !ok {
			v = defaultAppVisibility()
		}
		app.DockVisible = v.DockVisible
		app.StartVisible = v.StartVisible
		out = append(out, app)
	}
	return out
}
