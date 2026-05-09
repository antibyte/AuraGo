package desktop

import (
	"context"
	"fmt"
	"strings"
	"time"
)

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
	s.invalidateBootstrapCache()
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
	if shortcut.TargetType != ShortcutTargetApp && shortcut.TargetType != ShortcutTargetDirectory {
		return fmt.Errorf("unsupported desktop shortcut target type")
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
	s.invalidateBootstrapCache()
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
