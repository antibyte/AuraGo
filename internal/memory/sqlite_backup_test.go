package memory

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDatabaseBackupPublishesPrivateContentAndCleansStaging(t *testing.T) {
	parent := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(parent, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE fixture(value TEXT); INSERT INTO fixture VALUES('private fixture')`); err != nil {
		t.Fatal(err)
	}
	backup, err := backupMainDatabase(db, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	copy, err := sql.Open("sqlite", backup)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	var value string
	if err := copy.QueryRow(`SELECT value FROM fixture`).Scan(&value); err != nil || value != "private fixture" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	info, err := os.Stat(backup)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("backup permissions=%v", info.Mode().Perm())
	}
	staging, err := filepath.Glob(filepath.Join(parent, ".memory-backup-*"))
	if err != nil || len(staging) != 0 {
		t.Fatalf("staging=%v err=%v", staging, err)
	}
}
