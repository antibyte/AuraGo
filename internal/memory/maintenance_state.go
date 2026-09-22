package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetMemoryMaintenanceState returns a persisted maintenance value. Missing
// keys are represented by an empty value so first-run maintenance starts at
// the beginning of its keyset.
func (s *SQLiteMemory) GetMemoryMaintenanceState(key string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("memory maintenance state store is unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("memory maintenance state key is required")
	}
	var value string
	err := s.db.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get memory maintenance state %s: %w", key, err)
	}
	return value, nil
}

// SetMemoryMaintenanceState upserts a persisted maintenance value.
func (s *SQLiteMemory) SetMemoryMaintenanceState(key, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory maintenance state store is unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory maintenance state key is required")
	}
	if _, err := s.db.Exec(`
		INSERT INTO memory_maintenance_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value); err != nil {
		return fmt.Errorf("set memory maintenance state %s: %w", key, err)
	}
	return nil
}

// ClearMemoryMaintenanceState removes a persisted maintenance value.
func (s *SQLiteMemory) ClearMemoryMaintenanceState(key string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory maintenance state store is unavailable")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory maintenance state key is required")
	}
	if _, err := s.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key = ?`, key); err != nil {
		return fmt.Errorf("clear memory maintenance state %s: %w", key, err)
	}
	return nil
}
