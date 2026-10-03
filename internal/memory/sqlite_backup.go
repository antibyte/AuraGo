package memory

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
)

// backupMainDatabase writes a consistent, owner-only copy of the main SQLite database
// next to it (VACUUM INTO includes WAL content) before a destructive migration. It
// returns the backup path; in-memory stores have no file and return "" without error.
func backupMainDatabase(db *sql.DB, label string) (string, error) {
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		return "", fmt.Errorf("locate database for backup: %w", err)
	}
	path := ""
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			rows.Close()
			return "", fmt.Errorf("read database list: %w", err)
		}
		if name == "main" {
			path = file
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", fmt.Errorf("read database list: %w", err)
	}
	if path == "" {
		return "", nil
	}
	backup := path + "." + label + "-" + rand.Text() + ".bak"
	if _, err := db.Exec(`VACUUM main INTO ?`, backup); err != nil {
		return "", fmt.Errorf("back up database: %w", err)
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		return "", fmt.Errorf("protect database backup: %w", err)
	}
	return backup, nil
}
