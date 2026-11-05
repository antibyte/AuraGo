package flows

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func v1MigrationFixture(t *testing.T, conflict bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_aaaaaaaaaa"), "mission_1", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`ALTER TABLE flow_runs DROP COLUMN durable_mission_id; ALTER TABLE flow_runs DROP COLUMN durable; DROP TABLE flow_version_notes; DROP TABLE flow_library_metadata; DROP TABLE flow_library_folders; DROP TABLE flow_block_run_pins; DROP TABLE flow_block_version_pins; DROP TABLE flow_checkpoints; DROP TABLE flow_test_scenarios; UPDATE flows_meta SET value='1' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if conflict {
		if _, err := raw.Exec(`CREATE TABLE flow_test_scenarios (unexpected TEXT)`); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestFlowsV2MigrationBacksUpExistingData(t *testing.T) {
	path := v1MigrationFixture(t, false)
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.GetFlow(context.Background(), "flow_aaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := s.db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
		t.Fatalf("migrated version = %q %v", version, err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v1-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var count int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM flows`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backup flows = %d %v", count, err)
	}
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "1" {
		t.Fatalf("backup version = %q %v", version, err)
	}
}

func TestFlowsV2MigrationFailureKeepsV1AndBackup(t *testing.T) {
	path := v1MigrationFixture(t, true)
	if s, err := OpenStore(path, discardLogger()); err == nil {
		_ = s.Close()
		t.Fatal("conflicting migration succeeded")
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var version string
	if err := raw.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "1" {
		t.Fatalf("version = %q %v", version, err)
	}
	var checkpoints int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='flow_checkpoints'`).Scan(&checkpoints); err != nil || checkpoints != 0 {
		t.Fatalf("partial migration: %d %v", checkpoints, err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v1-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("failed migration backup = %v %v", backups, err)
	}
}
