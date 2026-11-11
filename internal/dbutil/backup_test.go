package dbutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupSQLiteIncludesWALAndPreservesExistingCopies(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA wal_autocheckpoint=0; CREATE TABLE records(value TEXT); INSERT INTO records VALUES ('committed WAL')`); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "source.db-wal")); err != nil || info.Size() == 0 {
		t.Fatalf("missing WAL: %v", err)
	}
	target := filepath.Join(dir, "migration.bak")
	if err := os.WriteFile(target, []byte("previous backup"), 0600); err != nil {
		t.Fatal(err)
	}
	actual, err := BackupSQLite(context.Background(), db, target)
	if err != nil {
		t.Fatal(err)
	}
	if actual == target {
		t.Fatal("overwrote previous backup")
	}
	before, _ := os.ReadFile(target)
	if string(before) != "previous backup" {
		t.Fatal("previous backup changed")
	}
	assertBackupPrivate(t, actual)
	copyDB, err := Open(actual)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var value string
	if err := copyDB.QueryRow(`SELECT value FROM records`).Scan(&value); err != nil || value != "committed WAL" {
		t.Fatalf("backup: %q %v", value, err)
	}
	stages, _ := filepath.Glob(filepath.Join(dir, ".sqlite-backup-*"))
	if len(stages) != 0 {
		t.Fatalf("staging leaked: %v", stages)
	}
}

func TestBackupSQLiteFailuresNeverPublishPartialCopy(t *testing.T) {
	for _, failure := range []string{"cancel", "cancel before publication", "closed database", "sync", "publication", "stage"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			db, err := Open(filepath.Join(dir, "source.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`CREATE TABLE records(value TEXT)`); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "migration.bak")
			if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			oldSync, oldRename := sqliteBackupSync, sqliteBackupRename
			t.Cleanup(func() { sqliteBackupSync, sqliteBackupRename = oldSync, oldRename })
			switch failure {
			case "cancel":
				cancel()
			case "cancel before publication":
				sqliteBackupSync = func(f *os.File) error { err := f.Sync(); cancel(); return err }
			case "closed database":
				db.Close()
			case "sync":
				sqliteBackupSync = func(*os.File) error { return errors.New("disk full") }
			case "publication":
				sqliteBackupRename = func(context.Context, string, string) error { return os.ErrPermission }
			case "stage":
				target = filepath.Join(target, "invalid.bak")
			}
			if _, err := BackupSQLite(ctx, db, target); err == nil {
				t.Fatal("expected failure")
			}
			data, _ := os.ReadFile(filepath.Join(dir, "migration.bak"))
			if string(data) != "old" {
				t.Fatal("existing copy changed")
			}
			stages, _ := filepath.Glob(filepath.Join(dir, ".sqlite-backup-*"))
			if len(stages) != 0 {
				t.Fatalf("staging leaked: %v", stages)
			}
			copies, _ := filepath.Glob(filepath.Join(dir, "migration.*.bak"))
			if len(copies) != 0 {
				t.Fatal("published failed copy")
			}
		})
	}
}
