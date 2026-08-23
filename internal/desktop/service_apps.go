package desktop

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	normalizedFiles := make(map[string][]byte, len(files))
	fileRels := make([]string, 0, len(files))
	maxBytes := int64(cfg.MaxFileSizeMB) * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 50 * 1024 * 1024
	}
	for rel, content := range files {
		cleanRel := cleanDesktopPath(rel)
		if cleanRel == "." || strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			return fmt.Errorf("desktop app file path escapes app directory")
		}
		if _, exists := normalizedFiles[cleanRel]; exists {
			return fmt.Errorf("desktop app contains duplicate normalized file path %q", cleanRel)
		}
		data := []byte(content)
		if int64(len(data)) > maxBytes {
			return fmt.Errorf("desktop app file %q exceeds max size", cleanRel)
		}
		normalizedFiles[cleanRel] = data
		fileRels = append(fileRels, cleanRel)
	}
	entryContent, ok := normalizedFiles[manifest.Entry]
	if !ok {
		return fmt.Errorf("desktop app entry file is missing")
	}
	if err := requireNonEmptyDesktopFile("app entry", string(entryContent)); err != nil {
		return err
	}
	sort.Strings(fileRels)
	baseRel := filepath.ToSlash(filepath.Join("Apps", manifest.ID))
	hashes := make(map[string]string, len(normalizedFiles))
	for rel, data := range normalizedFiles {
		sum := sha256.Sum256(data)
		hashes[rel] = "sha256:" + hex.EncodeToString(sum[:])
	}
	signature, err := s.signDesktopIntegrity("app", manifest.ID, hashes)
	if err != nil {
		return fmt.Errorf("build desktop app integrity: %w", err)
	}
	manifest.Integrity = &IntegrityData{
		Hashes: hashes,
		Signature: &IntegritySignature{
			Algorithm: "ed25519",
			Value:     signature,
		},
	}
	now := time.Now().UTC()
	manifest.CreatedAt = now
	manifest.UpdatedAt = now
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal desktop app manifest: %w", err)
	}
	appDir, err := s.resolveWorkspacePathNoSymlinks(baseRel, true)
	if err != nil {
		return err
	}
	appsDir := filepath.Dir(appDir)
	workspaceRoot, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		return fmt.Errorf("resolve desktop workspace: %w", err)
	}

	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if err := os.MkdirAll(appsDir, 0o700); err != nil {
		return fmt.Errorf("create desktop apps directory: %w", err)
	}
	if err := validateNoSymlinkComponents(workspaceRoot, appsDir, false); err != nil {
		return err
	}
	stagingDir, err := os.MkdirTemp(appsDir, "."+manifest.ID+".install-")
	if err != nil {
		return fmt.Errorf("create desktop app staging directory: %w", err)
	}
	defer func() {
		if stagingDir != "" {
			_ = os.RemoveAll(stagingDir)
		}
	}()
	_ = os.Chmod(stagingDir, 0o700)
	for _, rel := range fileRels {
		target := filepath.Join(stagingDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("create desktop app staging path %q: %w", rel, err)
		}
		if err := validateNoSymlinkComponents(stagingDir, filepath.Dir(target), false); err != nil {
			return err
		}
		if _, err := secureWriteWorkspaceFile(target, normalizedFiles[rel]); err != nil {
			return fmt.Errorf("stage desktop app file %q: %w", rel, err)
		}
	}

	backupDir := appDir + ".replace-" + now.Format("20060102150405.000000000")
	oldStaged := false
	if _, err := os.Stat(appDir); err == nil {
		if err := os.Rename(appDir, backupDir); err != nil {
			return fmt.Errorf("stage previous desktop app files: %w", err)
		}
		oldStaged = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat previous desktop app files: %w", err)
	}
	rollbackFiles := func() error {
		removeErr := os.RemoveAll(appDir)
		if oldStaged {
			if restoreErr := os.Rename(backupDir, appDir); restoreErr != nil {
				return fmt.Errorf("remove replacement: %v; restore previous files: %w", removeErr, restoreErr)
			}
		}
		return removeErr
	}
	if err := os.Rename(stagingDir, appDir); err != nil {
		if oldStaged {
			_ = os.Rename(backupDir, appDir)
		}
		return fmt.Errorf("activate desktop app files: %w", err)
	}
	stagingDir = ""

	db := s.getDB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		_ = rollbackFiles()
		return fmt.Errorf("begin desktop app install: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO desktop_apps(id, name, version, icon, entry, manifest_json, created_at, updated_at)
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
		_ = rollbackFiles()
		return fmt.Errorf("save desktop app manifest: %w", err)
	}
	if err := tx.Commit(); err != nil {
		rollbackErr := rollbackFiles()
		if rollbackErr != nil {
			return fmt.Errorf("commit desktop app manifest: %v; rollback files: %w", err, rollbackErr)
		}
		return fmt.Errorf("commit desktop app manifest: %w", err)
	}
	if oldStaged {
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("desktop app installed but previous files could not be removed: %w", err)
		}
	}
	_ = s.Audit(ctx, "install_app", manifest.ID, manifest, source)
	s.InvalidateApps()
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
	s.InvalidateApps()
	s.InvalidateWidgets()
	s.InvalidateShortcuts()
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
	s.InvalidateApps()
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

func (s *Service) validateGeneratedAppEntry(ctx context.Context, app AppManifest) AppManifest {
	app.EntryPath = filepath.ToSlash(filepath.Join("Apps", app.ID, app.Entry))
	entryPath, err := s.resolveWorkspacePathNoSymlinks(app.EntryPath, true)
	if err != nil {
		app.Health = "broken"
		app.HealthReason = "invalid_entry_path"
		return app
	}
	data, _, err := secureReadWorkspaceFile(entryPath)
	if err != nil {
		app.Health = "broken"
		if os.IsNotExist(err) {
			app.HealthReason = "missing_entry_file"
		} else {
			app.HealthReason = "unreadable_entry_file"
		}
		return app
	}
	if strings.TrimSpace(string(data)) == "" {
		app.Health = "broken"
		app.HealthReason = "empty_entry_file"
		return app
	}
	if reason := s.verifyDesktopIntegrity("app", app.ID, filepath.ToSlash(filepath.Join("Apps", app.ID)), app.Integrity); reason != "" {
		app.Health = "broken"
		app.HealthReason = reason
		return app
	}
	app.Health = ""
	app.HealthReason = ""
	return app
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
			v = appVisibility{DockVisible: app.DockVisible, StartVisible: app.StartVisible}
		}
		if app.Internal {
			v.DockVisible = false
			v.StartVisible = false
		}
		app.DockVisible = v.DockVisible
		app.StartVisible = v.StartVisible
		out = append(out, app)
	}
	return out
}
