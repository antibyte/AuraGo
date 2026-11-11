package flows

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

var testPurgeKey = strings.Repeat("0123456789abcdef", 4)

func TestSweepPurgeBackupsDeletesAtSevenDaysAndKeepsSix(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	put := func(name string, age time.Duration) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	put(".flows-purge-backup-a.enc", 7*24*time.Hour+time.Minute)
	put(".flows-pre-restore-b.enc", 8*24*time.Hour)
	put(".flows-purge-backup-c.enc", 6*24*time.Hour+23*time.Hour)
	put("other.enc", 30*24*time.Hour)
	put(".flows-purge-backup-tmp-plain", 30*24*time.Hour) // a plaintext leftover is the scan's business
	deleted, err := SweepPurgeBackups(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(deleted)
	if strings.Join(deleted, ",") != ".flows-pre-restore-b.enc,.flows-purge-backup-a.enc" {
		t.Fatalf("deleted = %v", deleted)
	}
	for _, kept := range []string{".flows-purge-backup-c.enc", "other.enc", ".flows-purge-backup-tmp-plain"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Fatalf("%s was swept: %v", kept, err)
		}
	}
}

func TestRestorePurgeBackupBringsBackTheOldDatabaseAndKeepsTheCurrentOne(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "flows.db")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	publishRevisions(t, s, "flow_cccccccccc", 1)
	plain := filepath.Join(dir, "plain.db")
	if _, err := s.db.Exec(`VACUUM INTO ?`, plain); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, PurgeBackupPrefix+"test"+PurgeBackupSuffix)
	if err := security.EncryptFileWithMasterKey(plain, backup, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(plain)
	// The database moves on after the backup.
	if _, err := s.CreateFlow(ctx, sampleFlow("flow_dddddddddd"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := RestorePurgeBackup(path, backup, testPurgeKey, storeNow); err != nil {
		t.Fatalf("RestorePurgeBackup: %v", err)
	}
	restored, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, err := restored.GetFlow(ctx, "flow_cccccccccc"); err != nil {
		t.Fatalf("the backed-up flow is missing: %v", err)
	}
	if _, err := restored.GetFlow(ctx, "flow_dddddddddd"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the later flow survived the restore: %v", err)
	}
	pre, _ := filepath.Glob(filepath.Join(dir, PreRestoreBackupPrefix+"*"+PurgeBackupSuffix))
	if len(pre) != 1 {
		t.Fatalf("pre-restore backups = %v, want one", pre)
	}
	prePlain := filepath.Join(dir, "pre.db")
	if err := security.DecryptFileWithMasterKey(pre[0], prePlain, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	old, err := sql.Open("sqlite", prePlain)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	var n int
	if err := old.QueryRow(`SELECT COUNT(*) FROM flows WHERE id='flow_dddddddddd'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the pre-restore copy lost the later flow: %d, %v", n, err)
	}
}
