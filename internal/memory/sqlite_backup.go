package memory

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/fileutil"
)

// backupMainDatabase writes a consistent, owner-only copy of the main SQLite database
// next to it (VACUUM INTO includes WAL content) before a destructive migration. It
// returns the backup path; in-memory stores have no file and return "" without error.
func backupMainDatabase(db *sql.DB, label string) (string, error) {
	path, err := mainDatabasePath(db)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	backup := path + "." + label + "-" + rand.Text() + ".bak"
	if _, err := db.Exec(`VACUUM main INTO ?`, backup); err != nil {
		return "", fmt.Errorf("back up database: %w", err)
	}
	if err := fileutil.RestrictToOwner(backup); err != nil {
		return "", fmt.Errorf("protect database backup: %w", err)
	}
	return backup, nil
}

func mainDatabasePath(db *sql.DB) (string, error) {
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
	return path, nil
}

func protectMemoryDatabase(db *sql.DB) error {
	path, err := mainDatabasePath(db)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	if err := rejectSnapshotSymlinks(path); err != nil {
		return err
	}
	if err := fileutil.RestrictToOwner(filepath.Dir(path)); err != nil {
		return fmt.Errorf("protect memory directory: %w", err)
	}
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		if _, err := os.Lstat(file); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if err := fileutil.RestrictToOwner(file); err != nil {
			return fmt.Errorf("protect memory database: %w", err)
		}
	}
	return nil
}
