package planner

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetPlannerMeta returns a planner_meta value or an empty string when unset.
func GetPlannerMeta(db *sql.DB, key string) (string, error) {
	if db == nil {
		return "", nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("planner meta key is required")
	}
	var value string
	err := db.QueryRow(`SELECT value FROM planner_meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get planner meta %s: %w", key, err)
	}
	return value, nil
}

// SetPlannerMeta upserts a planner_meta value.
func SetPlannerMeta(db *sql.DB, key, value string) error {
	if db == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("planner meta key is required")
	}
	if _, err := db.Exec(`
		INSERT INTO planner_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value); err != nil {
		return fmt.Errorf("set planner meta %s: %w", key, err)
	}
	return nil
}