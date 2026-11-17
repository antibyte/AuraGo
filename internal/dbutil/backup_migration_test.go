package dbutil

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/fileutil"
)

func TestBackupSQLiteForMigrationReusesOnlyIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	db, err := Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA wal_autocheckpoint=0; CREATE TABLE records(value TEXT); INSERT INTO records VALUES ('before')`); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "schema-v17.bak")
	first, err := BackupSQLiteForMigration(context.Background(), db, target)
	if err != nil {
		t.Fatal(err)
	}
	assertBackupPrivate(t, first)
	for i := 0; i < 3; i++ {
		if next, err := BackupSQLiteForMigration(context.Background(), db, target); err != nil || next != first {
			t.Fatalf("unchanged retry %d created another backup: %s, %v", i, next, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if next, err := BackupSQLiteForMigration(context.Background(), db, target); err != nil || next != first {
		t.Fatalf("restart did not reuse backup: %s, %v", next, err)
	}
	if _, err := db.Exec(`PRAGMA wal_autocheckpoint=0; INSERT INTO records VALUES ('committed WAL')`); err != nil {
		t.Fatal(err)
	}
	second, err := BackupSQLiteForMigration(context.Background(), db, target)
	if err != nil || second == first {
		t.Fatalf("changed data reused old backup: %s, %v", second, err)
	}
	assertBackupPrivate(t, second)
	copies, _ := filepath.Glob(filepath.Join(dir, "schema-v17-*.bak"))
	if len(copies) != 2 {
		t.Fatalf("expected two distinct snapshots, got %v", copies)
	}
	copyDB, err := Open(second)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var count int
	if err := copyDB.QueryRow(`SELECT count(*) FROM records`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("committed WAL missing: count=%d, %v", count, err)
	}
}

func TestBackupSQLiteForMigrationRefusesCorruptedExistingCopy(t *testing.T) {
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
	first, err := BackupSQLiteForMigration(context.Background(), db, target)
	if err != nil {
		t.Fatal(err)
	}
	corrupt, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	corrupt[len(corrupt)-1] ^= 1
	if err := os.WriteFile(first, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BackupSQLiteForMigration(context.Background(), db, target); err == nil {
		t.Fatal("corrupted content-addressed backup reused")
	}
	after, err := os.ReadFile(first)
	if err != nil || !bytes.Equal(after, corrupt) {
		t.Fatal("existing copy was changed")
	}
	stages, _ := filepath.Glob(filepath.Join(dir, ".sqlite-backup-*"))
	if len(stages) != 0 {
		t.Fatalf("staging leaked: %v", stages)
	}
}

func TestBackupSQLiteForMigrationHandlesPublicationRaceAndRevocation(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE records(value TEXT)`); err != nil {
		t.Fatal(err)
	}
	oldLink := sqliteBackupLink
	t.Cleanup(func() { sqliteBackupLink = oldLink })
	calls := 0
	sqliteBackupLink = func(source, destination string) error {
		calls++
		if err := os.Link(source, destination); err != nil {
			return err
		}
		return os.ErrExist
	}
	target := filepath.Join(dir, "migration.bak")
	first, err := BackupSQLiteForMigration(context.Background(), db, target)
	if err != nil || calls != 1 {
		t.Fatalf("publication race: %s, %v, calls=%d", first, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = fileutil.WithPublicationGate(ctx, func(commit func() error) error { cancel(); return commit() })
	if _, err := BackupSQLiteForMigration(ctx, db, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("revoked reuse succeeded: %v", err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatal("revocation removed the existing backup")
	}
}
