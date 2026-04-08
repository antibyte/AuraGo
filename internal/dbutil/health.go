package dbutil

import (
	"database/sql"
	"fmt"
)

// HealthCheck performs a basic integrity check on the database.
func HealthCheck(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	var result string
	err := db.QueryRow("PRAGMA integrity_check(1)").Scan(&result)
	if err != nil {
		return fmt.Errorf("integrity check query failed: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	return nil
}
