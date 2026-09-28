package gamemaker

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLite creates a consistent backup including committed WAL data before ALTER.
func migrateVoxelVariant(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(gm_projects)`)
	if err != nil {
		return fmt.Errorf("inspect voxel migration: %w", err)
	}
	exists, hasVariant := false, false
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		exists = true
		hasVariant = hasVariant || name == "variant"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !exists || hasVariant {
		return nil
	}
	var seq int
	var name, path string
	if err := db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		return fmt.Errorf("locate migration database: %w", err)
	}
	if path != "" {
		backup := path + ".before-voxel-" + time.Now().UTC().Format("20060102T150405.000000000") + ".bak"
		if _, err := db.Exec(`VACUUM INTO '` + strings.ReplaceAll(backup, "'", "''") + `'`); err != nil {
			return fmt.Errorf("backup before voxel migration: %w", err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE gm_projects ADD COLUMN variant TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("migrate voxel variant: %w", err)
	}
	return nil
}
