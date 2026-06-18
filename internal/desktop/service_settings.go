package desktop

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
	db := s.getDB()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `INSERT INTO desktop_settings(key, value, updated_at)
		VALUES(?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now); err != nil {
		return fmt.Errorf("save desktop setting: %w", err)
	}
	_ = s.Audit(ctx, "set_setting", key, map[string]interface{}{"value": value}, source)
	s.InvalidateSettings()
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
	db := s.getDB()
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
	s.InvalidateSettings()
	return nil
}

func validateDesktopSetting(key, value string) error {
	for _, def := range DesktopSettingDefinitions() {
		if def.Key != key {
			continue
		}
		if len(def.Values) == 0 {
			return validateFreeformDesktopSetting(key, value)
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

func validateFreeformDesktopSetting(key, value string) error {
	switch key {
	case "agent.provider":
		if len(value) > 128 {
			return fmt.Errorf("invalid desktop setting value for %s", key)
		}
		for _, r := range value {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ':' || r == '/' {
				continue
			}
			return fmt.Errorf("invalid desktop setting value for %s", key)
		}
		return nil
	case "pet.active_id":
		if value == "" {
			return nil
		}
		if !petIDPattern.MatchString(value) {
			return fmt.Errorf("invalid desktop setting value for %s", key)
		}
		return nil
	case "pet.scale", "pet.position_x", "pet.position_y":
		if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err != nil {
			return fmt.Errorf("invalid desktop setting value for %s", key)
		}
		return nil
	default:
		return fmt.Errorf("invalid desktop setting value for %s", key)
	}
}

func (s *Service) listSettings(ctx context.Context) (map[string]string, error) {
	db := s.getDB()
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
