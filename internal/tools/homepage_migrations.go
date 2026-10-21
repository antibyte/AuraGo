package tools

import (
	"aurago/internal/dbutil"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
)

const homepageSchemaVersion = 1

// Back up the old schema before any DDL, including CREATE IF NOT EXISTS.
func prepareHomepageRegistryMigration(db *sql.DB, path string) error {
	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		return err
	}
	if version > homepageSchemaVersion {
		return fmt.Errorf("homepage schema %d is newer than supported schema %d", version, homepageSchemaVersion)
	}
	if version == homepageSchemaVersion {
		return nil
	}
	var existing int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='homepage_projects'").Scan(&existing); err != nil {
		return err
	}
	if existing == 0 || path == ":memory:" {
		return nil
	}
	backup := fmt.Sprintf("%s.homepage-v%d-%s.bak", path, homepageSchemaVersion, rand.Text())
	if _, err := db.Exec("VACUUM main INTO ?", backup); err != nil {
		return fmt.Errorf("back up homepage registry: %w", err)
	}
	if err := os.Chmod(backup, 0600); err != nil {
		return fmt.Errorf("protect homepage migration backup: %w", err)
	}
	return nil
}

func migrateHomepageRegistry(db *sql.DB) error {
	version, err := dbutil.GetUserVersion(db)
	if err != nil || version == homepageSchemaVersion {
		return err
	}
	// Init owns a single connection and has not published the database yet.
	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer db.Exec("PRAGMA foreign_keys=ON")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		"ALTER TABLE homepage_projects ADD COLUMN workspace_root TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE homepage_projects ADD COLUMN origin TEXT NOT NULL DEFAULT 'unknown'",
		"ALTER TABLE homepage_projects ADD COLUMN creation_run_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE homepage_revision_files ADD COLUMN file_mode INTEGER NOT NULL DEFAULT 0",
		`CREATE TABLE homepage_deploy_targets_v1 (
            id INTEGER PRIMARY KEY AUTOINCREMENT, project_id INTEGER NOT NULL,
            provider TEXT NOT NULL, provider_target_id TEXT DEFAULT '',
            url TEXT DEFAULT '', remote_path TEXT DEFAULT '', metadata_json TEXT DEFAULT '',
            last_seen_at DATETIME DEFAULT NULL, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            target_key TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'unknown',
            UNIQUE(project_id, provider, target_key),
            FOREIGN KEY(project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE)`,
		`INSERT INTO homepage_deploy_targets_v1
            (id,project_id,provider,provider_target_id,url,remote_path,metadata_json,last_seen_at,updated_at,target_key,status)
            SELECT id,project_id,provider,provider_target_id,url,remote_path,metadata_json,last_seen_at,updated_at,
            'legacy:' || id,'unknown' FROM homepage_deploy_targets`,
		"DROP TABLE homepage_deploy_targets",
		"ALTER TABLE homepage_deploy_targets_v1 RENAME TO homepage_deploy_targets",
		"CREATE INDEX idx_hdt_project_id ON homepage_deploy_targets(project_id)",
		"CREATE INDEX idx_hdt_provider ON homepage_deploy_targets(provider)",
		"CREATE INDEX idx_hse_correlation ON homepage_site_events(correlation_id,id)",
		`UPDATE homepage_revisions SET restorable=0 WHERE id IN
            (SELECT revision_id FROM homepage_revision_files WHERE change_type IN ('added','modified')
             AND length(CAST(content_after AS BLOB))!=size_after)`,
		"PRAGMA user_version=1",
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate homepage registry: %w", err)
		}
	}
	rows, err := tx.Query("PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	invalid := rows.Next()
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if invalid {
		return fmt.Errorf("homepage migration failed foreign-key verification")
	}
	return tx.Commit()
}
