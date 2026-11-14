package dbutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/fileutil"
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
	for _, failure := range []string{"cancel", "cancel before publication", "cancel at publication gate", "closed database", "sync", "publication", "stage"} {
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
			oldSync, oldLink := sqliteBackupSync, sqliteBackupLink
			t.Cleanup(func() { sqliteBackupSync, sqliteBackupLink = oldSync, oldLink })
			switch failure {
			case "cancel":
				cancel()
			case "cancel before publication":
				sqliteBackupSync = func(f *os.File) error { err := f.Sync(); cancel(); return err }
			case "cancel at publication gate":
				ctx = fileutil.WithPublicationGate(ctx, func(commit func() error) error { cancel(); return commit() })
			case "closed database":
				db.Close()
			case "sync":
				sqliteBackupSync = func(*os.File) error { return errors.New("disk full") }
			case "publication":
				sqliteBackupLink = func(string, string) error { return os.ErrPermission }
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

func TestBackupSQLitePreservesConcurrentDestination(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE records(value TEXT); INSERT INTO records VALUES ('snapshot')"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "migration.bak")
	oldLink := sqliteBackupLink
	t.Cleanup(func() { sqliteBackupLink = oldLink })
	attempts := 0
	sqliteBackupLink = func(source, destination string) error {
		attempts++
		if attempts == 1 {
			if destination != target {
				t.Fatal("first publication did not use the requested destination")
			}
			if err := os.WriteFile(destination, []byte("concurrent backup"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return oldLink(source, destination)
	}
	actual, err := BackupSQLite(context.Background(), db, target)
	if err != nil {
		t.Fatal(err)
	}
	if actual == target || attempts != 2 {
		t.Fatalf("collision was not retried with a new name: %q (%d attempts)", actual, attempts)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "concurrent backup" {
		t.Fatalf("concurrent backup changed: %q, %v", data, err)
	}
	assertBackupPrivate(t, actual)
	copyDB, err := Open(actual)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var value string
	if err := copyDB.QueryRow("SELECT value FROM records").Scan(&value); err != nil || value != "snapshot" {
		t.Fatalf("new snapshot is invalid: %q, %v", value, err)
	}
	if stages, _ := filepath.Glob(filepath.Join(dir, ".sqlite-backup-*")); len(stages) != 0 {
		t.Fatalf("staging leaked: %v", stages)
	}
}
