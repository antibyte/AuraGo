package desktop

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// dashboardTargetIDRe is the HomeDash dashboard id form; keep in sync with internal/homedash/ids.go.
var dashboardTargetIDRe = regexp.MustCompile(`^d_[a-z2-7]{12}$`)

// DashboardShortcutID is the shortcut id of a HomeDash dashboard.
func DashboardShortcutID(dashboardID string) string { return "dashboard-" + dashboardID }

// AddDesktopDashboardShortcut pins a HomeDash dashboard to the desktop. The caller has already
// validated the dashboard and passes its current name.
func (s *Service) AddDesktopDashboardShortcut(ctx context.Context, dashboardID, name, source string) (Shortcut, error) {
	if err := s.ensureReady(ctx); err != nil {
		return Shortcut{}, err
	}
	if s.Config().ReadOnly {
		return Shortcut{}, fmt.Errorf("virtual desktop is read-only")
	}
	dashboardID = strings.ToLower(strings.TrimSpace(dashboardID))
	shortcut := Shortcut{
		ID:         DashboardShortcutID(dashboardID),
		TargetType: ShortcutTargetDashboard,
		TargetID:   dashboardID,
		Name:       name,
		Icon:       "homedash",
	}
	if err := s.upsertDesktopShortcut(ctx, shortcut, source); err != nil {
		return Shortcut{}, err
	}
	shorts, err := s.listShortcuts(ctx)
	if err != nil {
		return Shortcut{}, err
	}
	for _, sc := range shorts {
		if sc.ID == shortcut.ID {
			return sc, nil
		}
	}
	return shortcut, nil
}

// RenameTargetShortcuts sets the label of every shortcut for the target. It reads first and
// writes only when a name differs, so ordinary autosaves take no write lock.
func (s *Service) RenameTargetShortcuts(ctx context.Context, targetType, targetID, name, source string) (bool, error) {
	if !s.isReady() { // hooks never (re)open a service
		return false, nil
	}
	if s.Config().ReadOnly {
		return false, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}
	db := s.getDB()
	var current string
	err := db.QueryRowContext(ctx, `SELECT name FROM desktop_shortcuts WHERE target_type = ? AND target_id = ? AND name <> ? LIMIT 1`,
		targetType, targetID, name).Scan(&current)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read desktop shortcut name: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := db.ExecContext(ctx, `UPDATE desktop_shortcuts SET name = ?, updated_at = ? WHERE target_type = ? AND target_id = ? AND name <> ?`,
		name, now, targetType, targetID, name)
	if err != nil {
		return false, fmt.Errorf("rename desktop shortcut: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	_ = s.Audit(ctx, "rename_shortcut", targetType+"-"+targetID, map[string]interface{}{"name": name}, source)
	s.InvalidateShortcuts()
	return true, nil
}

// RemoveTargetShortcuts deletes every shortcut for the target and reports whether any existed.
func (s *Service) RemoveTargetShortcuts(ctx context.Context, targetType, targetID, source string) (bool, error) {
	if !s.isReady() { // hooks never (re)open a service
		return false, nil
	}
	if s.Config().ReadOnly {
		return false, nil
	}
	res, err := s.getDB().ExecContext(ctx, `DELETE FROM desktop_shortcuts WHERE target_type = ? AND target_id = ?`, targetType, targetID)
	if err != nil {
		return false, fmt.Errorf("remove desktop shortcuts: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	_ = s.Audit(ctx, "remove_shortcut", targetType+"-"+targetID, map[string]interface{}{}, source)
	s.InvalidateShortcuts()
	return true, nil
}

// AddDesktopAppShortcut pins an existing built-in or installed app to the desktop.
func (s *Service) AddDesktopAppShortcut(ctx context.Context, appID, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	appID = strings.ToLower(strings.TrimSpace(appID))
	app, ok, err := s.findApp(ctx, appID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("desktop app not found")
	}
	shortcut := Shortcut{
		ID:         "app-" + app.ID,
		TargetType: ShortcutTargetApp,
		TargetID:   app.ID,
		Name:       app.Name,
		Icon:       iconOrDefault(app.Icon, InferDesktopIconName(app.ID, app.Name)),
	}
	return s.upsertDesktopShortcut(ctx, shortcut, source)
}

// RemoveDesktopShortcut removes one pinned desktop icon without uninstalling the app.
func (s *Service) RemoveDesktopShortcut(ctx context.Context, id, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if s.Config().ReadOnly {
		return fmt.Errorf("virtual desktop is read-only")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("desktop shortcut id is required")
	}
	db := s.getDB()
	if _, err := db.ExecContext(ctx, `DELETE FROM desktop_shortcuts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("remove desktop shortcut: %w", err)
	}
	_ = s.Audit(ctx, "remove_shortcut", id, map[string]interface{}{}, source)
	s.InvalidateShortcuts()
	return nil
}

func (s *Service) upsertDesktopShortcut(ctx context.Context, shortcut Shortcut, source string) error {
	shortcut.ID = strings.TrimSpace(shortcut.ID)
	shortcut.TargetType = strings.ToLower(strings.TrimSpace(shortcut.TargetType))
	shortcut.TargetID = strings.ToLower(strings.TrimSpace(shortcut.TargetID))
	shortcut.Path = cleanDesktopPathSlash(shortcut.Path)
	shortcut.Name = strings.TrimSpace(shortcut.Name)
	shortcut.Icon = strings.TrimSpace(shortcut.Icon)
	if shortcut.ID == "" {
		return fmt.Errorf("desktop shortcut id is required")
	}
	if shortcut.TargetType != ShortcutTargetApp && shortcut.TargetType != ShortcutTargetDirectory && shortcut.TargetType != ShortcutTargetDashboard {
		return fmt.Errorf("unsupported desktop shortcut target type")
	}
	if shortcut.TargetType == ShortcutTargetDashboard && !dashboardTargetIDRe.MatchString(shortcut.TargetID) {
		return fmt.Errorf("desktop shortcut dashboard id is invalid")
	}
	if shortcut.TargetType == ShortcutTargetApp && shortcut.TargetID == "" {
		return fmt.Errorf("desktop shortcut app id is required")
	}
	if shortcut.TargetType == ShortcutTargetDirectory {
		if shortcut.Path == "." {
			return fmt.Errorf("desktop shortcut path is required")
		}
		if _, err := s.ResolvePath(shortcut.Path); err != nil {
			return err
		}
	}
	if shortcut.Name == "" {
		shortcut.Name = shortcut.TargetID
	}
	if shortcut.Icon == "" {
		shortcut.Icon = InferDesktopIconName(shortcut.TargetID, shortcut.Name, shortcut.Path)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	db := s.getDB()
	if _, err := db.ExecContext(ctx, `INSERT INTO desktop_shortcuts(id, target_type, target_id, path, name, icon, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			target_type = excluded.target_type,
			target_id = excluded.target_id,
			path = excluded.path,
			name = excluded.name,
			icon = excluded.icon,
			updated_at = excluded.updated_at`,
		shortcut.ID, shortcut.TargetType, shortcut.TargetID, shortcut.Path, shortcut.Name, shortcut.Icon, now, now); err != nil {
		return fmt.Errorf("save desktop shortcut: %w", err)
	}
	_ = s.Audit(ctx, "upsert_shortcut", shortcut.ID, shortcut, source)
	s.InvalidateShortcuts()
	return nil
}

func iconOrDefault(icon, fallback string) string {
	icon = strings.TrimSpace(icon)
	if icon != "" {
		return icon
	}
	return fallback
}

func (s *Service) listShortcuts(ctx context.Context) ([]Shortcut, error) {
	db := s.getDB()
	rows, err := db.QueryContext(ctx, `SELECT id, target_type, target_id, path, name, icon, created_at, updated_at
		FROM desktop_shortcuts ORDER BY created_at, name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list desktop shortcuts: %w", err)
	}
	defer rows.Close()
	var shortcuts []Shortcut
	for rows.Next() {
		var shortcut Shortcut
		var createdAt, updatedAt string
		if err := rows.Scan(&shortcut.ID, &shortcut.TargetType, &shortcut.TargetID, &shortcut.Path, &shortcut.Name, &shortcut.Icon, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan desktop shortcut: %w", err)
		}
		shortcut.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		shortcut.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		shortcuts = append(shortcuts, shortcut)
	}
	return shortcuts, rows.Err()
}
