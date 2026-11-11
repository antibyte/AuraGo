package flows

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestFlowsV9MigrationAddsPurgeMarksAfterAVerifiedBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	publishRevisions(t, s, "flow_aaaaaaaaaa", 2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Turn it back into a schema 8 database.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DROP TABLE flow_version_purge; UPDATE flows_meta SET value='8' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	var version string
	if err := s.db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "9" {
		t.Fatalf("schema = %q, %v, want 9", version, err)
	}
	if got := markCount(t, s); got != 0 {
		t.Fatalf("marks = %d", got)
	}
	if got := versionRevisions(t, s, "flow_aaaaaaaaaa"); len(got) != 2 {
		t.Fatalf("migration changed the versions: %v", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, ".flows-v8-backup-*", "flows.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, %v, want one", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if err := backup.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version); err != nil || version != "8" {
		t.Fatalf("backup schema = %q, %v, want 8", version, err)
	}
	// A reopen at schema 9 only verifies; it takes no second backup.
	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if again, _ := filepath.Glob(filepath.Join(dir, ".flows-v8-backup-*")); len(again) != 1 {
		t.Fatalf("a reopen took another backup: %v", again)
	}
}
