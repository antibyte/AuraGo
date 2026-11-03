package memory

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMemorySecondOpenPreservesLivePublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	first, err := NewSQLiteMemory(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := first.db.Exec(`INSERT INTO memory_maintenance_meta(key,value) VALUES('snapshot.publication.live','publishing')`); err != nil {
		t.Fatal(err)
	}
	if _, err := first.db.Exec(`INSERT INTO archived_messages(role, content, consolidation_status, consolidation_claimed_at) VALUES('user','fixture','in_progress',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	second, err := NewSQLiteMemory(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var status string
	if err := first.db.QueryRow(`SELECT consolidation_status FROM archived_messages WHERE content='fixture'`).Scan(&status); err != nil || status != "in_progress" {
		t.Fatalf("live claim=%q: %v", status, err)
	}
	var count int
	if err := first.db.QueryRow(`SELECT count(*) FROM memory_maintenance_meta WHERE key='snapshot.publication.live'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("live marker count=%d: %v", count, err)
	}
}

func TestMemoryDatabasePreservesExistingParentPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "memory.db")
	stm, err := NewSQLiteMemory(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	parent, err := os.Stat(dir)
	if err != nil || parent.Mode().Perm() != 0755 {
		t.Fatalf("parent mode changed: %v", err)
	}
	db, err := os.Stat(path)
	if err != nil || db.Mode().Perm() != 0600 {
		t.Fatalf("database not private: %v", err)
	}
}
