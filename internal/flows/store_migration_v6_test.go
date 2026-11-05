package flows

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func durableV5Fixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_v6test"), "mission_v6", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;
		DROP TABLE flow_durable_gates; DROP TABLE flow_child_traces; ALTER TABLE flow_runs DROP COLUMN child_history_state; ALTER TABLE flow_runs DROP COLUMN child_history_bytes; ALTER TABLE flow_runs DROP COLUMN child_history_metadata; ALTER TABLE flow_runs DROP COLUMN durable_mission_id; ALTER TABLE flow_runs DROP COLUMN durable; UPDATE flows_meta SET value='5' WHERE key='schema_version'; PRAGMA wal_checkpoint(TRUNCATE);
		INSERT INTO flow_runs(id,flow_id,revision,mode,status,started_at) VALUES('run_old','flow_v6test',1,'live','waiting','2026-11-05T10:00:00Z');
		INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at) VALUES('run_old',1,'waiting','2026-11-05T11:00:00Z','2026-11-06T10:00:00Z',X'FF8001','2026-11-05T10:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() <= 32 {
		t.Fatalf("missing WAL: %v %v", info, err)
	}
	return path, db
}

func TestFlowsV6MigrationBacksUpWALAndPreservesLegacyCheckpoints(t *testing.T) {
	path, old := durableV5Fixture(t)
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var marker int
	var owner string
	var raw []byte
	if err := s.db.QueryRow(`SELECT r.durable,r.durable_mission_id,c.ciphertext FROM flow_runs r JOIN flow_checkpoints c ON c.run_id=r.id WHERE r.id='run_old'`).Scan(&marker, &owner, &raw); err != nil || marker != 0 || owner != "" || string(raw) != string([]byte{0xff, 0x80, 1}) {
		t.Fatalf("legacy checkpoint changed: %d %x %v", marker, raw, err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v5-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup: %v %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var version string
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "5" {
		t.Fatalf("backup schema %q: %v", version, err)
	}
	if err := backup.QueryRow(`SELECT ciphertext FROM flow_checkpoints WHERE run_id='run_old'`).Scan(&raw); err != nil || string(raw) != string([]byte{0xff, 0x80, 1}) {
		t.Fatalf("backup WAL lost: %x %v", raw, err)
	}
	if _, err := s.db.Exec(`UPDATE flow_runs SET durable=2 WHERE id='run_old'`); err == nil {
		t.Fatal("invalid marker accepted")
	}
	_ = old
}

func TestFlowsV6MigrationRollsBackMarkerOnFailure(t *testing.T) {
	path, db := durableV5Fixture(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_v6 BEFORE UPDATE OF value ON flows_meta WHEN OLD.value='5' AND NEW.value='6' BEGIN SELECT RAISE(ABORT,'injected'); END; DROP INDEX idx_flows_mission`); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStore(path, discardLogger()); err == nil {
		s.Close()
		t.Fatal("migration failure accepted")
	}
	var version string
	var column, index int
	if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "5" {
		t.Fatalf("version: %q %v", version, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('flow_runs') WHERE name IN ('durable','durable_mission_id')`).Scan(&column); err != nil || column != 0 {
		t.Fatalf("column survived rollback: %d %v", column, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='idx_flows_mission'`).Scan(&index); err != nil || index != 0 {
		t.Fatalf("baseline wrote after failure: %d %v", index, err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v5-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup: %v %v", backups, err)
	}
}
