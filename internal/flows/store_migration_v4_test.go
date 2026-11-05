package flows

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func libraryV3Fixture(t *testing.T, conflict bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"flow_v4parent", "flow_v4block"} {
		doc := sampleFlow(id)
		if id == "flow_v4block" {
			doc.Kind = KindBlock
		}
		if _, err := s.CreateFlow(context.Background(), doc, "", storeNow); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v*-backup-*")); err != nil || len(paths) != 0 {
		t.Fatalf("fresh store created migration backups: %v, %v", paths, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`ALTER TABLE flow_runs DROP COLUMN durable_mission_id; ALTER TABLE flow_runs DROP COLUMN durable; DROP TABLE flow_version_notes; DROP TABLE flow_library_metadata;
		DROP TABLE flow_library_folders; UPDATE flows_meta SET value='3' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if conflict {
		if _, err := db.Exec(`CREATE TABLE flow_version_notes(unexpected TEXT)`); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func libraryMigrationData(t *testing.T, db *sql.DB) map[string][][]any {
	t.Helper()
	result := make(map[string][][]any)
	for _, table := range []string{"flows", "flow_versions", "flow_runs", "flow_run_steps", "flow_timers", "flow_test_data", "flow_test_scenarios", "flow_checkpoints", "flow_block_version_pins", "flow_block_run_pins"} {
		rows, err := db.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			targets := make([]any, len(columns))
			for i := range values {
				targets[i] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			// The v6 marker is an additive field absent in older backups.
			if table == "flow_runs" && len(columns) > 0 && columns[len(columns)-1] == "durable_mission_id" {
				values = values[:len(values)-2]
			}
			result[table] = append(result[table], values)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity = %q, %v", integrity, err)
	}
	return result
}

func TestFlowsV4MigrationPreservesWALPinsAndRestorableBackup(t *testing.T) {
	path := libraryV3Fixture(t, false)
	writer, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	if _, err := writer.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; PRAGMA wal_checkpoint(TRUNCATE);
		UPDATE flows SET name='Latest WAL name',live_json=draft_json,live_revision=1,published_draft_revision=1;
		INSERT INTO flow_versions(flow_id,revision,json,published_at) SELECT id,1,draft_json,created_at FROM flows;
		INSERT INTO flow_runs(id,flow_id,mode,status,started_at) VALUES('run_v4','flow_v4parent','test','success','2026-11-05T10:00:00Z');
		INSERT INTO flow_block_version_pins VALUES('flow_v4parent',1,'flow_v4block',1,zeroblob(32));
		INSERT INTO flow_block_run_pins VALUES('run_v4','flow_v4block',1,zeroblob(32));
		INSERT INTO flow_test_data VALUES('flow_v4parent','sample','trigger','{"value":false}','2026-11-05T10:00:00Z');
		INSERT INTO flow_test_scenarios(flow_id,id,ciphertext,updated_at) VALUES('flow_v4parent','scenario_v4',X'000180FF2A','2026-11-05T10:00:00Z');
		INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at)
		VALUES('run_v4',7,'waiting','2026-11-05T11:00:00Z','2026-11-06T10:00:00Z',X'FF8001002A','2026-11-05T10:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() <= 32 {
		t.Fatalf("fixture has no committed WAL: %v, %v", info, err)
	}
	want := libraryMigrationData(t, writer)
	assertData := func(db *sql.DB, version string) {
		t.Helper()
		if got := libraryMigrationData(t, db); !reflect.DeepEqual(got, want) {
			t.Fatal("migration changed existing executable, pin, run or encrypted test data")
		}
		var actual string
		if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&actual); err != nil || actual != version {
			t.Fatalf("schema = %q, want %q, %v", actual, version, err)
		}
	}
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assertData(s.db, "6")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v3-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	assertData(backup, "3")
	var newTables int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('flow_library_folders','flow_library_metadata','flow_version_notes','idx_flow_library_folder')`).Scan(&newTables); err != nil || newTables != 0 {
		t.Fatalf("backup contains migration writes: %d, %v", newTables, err)
	}
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	assertData(reopened.db, "6")
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if again, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v3-backup-*", "flows.db")); err != nil || len(again) != 1 {
		t.Fatalf("reopen repeated the migration: %v, %v", again, err)
	}
	data, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	restorePath := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(restorePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenStore(restorePath, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	assertData(restored.db, "6")
}

func TestFlowsV4MigrationFailureRollsBackBeforeVersionAdvance(t *testing.T) {
	path := libraryV3Fixture(t, true)
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
	if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "3" {
		t.Fatalf("failed migration advanced schema: %q, %v", version, err)
	}
	var partial, existing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('flow_library_folders','flow_library_metadata','idx_flow_library_folder')`).Scan(&partial); err != nil || partial != 0 {
		t.Fatalf("failed migration left writes: %d, %v", partial, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM flows`).Scan(&existing); err != nil || existing != 2 {
		t.Fatalf("existing flows changed: %d, %v", existing, err)
	}
	if paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v3-backup-*", "flows.db")); err != nil || len(paths) != 1 {
		t.Fatalf("failed migration backup = %v, %v", paths, err)
	}
}

func TestFlowsV4RefusesInvalidLibrarySchemaBeforeAnyWrites(t *testing.T) {
	for name, corrupt := range map[string]string{
		"missing table":  `DROP TABLE flow_version_notes`,
		"missing column": `ALTER TABLE flow_version_notes RENAME COLUMN note_revision TO unexpected_revision`,
		"missing index":  `DROP INDEX idx_flow_library_folder`,
		"missing ownership constraints": `DROP TABLE flow_library_metadata;
			CREATE TABLE flow_library_metadata(flow_id TEXT PRIMARY KEY,folder_id TEXT,tags_json TEXT,revision INTEGER);
			CREATE INDEX idx_flow_library_folder ON flow_library_metadata(folder_id,flow_id)`,
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
			if _, err := db.Exec(corrupt + `; DROP INDEX idx_flows_mission`); err != nil {
				t.Fatal(err)
			}
			if reopened, err := OpenStore(path, discardLogger()); err == nil {
				reopened.Close()
				t.Fatal("invalid library schema accepted")
			}
			var version string
			var recreated int
			if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
				t.Fatalf("schema changed: %q, %v", version, err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='idx_flows_mission'`).Scan(&recreated); err != nil || recreated != 0 {
				t.Fatalf("validation ran after baseline writes: %d, %v", recreated, err)
			}
			if paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v*-backup-*")); err != nil || len(paths) != 0 {
				t.Fatalf("invalid schema produced backups: %v, %v", paths, err)
			}
		})
	}
}
