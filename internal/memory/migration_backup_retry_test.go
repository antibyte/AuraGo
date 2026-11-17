package memory

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/dbutil"
)

func TestCoreMemoryFailedMigrationReusesBackup(t *testing.T) {
	dir := t.TempDir()
	db, err := dbutil.Open(filepath.Join(dir, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE core_memory(id INTEGER PRIMARY KEY, fact TEXT NOT NULL, normalized_fact TEXT NOT NULL DEFAULT '');
		INSERT INTO core_memory(fact) VALUES ('same fact'), ('Same Fact');
		CREATE TRIGGER stop_migration BEFORE DELETE ON core_memory BEGIN SELECT RAISE(ABORT, 'fixture migration failure'); END;`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := migrateCoreMemoryUniqueFacts(db, nil); err == nil {
			t.Fatal("fixture migration unexpectedly succeeded")
		}
		copies, _ := filepath.Glob(filepath.Join(dir, "memory.db.core-memory-normalized-*.bak"))
		if len(copies) != 1 {
			t.Fatalf("retry %d left %d backups", i, len(copies))
		}
	}
	if _, err := db.Exec(`DROP TRIGGER stop_migration`); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateCoreMemoryUniqueFacts(db, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationProbeNeverMutatesRetainedBackup(t *testing.T) {
	db, err := dbutil.Open(filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE records(value TEXT); INSERT INTO records VALUES ('keep')`); err != nil {
		t.Fatal(err)
	}
	backup, err := backupMainDatabase(db, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := freshMainDatabaseProbe(db, "fixture-probe")
	if err != nil || probe == backup || probe == "" {
		t.Fatalf("probe reused retained backup: %q, %v", probe, err)
	}
	probeDB, err := dbutil.Open(probe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := probeDB.Exec(`DELETE FROM records`); err != nil {
		t.Fatal(err)
	}
	if err := probeDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	if again, err := backupMainDatabase(db, "fixture"); err != nil || again != backup {
		t.Fatalf("probe corrupted retained backup: %q, %v", again, err)
	}
}
