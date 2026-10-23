package tools

import (
	"aurago/internal/dbutil"
	"aurago/internal/fileutil"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const homepageSchemaVersion = 2

// homepageRegistryV1Statements are the schema v1 DDL statements. They run for
// every registry that is still on schema v0.
var homepageRegistryV1Statements = []string{
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
}

// homepageRegistryV2Statements enforce one registry row per project directory.
// The index is partial: rows without a project directory (the provider ledger
// rows) are not a project and must not collide.
var homepageRegistryV2Statements = []string{
	"CREATE UNIQUE INDEX IF NOT EXISTS idx_hp_project_dir_unique ON homepage_projects(project_dir) WHERE project_dir <> ''",
}

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
	if existing == 0 {
		return nil
	}
	var databaseFile string
	if err := db.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&databaseFile); err != nil {
		return err
	}
	if databaseFile == "" {
		return nil
	}
	backup := fmt.Sprintf("%s.homepage-v%d-%s.bak", databaseFile, homepageSchemaVersion, rand.Text())
	reserved, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("reserve protected homepage migration backup: %w", err)
	}
	if err := reserved.Close(); err != nil {
		return err
	}
	if err := fileutil.RestrictToOwner(backup); err != nil {
		return fmt.Errorf("protect homepage migration backup before copying: %w", err)
	}
	if _, err := db.Exec("VACUUM main INTO ?", backup); err != nil {
		return fmt.Errorf("back up homepage registry: %w", err)
	}
	if err := fileutil.RestrictToOwner(backup); err != nil {
		return fmt.Errorf("protect homepage migration backup: %w", err)
	}
	return nil
}

func migrateHomepageRegistry(db *sql.DB) error {
	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		return err
	}
	if version >= homepageSchemaVersion {
		return nil
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
	var statements []string
	if version < 1 {
		statements = append(statements, homepageRegistryV1Statements...)
	}
	if version < 2 {
		// The duplicate merge runs before the unique index is created.
		if err := mergeDuplicateHomepageProjects(tx); err != nil {
			return fmt.Errorf("migrate homepage registry: %w", err)
		}
		statements = append(statements, homepageRegistryV2Statements...)
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate homepage registry: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version=%d", homepageSchemaVersion)); err != nil {
		return fmt.Errorf("migrate homepage registry: %w", err)
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

// mergeDuplicateHomepageProjects collapses registry rows that share a project
// directory into a single row. The row with the newest updated_at survives,
// the child rows of the removed rows are re-pointed at the survivor, and every
// merge is recorded in the Homepage ledger.
func mergeDuplicateHomepageProjects(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT project_dir FROM homepage_projects
		WHERE project_dir <> '' GROUP BY project_dir HAVING COUNT(*) > 1`)
	if err != nil {
		return err
	}
	var projectDirs []string
	for rows.Next() {
		var projectDir string
		if err := rows.Scan(&projectDir); err != nil {
			rows.Close()
			return err
		}
		projectDirs = append(projectDirs, projectDir)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	for _, projectDir := range projectDirs {
		if err := mergeHomepageProjectDir(tx, projectDir); err != nil {
			return err
		}
	}
	return nil
}

// mergeHomepageProjectDir merges every registry row of one project directory
// into the row with the newest updated_at.
func mergeHomepageProjectDir(tx *sql.Tx, projectDir string) error {
	var keeper int64
	if err := tx.QueryRow(`SELECT id FROM homepage_projects WHERE project_dir = ?
		ORDER BY updated_at DESC, id DESC LIMIT 1`, projectDir).Scan(&keeper); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT id FROM homepage_projects WHERE project_dir = ? AND id <> ?`, projectDir, keeper)
	if err != nil {
		return err
	}
	var losers []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		losers = append(losers, id)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	if len(losers) == 0 {
		return nil
	}
	for _, loser := range losers {
		if err := repointHomepageProjectChildren(tx, keeper, loser); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(map[string]interface{}{
		"project_dir": projectDir,
		"kept_id":     keeper,
		"merged_ids":  losers,
	})
	if err != nil {
		return err
	}
	summary := fmt.Sprintf("Merged %d duplicate registry row(s) for project directory %q into row %d", len(losers), projectDir, keeper)
	if _, err := tx.Exec(`INSERT INTO homepage_site_events(project_id,actor,source,event_type,summary,payload_json)
		VALUES(?,'system','homepage_registry','project_dir_merged',?,?)`, keeper, summary, string(payload)); err != nil {
		return err
	}
	for _, loser := range losers {
		if _, err := tx.Exec("DELETE FROM homepage_projects WHERE id = ?", loser); err != nil {
			return err
		}
	}
	return nil
}

// repointHomepageProjectChildren re-points the child rows of a removed
// registry row at the surviving row. Rows with a unique constraint keep the
// survivor's value.
func repointHomepageProjectChildren(tx *sql.Tx, keeper, loser int64) error {
	for _, table := range []string{
		"homepage_revisions",
		"homepage_history",
		"homepage_site_events",
		"homepage_deployments",
		"homepage_remote_observations",
	} {
		if _, err := tx.Exec(fmt.Sprintf("UPDATE %s SET project_id = ? WHERE project_id = ?", table), keeper, loser); err != nil {
			return fmt.Errorf("re-point %s: %w", table, err)
		}
	}

	// Deployment targets are unique per (project_id, provider).
	rows, err := tx.Query("SELECT id, provider FROM homepage_deploy_targets WHERE project_id = ?", loser)
	if err != nil {
		return err
	}
	type deployTarget struct {
		id       int64
		provider string
	}
	var targets []deployTarget
	for rows.Next() {
		var target deployTarget
		if err := rows.Scan(&target.id, &target.provider); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, target)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return rowErr
	}
	for _, target := range targets {
		var existing int64
		err := tx.QueryRow("SELECT id FROM homepage_deploy_targets WHERE project_id = ? AND provider = ?", keeper, target.provider).Scan(&existing)
		switch {
		case err == nil:
			// The survivor already has a target for this provider: re-point the
			// references and drop the duplicate target.
			if _, err := tx.Exec("UPDATE homepage_deployments SET target_id = ? WHERE target_id = ?", existing, target.id); err != nil {
				return err
			}
			if _, err := tx.Exec("UPDATE homepage_remote_observations SET target_id = ? WHERE target_id = ?", existing, target.id); err != nil {
				return err
			}
			if _, err := tx.Exec("DELETE FROM homepage_deploy_targets WHERE id = ?", target.id); err != nil {
				return err
			}
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec("UPDATE homepage_deploy_targets SET project_id = ? WHERE id = ?", keeper, target.id); err != nil {
				return err
			}
		default:
			return err
		}
	}

	// File state is unique per (project_id, rel_path): the survivor's row wins.
	if _, err := tx.Exec(`DELETE FROM homepage_site_file_state
		WHERE project_id = ? AND rel_path IN (SELECT rel_path FROM homepage_site_file_state WHERE project_id = ?)`,
		loser, keeper); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE homepage_site_file_state SET project_id = ? WHERE project_id = ?", keeper, loser); err != nil {
		return err
	}

	// Project state is keyed by project_id: the survivor's state wins.
	if _, err := tx.Exec(`DELETE FROM homepage_project_state
		WHERE project_id = ? AND EXISTS (SELECT 1 FROM homepage_project_state WHERE project_id = ?)`,
		loser, keeper); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE homepage_project_state SET project_id = ? WHERE project_id = ?", keeper, loser); err != nil {
		return err
	}
	return nil
}
