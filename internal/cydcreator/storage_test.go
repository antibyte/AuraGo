package cydcreator

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func openTestStore(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "cyd_creator.db")
	db, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	return cols
}

func TestStoreCreatesTheOverviewSchema(t *testing.T) {
	db, _ := openTestStore(t)
	want := map[string][]string{
		"cc_meta":     {"key", "value"},
		"cc_projects": {"id", "slug", "name", "board", "theme", "version", "locked_by_job", "draft_job", "last_build", "app_changed_since_job", "created_at", "updated_at"},
		"cc_builds": {"id", "project_id", "job_id", "origin", "status", "board", "project_version", "error_code", "diagnostics",
			"usage", "artifacts", "created_at", "started_at", "finished_at"},
		"cc_events": {"seq", "project_id", "type", "job_id", "build_id", "data", "created_at"},
		"cc_jobs": {"id", "project_id", "mode", "status", "phase", "prompt", "plan", "repair_attempt", "error_code", "error_message",
			"build_id", "staging_dir", "resume_of", "created_at", "started_at", "finished_at"},
		"cc_messages":       {"id", "project_id", "job_id", "role", "text", "attachments", "created_at"},
		"cc_agent_context":  {"job_id", "project_id", "data", "updated_at"},
		"cc_revisions":      {"id", "project_id", "label", "reason", "job_id", "build_id", "layout_sha256", "created_at"},
		"cc_revision_files": {"revision_id", "path", "sha256", "size"},
		"cc_devices": {"id", "project_id", "name", "board", "status", "token_hash", "build_id", "target_build_id", "ip", "rssi",
			"free_heap", "template_version", "ota_state", "last_error", "last_seen", "created_at", "expires_at"},
	}
	for table, cols := range want {
		if got := tableColumns(t, db, table); !slices.Equal(got, cols) {
			t.Errorf("%s columns = %v, want %v", table, got, cols)
		}
	}
	for _, index := range []string{"cc_builds_project", "cc_events_project", "cc_devices_project"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&name); err != nil {
			t.Errorf("index %s missing: %v", index, err)
		}
	}
	if v, err := storeSchemaVersion(db); err != nil || v != 1 {
		t.Fatalf("schema version = %d, %v; want 1", v, err)
	}
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v; want wal", mode, err)
	}
}

func TestStoreReopenKeepsDataAndVersion(t *testing.T) {
	db, path := openTestStore(t)
	if _, err := db.Exec(`INSERT INTO cc_meta(key,value) VALUES('probe','1')`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	again, err := openStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	var probe string
	if err := again.QueryRow(`SELECT value FROM cc_meta WHERE key='probe'`).Scan(&probe); err != nil || probe != "1" {
		t.Fatalf("probe = %q, %v", probe, err)
	}
	if v, _ := storeSchemaVersion(again); v != 1 {
		t.Fatalf("version after reopen = %d", v)
	}
}

func TestStoreRefusesANewerSchema(t *testing.T) {
	db, path := openTestStore(t)
	if _, err := db.Exec(`UPDATE cc_meta SET value='99' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if again, err := openStore(path); err == nil {
		_ = again.Close()
		t.Fatal("openStore accepted schema version 99")
	}
}

func TestStoreMigrationBacksUpAnExistingDatabase(t *testing.T) {
	db, path := openTestStore(t)
	_ = db.Close()
	saved := storeMigrations
	t.Cleanup(func() { storeMigrations = saved })
	storeMigrations = append(append([]func(*sql.Tx) error(nil), saved...), func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE cc_probe_v2 (id TEXT)`)
		return err
	})
	again, err := openStore(path)
	if err != nil {
		t.Fatalf("migrate to v2: %v", err)
	}
	defer again.Close()
	if v, _ := storeSchemaVersion(again); v != 2 {
		t.Fatalf("version = %d, want 2", v)
	}
	if _, err := os.Stat(path + ".v1.bak"); err != nil {
		t.Fatalf("backup before migration missing: %v", err)
	}
}

func TestRecoverStoreMarksUnfinishedWorkInterrupted(t *testing.T) {
	db, _ := openTestStore(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	now := "2026-10-10T08:00:00.000Z"
	exec(`INSERT INTO cc_projects(id,slug,name,board,theme,locked_by_job,created_at,updated_at) VALUES
		('ccp_1','a','A','esp32-2432s028r','clear','ccj_run',?,?),('ccp_2','b','B','esp32-2432s028r','clear','ccj_wait',?,?)`, now, now, now, now)
	exec(`INSERT INTO cc_builds(id,project_id,origin,status,board,project_version,created_at) VALUES
		('ccb_q','ccp_1','user','queued','esp32-2432s028r',1,?),('ccb_r','ccp_1','user','running','esp32-2432s028r',1,?),
		('ccb_s','ccp_1','user','succeeded','esp32-2432s028r',1,?)`, now, now, now)
	exec(`INSERT INTO cc_jobs(id,project_id,mode,status,phase,prompt,created_at) VALUES
		('ccj_run','ccp_1','change','running','coding','x',?),('ccj_q','ccp_1','change','queued','queued','x',?),
		('ccj_wait','ccp_2','new','awaiting_confirmation','planning','x',?),('ccj_ok','ccp_2','new','succeeded','done','x',?)`, now, now, now, now)

	recovered, err := recoverStore(context.Background(), db, "2026-10-10T09:00:00.000Z")
	if err != nil {
		t.Fatalf("recoverStore: %v", err)
	}
	if len(recovered) != 2 || recovered[0].ID != "ccb_q" || recovered[1].ID != "ccb_r" {
		t.Fatalf("recovered = %+v, want ccb_q and ccb_r", recovered)
	}
	status := func(table, id string) (string, string) {
		var st, code string
		col := "error_code"
		if err := db.QueryRow(`SELECT status, `+col+` FROM `+table+` WHERE id=?`, id).Scan(&st, &code); err != nil {
			t.Fatal(err)
		}
		return st, code
	}
	for _, id := range []string{"ccb_q", "ccb_r"} {
		if st, code := status("cc_builds", id); st != BuildInterrupted || code != CodeBuildInterrupted {
			t.Errorf("build %s = %s/%s, want interrupted/%s", id, st, code, CodeBuildInterrupted)
		}
	}
	if st, _ := status("cc_builds", "ccb_s"); st != BuildSucceeded {
		t.Errorf("finished build changed to %s", st)
	}
	for id, want := range map[string]string{"ccj_run": "interrupted", "ccj_q": "interrupted", "ccj_wait": "awaiting_confirmation", "ccj_ok": "succeeded"} {
		if st, _ := status("cc_jobs", id); st != want {
			t.Errorf("job %s = %s, want %s", id, st, want)
		}
	}
	var lock1, lock2 string
	_ = db.QueryRow(`SELECT locked_by_job FROM cc_projects WHERE id='ccp_1'`).Scan(&lock1)
	_ = db.QueryRow(`SELECT locked_by_job FROM cc_projects WHERE id='ccp_2'`).Scan(&lock2)
	if lock1 != "" || lock2 != "ccj_wait" {
		t.Fatalf("locks = %q/%q, want released/ccj_wait", lock1, lock2)
	}
}
