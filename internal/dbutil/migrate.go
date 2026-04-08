package dbutil

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// MigrateAddColumn adds a column to a table if it doesn't already exist.
// It checks using PRAGMA table_info and only executes ALTER TABLE if needed.
// Returns nil if the column already exists or was added successfully.
func MigrateAddColumn(db *sql.DB, table, column, definition string, logger *slog.Logger) error {
	// Check if column already exists
	hasCol, err := hasColumn(db, table, column)
	if err != nil {
		return fmt.Errorf("check column existence for %s.%s: %w", table, column, err)
	}
	if hasCol {
		return nil
	}

	// Log migration intent
	logger.Info("Migrating SQLite: adding column", "table", table, "column", column)

	// Execute ALTER TABLE
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}

	return nil
}

// MigrateAddColumnChecked adds a column with a verification check after adding.
// This variant is tolerant of duplicate-column errors that may occur due to
// schema drift, verifying that the column exists after attempting to add it.
func MigrateAddColumnChecked(db *sql.DB, table, column, definition string, logger *slog.Logger) error {
	// Check if column already exists
	hasCol, err := hasColumn(db, table, column)
	if err != nil {
		return fmt.Errorf("check column existence for %s.%s: %w", table, column, err)
	}
	if hasCol {
		return nil
	}

	// Log migration intent
	logger.Info("Migrating SQLite: adding column (checked)", "table", table, "column", column)

	// Execute ALTER TABLE
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if _, err := db.Exec(stmt); err != nil {
		// Check if it's a duplicate column error - if so, re-verify
		if isDuplicateColumnError(err) {
			// Re-check if column now exists
			hasColAfter, checkErr := hasColumn(db, table, column)
			if checkErr != nil {
				return fmt.Errorf("re-check column existence after duplicate error: %w", checkErr)
			}
			if hasColAfter {
				// Column exists now, treat as success
				return nil
			}
		}
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}

	// Verify column was added
	hasColAfter, err := hasColumn(db, table, column)
	if err != nil {
		return fmt.Errorf("verify column addition for %s.%s: %w", table, column, err)
	}
	if !hasColAfter {
		return fmt.Errorf("column %s.%s was not added successfully", table, column)
	}

	return nil
}

// hasColumn checks if a column exists in a table using PRAGMA table_info.
func hasColumn(db *sql.DB, table, column string) (bool, error) {
	query := "SELECT count(*) > 0 FROM pragma_table_info(?) WHERE name=?"
	var exists bool
	err := db.QueryRow(query, table, column).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("query column info: %w", err)
	}
	return exists, nil
}

// isDuplicateColumnError checks if the error is a "duplicate column name" error.
func isDuplicateColumnError(err error) bool {
	errStr := err.Error()
	return len(errStr) >= 19 && errStr[len(errStr)-19:] == "duplicate column name"
}
