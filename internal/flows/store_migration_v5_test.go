package flows

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func groupsV4Fixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"flow_v5parent", "flow_v5block"} {
		doc := sampleFlow(id)
		if id == "flow_v5block" {
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
		t.Fatalf("fresh store created backups: %v, %v", paths, err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;
		UPDATE flows_meta SET value='4' WHERE key='schema_version'; PRAGMA wal_checkpoint(TRUNCATE);
		UPDATE flows SET name='Only in WAL',live_json=draft_json,live_revision=1,published_draft_revision=1;
		INSERT INTO flow_versions(flow_id,revision,json,published_at) SELECT id,1,draft_json,created_at FROM flows;
		INSERT INTO flow_runs(id,flow_id,mode,status,started_at) VALUES('run_v5','flow_v5parent','test','success','2026-11-05T10:00:00Z');
		INSERT INTO flow_block_version_pins VALUES('flow_v5parent',1,'flow_v5block',1,zeroblob(32));
		INSERT INTO flow_block_run_pins VALUES('run_v5','flow_v5block',1,zeroblob(32));
		INSERT INTO flow_test_data VALUES('flow_v5parent','sample','trigger','{"value":false}','2026-11-05T10:00:00Z');
		INSERT INTO flow_test_scenarios VALUES('flow_v5parent','scenario_v5',X'000180FF2A','2026-11-05T10:00:00Z');
		INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at)
		VALUES('run_v5',7,'waiting','2026-11-05T11:00:00Z','2026-11-06T10:00:00Z',X'FF8001002A','2026-11-05T10:00:00Z');
		INSERT INTO flow_library_folders VALUES('folder_v5','Existing folder',3,'2026-11-05T10:00:00Z');
		INSERT INTO flow_library_metadata VALUES('flow_v5parent','folder_v5','["Existing tag"]',4);
		INSERT INTO flow_version_notes VALUES('flow_v5parent',1,'Existing publication note',2)`); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.QueryRow(`SELECT draft_json FROM flows WHERE id='flow_v5parent'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(raw))
	key := hex.EncodeToString(hash[:])
	if _, err := db.Exec(`INSERT INTO flow_test_docs VALUES('flow_v5parent',?,?); UPDATE flow_runs SET doc_hash=? WHERE id='run_v5'`, key, raw, key); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() <= 32 {
		t.Fatalf("fixture has no committed WAL: %v, %v", info, err)
	}
	return path, db
}

func groupsMigrationData(t *testing.T, db *sql.DB) map[string][][]any {
	t.Helper()
	data := libraryMigrationData(t, db)
	for _, table := range []string{"flow_test_docs", "flow_library_folders", "flow_library_metadata", "flow_version_notes"} {
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
			data[table] = append(data[table], values)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	return data
}

func TestFlowsV5MigrationPreservesWALAndRestorableBackup(t *testing.T) {
	path, writer := groupsV4Fixture(t)
	want := groupsMigrationData(t, writer)
	assertData := func(db *sql.DB, version string) {
		t.Helper()
		if got := groupsMigrationData(t, db); !reflect.DeepEqual(got, want) {
			t.Fatal("migration changed existing documents, hashes, pins, encrypted data or library metadata")
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
	assertData(s.db, "5")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v4-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	assertData(backup, "4")
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	assertData(reopened.db, "5")
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if paths, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v4-backup-*", "flows.db")); err != nil || len(paths) != 1 {
		t.Fatalf("reopen repeated the backup: %v, %v", paths, err)
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
	assertData(restored.db, "5")
}

func TestFlowsV5MigrationFailureKeepsMarkerAndBackupBeforeBaselineWrites(t *testing.T) {
	path, db := groupsV4Fixture(t)
	want := groupsMigrationData(t, db)
	if _, err := db.Exec(`DROP INDEX idx_flows_mission;
		CREATE TRIGGER reject_groups_marker BEFORE UPDATE OF value ON flows_meta
		WHEN OLD.key='schema_version' AND OLD.value='4' AND NEW.value='5'
		BEGIN SELECT RAISE(ABORT,'injected marker failure'); END`); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStore(path, discardLogger()); err == nil {
		s.Close()
		t.Fatal("marker write failure accepted")
	}
	var version string
	var recreated int
	if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "4" {
		t.Fatalf("failed migration changed marker: %q, %v", version, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='idx_flows_mission'`).Scan(&recreated); err != nil || recreated != 0 {
		t.Fatalf("baseline wrote before compatibility migration: %d, %v", recreated, err)
	}
	if got := groupsMigrationData(t, db); !reflect.DeepEqual(got, want) {
		t.Fatal("failed migration changed stored data")
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v4-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("failed migration backup = %v, %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if got := groupsMigrationData(t, backup); !reflect.DeepEqual(got, want) {
		t.Fatal("failed migration backup lost data")
	}
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "4" {
		t.Fatalf("backup marker = %q, %v", version, err)
	}
}
