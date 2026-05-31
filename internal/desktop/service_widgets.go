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
	permissions, err := normalizeDesktopPermissions(widget.Permissions)
	if err != nil {
		return err
	}
	widget.Permissions = permissions
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
		widget.Icon = InferDesktopIconName(widget.ID, widget.Title, widget.Type, widget.Entry, widget.AppID)
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
	if !widget.Builtin {
		var existingVisible int
		db := s.getDB()
		err = db.QueryRowContext(ctx, `SELECT COALESCE(visible, 1) FROM desktop_widgets WHERE id = ?`, widget.ID).Scan(&existingVisible)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check desktop widget visibility: %w", err)
		}
		if err == sql.ErrNoRows {
			widget.Visible = true
		} else {
			widget.Visible = existingVisible != 0
		}
	}
	if !widget.Builtin && widget.Entry != "" {
		baseRel := widgetBaseRel(widget)
		widget.Integrity, err = s.buildDesktopIntegrity("widget", widget.ID, baseRel, []string{widget.Entry})
		if err != nil {
			return fmt.Errorf("build desktop widget integrity: %w", err)
		}
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
	db := s.getDB()
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_widgets(id, app_id, title, x, y, w, h, config_json, widget_json, visible, builtin, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			app_id = excluded.app_id,
			title = excluded.title,
			x = excluded.x,
			y = excluded.y,
			w = excluded.w,
			h = excluded.h,
			config_json = excluded.config_json,
			widget_json = excluded.widget_json,
			visible = excluded.visible,
			builtin = excluded.builtin,
			updated_at = excluded.updated_at`,
		widget.ID, widget.AppID, widget.Title, widget.X, widget.Y, widget.W, widget.H, string(configJSON), string(widgetJSON), boolToInt(widget.Visible), boolToInt(widget.Builtin), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save desktop widget: %w", err)
	}
	_ = s.Audit(ctx, "upsert_widget", widget.ID, widget, source)
	s.InvalidateWidgets()
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
	db := s.getDB()
	var isBuiltin int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(builtin, 0) FROM desktop_widgets WHERE id = ?`, id).Scan(&isBuiltin); err != nil {
		return fmt.Errorf("desktop widget not found")
	}
	if isBuiltin != 0 {
		return fmt.Errorf("built-in desktop widgets cannot be deleted")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM desktop_widgets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete desktop widget: %w", err)
	}
	_ = s.Audit(ctx, "delete_widget", id, map[string]interface{}{}, source)
	s.InvalidateWidgets()
	return nil
}

// SetWidgetVisible toggles the visibility of a desktop widget.
func (s *Service) SetWidgetVisible(ctx context.Context, id string, visible bool, source string) error {
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
	db := s.getDB()
	result, err := db.ExecContext(ctx, `UPDATE desktop_widgets SET visible = ?, updated_at = ? WHERE id = ?`,
		boolToInt(visible), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update desktop widget visibility: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("desktop widget not found")
	}
	_ = s.Audit(ctx, "set_widget_visible", id, map[string]interface{}{"visible": visible}, source)
	s.InvalidateWidgets()
	return nil
}

func (s *Service) validateWidgetEntryFile(appID, entry string) error {
	base := "Widgets"
	if appID != "" {
		base = filepath.ToSlash(filepath.Join("Apps", appID))
	}
	path, err := s.resolveWorkspacePathNoSymlinks(filepath.ToSlash(filepath.Join(base, entry)), true)
	if err != nil {
		return err
	}
	content, _, err := secureReadWorkspaceFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("desktop widget entry file is missing")
		}
		return fmt.Errorf("read desktop widget entry file: %w", err)
	}
	return requireNonEmptyDesktopFile("widget entry", string(content))
}

func (s *Service) listWidgets(ctx context.Context) ([]Widget, error) {
	return s.ListAllWidgets(ctx)
}

func (s *Service) ListAllWidgets(ctx context.Context) ([]Widget, error) {
	db := s.getDB()
	rows, err := db.QueryContext(ctx, `SELECT id, app_id, title, x, y, w, h, config_json, widget_json, visible, builtin, created_at, updated_at FROM desktop_widgets ORDER BY y, x, title COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list desktop widgets: %w", err)
	}
	defer rows.Close()
	var widgets []Widget
	for rows.Next() {
		var widget Widget
		var configJSON, widgetJSON, createdAt, updatedAt string
		var visible, builtin int
		if err := rows.Scan(&widget.ID, &widget.AppID, &widget.Title, &widget.X, &widget.Y, &widget.W, &widget.H, &configJSON, &widgetJSON, &visible, &builtin, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan desktop widget: %w", err)
		}
		if strings.TrimSpace(widgetJSON) != "" && strings.TrimSpace(widgetJSON) != "{}" {
			_ = json.Unmarshal([]byte(widgetJSON), &widget)
		}
		widget.Visible = visible != 0
		widget.Builtin = builtin != 0
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
		widget = s.validateWidgetEntry(widget)
		widgets = append(widgets, widget)
	}
	return widgets, rows.Err()
}

func (s *Service) validateWidgetEntry(widget Widget) Widget {
	if widget.Entry == "" {
		widget.Health = ""
		widget.HealthReason = ""
		widget.EntryPath = ""
		return widget
	}
	baseRel := widgetBaseRel(widget)
	widget.EntryPath = filepath.ToSlash(filepath.Join(baseRel, widget.Entry))
	entryPath, err := s.resolveWorkspacePathNoSymlinks(widget.EntryPath, true)
	if err != nil {
		widget.Health = "broken"
		widget.HealthReason = "invalid_entry_path"
		return widget
	}
	data, _, err := secureReadWorkspaceFile(entryPath)
	if err != nil {
		widget.Health = "broken"
		if os.IsNotExist(err) {
			widget.HealthReason = "missing_entry_file"
		} else {
			widget.HealthReason = "unreadable_entry_file"
		}
		return widget
	}
	if strings.TrimSpace(string(data)) == "" {
		widget.Health = "broken"
		widget.HealthReason = "empty_entry_file"
		return widget
	}
	if reason := s.verifyDesktopIntegrity("widget", widget.ID, baseRel, widget.Integrity); reason != "" {
		widget.Health = "broken"
		widget.HealthReason = reason
		return widget
	}
	widget.Health = ""
	widget.HealthReason = ""
	return widget
}

func widgetBaseRel(widget Widget) string {
	if strings.TrimSpace(widget.AppID) != "" {
		return filepath.ToSlash(filepath.Join("Apps", widget.AppID))
	}
	return "Widgets"
}

func cleanOptionalDesktopFile(rawPath string) string {
	if strings.TrimSpace(rawPath) == "" {
		return ""
	}
	return filepath.ToSlash(cleanDesktopPath(rawPath))
}
