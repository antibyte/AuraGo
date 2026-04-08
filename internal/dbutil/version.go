package dbutil

import (
	"database/sql"
	"fmt"
)

// GetUserVersion returns the current schema version from PRAGMA user_version.
func GetUserVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("get user_version: %w", err)
	}
	return version, nil
}

// SetUserVersion sets the schema version via PRAGMA user_version.
// Version must be >= 0.
func SetUserVersion(db *sql.DB, version int) error {
	if version < 0 {
		return fmt.Errorf("invalid version %d: version must be >= 0", version)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return fmt.Errorf("set user_version to %d: %w", version, err)
	}
	return nil
}
