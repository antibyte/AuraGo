package flows

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func childHistoryV6Fixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFlow(context.Background(), sampleFlow("flow_v7test"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	block := sampleFlow("block_v7test")
	block.Kind = KindBlock
	if _, err := s.CreateFlow(context.Background(), block, "", storeNow); err != nil {
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
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0;
		DROP TABLE flow_child_traces;
		ALTER TABLE flow_runs DROP COLUMN child_history_state;
		ALTER TABLE flow_runs DROP COLUMN child_history_bytes;
		ALTER TABLE flow_runs DROP COLUMN child_history_metadata;
		UPDATE flows_meta SET value='6' WHERE key='schema_version'; PRAGMA wal_checkpoint(TRUNCATE);
		INSERT INTO flow_runs(id,flow_id,revision,mode,status,started_at,durable,durable_mission_id)
		VALUES('run_v7old','flow_v7test',1,'live','waiting','2026-11-05T10:00:00Z',1,'mission_old');
		INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at)
		VALUES('run_v7old',1,'waiting','2026-11-05T11:00:00Z','2026-11-06T10:00:00Z',X'FF8001','2026-11-05T10:00:00Z');
		UPDATE flows SET live_json=draft_json,live_revision=1 WHERE id='block_v7test';
		INSERT INTO flow_versions(flow_id,revision,json,published_at) SELECT id,1,draft_json,created_at FROM flows WHERE id='block_v7test';
		INSERT INTO flow_block_run_pins VALUES('run_v7old','block_v7test',1,zeroblob(32));
		INSERT INTO flow_test_scenarios VALUES('flow_v7test','scenario_v7',X'000180FF2A','2026-11-05T10:00:00Z');
		INSERT INTO flow_test_docs(flow_id,hash,json) SELECT id,'v7_original_hash',draft_json FROM flows WHERE id='flow_v7test';
		UPDATE flow_runs SET doc_hash='v7_original_hash' WHERE id='run_v7old'`)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() <= 32 {
		t.Fatalf("WAL: %v %v", info, err)
	}
	return path, db
}

func assertChildHistoryV6Data(t *testing.T, db *sql.DB) {
	t.Helper()
	var pin, scenario string
	var matching int
	if err := db.QueryRow(`SELECT hex(block_hash) FROM flow_block_run_pins WHERE run_id='run_v7old' AND block_flow_id='block_v7test' AND block_revision=1`).Scan(&pin); err != nil || pin != strings.Repeat("00", 32) {
		t.Fatalf("run pin %q: %v", pin, err)
	}
	if err := db.QueryRow(`SELECT hex(ciphertext) FROM flow_test_scenarios WHERE flow_id='flow_v7test' AND id='scenario_v7'`).Scan(&scenario); err != nil || scenario != "000180FF2A" {
		t.Fatalf("scenario %q: %v", scenario, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM flow_test_docs d JOIN flow_runs r ON r.flow_id=d.flow_id AND r.doc_hash=d.hash JOIN flows f ON f.id=d.flow_id WHERE r.id='run_v7old' AND d.hash='v7_original_hash' AND d.json=f.draft_json`).Scan(&matching); err != nil || matching != 1 {
		t.Fatalf("run doc/hash: %d %v", matching, err)
	}
}

func TestFlowsV7MigrationPreservesWALAndBackup(t *testing.T) {
	path, _ := childHistoryV6Fixture(t)
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	var history, owner string
	var ciphertext []byte
	if err := s.db.QueryRow(`SELECT r.child_history_state,r.durable_mission_id,c.ciphertext FROM flow_runs r JOIN flow_checkpoints c ON c.run_id=r.id WHERE r.id='run_v7old'`).Scan(&history, &owner, &ciphertext); err != nil || history != "legacy_unknown" || owner != "mission_old" || string(ciphertext) != string([]byte{0xff, 0x80, 1}) {
		t.Fatalf("migration data %q %q %x: %v", history, owner, ciphertext, err)
	}
	assertChildHistoryV6Data(t, s.db)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v6-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup: %v %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var version string
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
		t.Fatalf("backup version %q: %v", version, err)
	}
	if err := backup.QueryRow(`SELECT ciphertext FROM flow_checkpoints WHERE run_id='run_v7old'`).Scan(&ciphertext); err != nil || string(ciphertext) != string([]byte{0xff, 0x80, 1}) {
		t.Fatalf("backup WAL: %x %v", ciphertext, err)
	}
	assertChildHistoryV6Data(t, backup)
	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	backups, err = filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v6-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("reopen backups: %v %v", backups, err)
	}
}

func TestFlowsV7MigrationRollbackBeforeBaselineWrites(t *testing.T) {
	path, db := childHistoryV6Fixture(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_v7 BEFORE UPDATE OF value ON flows_meta WHEN OLD.value='6' AND NEW.value='7' BEGIN SELECT RAISE(ABORT,'injected'); END; DROP INDEX idx_flows_mission`); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStore(path, discardLogger()); err == nil {
		_ = s.Close()
		t.Fatal("failed migration accepted")
	}
	var version string
	var count int
	if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "6" {
		t.Fatalf("version %q: %v", version, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name IN ('flow_child_traces','idx_flows_mission')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback tables/index: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('flow_runs') WHERE name LIKE 'child_history_%'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback columns: %d %v", count, err)
	}
}
