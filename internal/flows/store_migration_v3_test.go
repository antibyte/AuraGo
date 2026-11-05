package flows

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func blockV2Fixture(t *testing.T, conflict bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_v3fixture"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`ALTER TABLE flow_runs DROP COLUMN durable_mission_id; ALTER TABLE flow_runs DROP COLUMN durable; DROP TABLE flow_version_notes; DROP TABLE flow_library_metadata; DROP TABLE flow_library_folders; DROP TABLE flow_block_run_pins; DROP TABLE flow_block_version_pins; UPDATE flows_meta SET value='2' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if conflict {
		if _, err := db.Exec(`CREATE TABLE flow_block_run_pins(unexpected TEXT)`); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestFlowsV3MigrationBacksUpAndPreservesData(t *testing.T) {
	path := blockV2Fixture(t, false)
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	// Keep the only copy of these committed updates in WAL during migration.
	if _, err := writer.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; PRAGMA wal_checkpoint(TRUNCATE);
		UPDATE flows SET name='Latest WAL value' WHERE id='flow_v3fixture';
		INSERT INTO flow_runs(id,flow_id,mode,status,started_at) VALUES('run_v3fixture','flow_v3fixture','test','success','2026-11-05T10:00:00Z');
		INSERT INTO flow_test_scenarios(flow_id,id,ciphertext,updated_at) VALUES('flow_v3fixture','scenario_v3',X'000180FF2A','2026-11-05T10:00:00Z');
		INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at)
		VALUES('run_v3fixture',7,'waiting','2026-11-05T11:00:00Z','2026-11-06T10:00:00Z',X'FF8001002A','2026-11-05T10:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() <= 32 {
		t.Fatalf("fixture has no populated WAL: %v, %v", info, err)
	}
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assertBlockMigrationData(t, s.db)
	var version string
	if err := s.db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
		t.Fatalf("version = %q, %v", version, err)
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v2-backup-*", "flows.db"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("backups: %v, %v", paths, err)
	}
	backup, err := sql.Open("sqlite", paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	assertBlockMigrationData(t, backup)
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "2" {
		t.Fatalf("backup version = %q, %v", version, err)
	}
	var pins int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('flow_block_version_pins','flow_block_run_pins')`).Scan(&pins); err != nil || pins != 0 {
		t.Fatalf("backup contains migration writes: pins=%d, %v", pins, err)
	}
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	assertBlockMigrationData(t, reopened.db)
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if again, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v2-backup-*", "flows.db")); err != nil || len(again) != 1 {
		t.Fatalf("reopen repeated the migration backup: %v, %v", again, err)
	}
	// The saved v2 file must itself be sufficient for a clean staged restore.
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(restoredPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenStore(restoredPath, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	assertBlockMigrationData(t, restored.db)
	if err := restored.db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
		t.Fatalf("restored schema = %q, %v", version, err)
	}
}

func assertBlockMigrationData(t *testing.T, db *sql.DB) {
	t.Helper()
	var name, status, state, integrity string
	var generation int
	var scenario, checkpoint []byte
	if err := db.QueryRow(`SELECT name FROM flows WHERE id='flow_v3fixture'`).Scan(&name); err != nil || name != "Latest WAL value" {
		t.Fatalf("latest flow metadata = %q, %v", name, err)
	}
	if err := db.QueryRow(`SELECT status FROM flow_runs WHERE id='run_v3fixture'`).Scan(&status); err != nil || status != "success" {
		t.Fatalf("retained run = %q, %v", status, err)
	}
	if err := db.QueryRow(`SELECT ciphertext FROM flow_test_scenarios WHERE flow_id='flow_v3fixture' AND id='scenario_v3'`).Scan(&scenario); err != nil || !bytes.Equal(scenario, []byte{0, 1, 128, 255, 42}) {
		t.Fatalf("scenario bytes changed: %v", err)
	}
	if err := db.QueryRow(`SELECT generation,state,ciphertext FROM flow_checkpoints WHERE run_id='run_v3fixture'`).Scan(&generation, &state, &checkpoint); err != nil || generation != 7 || state != "waiting" || !bytes.Equal(checkpoint, []byte{255, 128, 1, 0, 42}) {
		t.Fatalf("opaque checkpoint changed: generation=%d state=%q, %v", generation, state, err)
	}
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("database integrity = %q, %v", integrity, err)
	}
}

func TestFlowsV3MigrationFailureIsAtomic(t *testing.T) {
	path := blockV2Fixture(t, true)
	if s, err := OpenStore(path, discardLogger()); err == nil {
		s.Close()
		t.Fatal("conflicting migration succeeded")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version string
	var count int
	if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "2" {
		t.Fatalf("version = %q, %v", version, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='flow_block_version_pins'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial migration: %d, %v", count, err)
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v2-backup-*", "flows.db"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("backup: %v, %v", paths, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM flows WHERE id='flow_v3fixture'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed migration lost existing flow: %d, %v", count, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name IN ('idx_block_version_pins_child','idx_block_run_pins_child')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration left indexes: %d, %v", count, err)
	}
}

func TestFlowsV3RefusesIncompletePinSchemaBeforeWriting(t *testing.T) {
	for name, corrupt := range map[string]string{
		"missing table":  `DROP TABLE flow_block_run_pins`,
		"missing column": `ALTER TABLE flow_block_run_pins RENAME COLUMN block_hash TO unexpected_hash`,
		"missing index":  `DROP INDEX idx_block_run_pins_child`,
		"missing ownership constraints": `DROP TABLE flow_block_run_pins;
			CREATE TABLE flow_block_run_pins(run_id TEXT,block_flow_id TEXT,block_revision INTEGER,block_hash BLOB);
			CREATE INDEX idx_block_run_pins_child ON flow_block_run_pins(block_flow_id,block_revision)`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "flows.db")
			s, err := OpenStore(path, discardLogger())
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			// Missing an unrelated older index detects a late verification pass:
			// schemaStatements would recreate it if any migration ran first.
			if _, err := db.Exec(corrupt + `; DROP INDEX idx_flows_mission`); err != nil {
				t.Fatal(err)
			}
			if reopened, err := OpenStore(path, discardLogger()); err == nil {
				reopened.Close()
				t.Fatal("incomplete v3 schema was accepted")
			}
			var version string
			var recreated int
			if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
				t.Fatalf("schema marker changed: %q, %v", version, err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_flows_mission'`).Scan(&recreated); err != nil || recreated != 0 {
				t.Fatalf("verification ran after schema writes: %d, %v", recreated, err)
			}
			if backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v*-backup-*")); err != nil || len(backups) != 0 {
				t.Fatalf("corrupt v3 unexpectedly migrated: %v, %v", backups, err)
			}
		})
	}
}
