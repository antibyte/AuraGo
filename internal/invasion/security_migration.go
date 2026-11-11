package invasion

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"
)

var eggSecurityColumns = []struct{ table, name, definition string }{
	{"eggs", "allow_filesystem_write", "INTEGER NOT NULL DEFAULT 0"},
	{"eggs", "allow_network_requests", "INTEGER NOT NULL DEFAULT 0"},
	{"eggs", "allow_unsandboxed_shell", "INTEGER NOT NULL DEFAULT 0"},
	{"safe_config_revisions", "permissions_json", "TEXT NOT NULL DEFAULT ''"},
}

func missingEggSecurityColumns(db *sql.DB) ([]string, error) {
	var statements []string
	for _, column := range eggSecurityColumns {
		var exists int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, column.table, column.name).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			statements = append(statements, "ALTER TABLE "+column.table+" ADD COLUMN "+column.name+" "+column.definition)
		}
	}
	return statements, nil
}

func prepareEggSecurityMigration(db *sql.DB, path string, existing bool) error {
	if !existing {
		return nil
	}
	backup, err := backupBeforeExportNestSecretMigration(db, path)
	if err != nil {
		return fmt.Errorf("backup before Invasion migration: %w", err)
	}
	if backup != "" {
		return nil
	}
	missing, err := missingEggSecurityColumns(db)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	_, err = backupInvasionDatabase(db, path+".pre-egg-security.bak")
	return err
}

func migrateEggSecurity(db *sql.DB) error {
	statements, err := missingEggSecurityColumns(db)
	if err != nil || len(statements) == 0 {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate Egg security: %w", err)
		}
	}
	return tx.Commit()
}

func backupInvasionDatabase(db *sql.DB, target string) (string, error) {
	if _, err := os.Stat(target); err == nil {
		target += "." + time.Now().UTC().Format("20060102T150405.000000000Z")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("create private Invasion backup: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if _, err := db.Exec(`VACUUM INTO ?`, target); err != nil {
		return "", fmt.Errorf("backup Invasion database: %w", err)
	}
	return target, nil
}
