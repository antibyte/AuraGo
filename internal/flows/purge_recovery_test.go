package flows

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestRestoreDoubleFailureProtectsRecoveryFromSweep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flows.db")
	store, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	publishRevisions(t, store, "flow_cccccccccc", 1)
	plain := filepath.Join(dir, "plain.db")
	if _, err := store.db.Exec(`VACUUM INTO ?`, plain); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, PurgeBackupPrefix+"source"+PurgeBackupSuffix)
	if err := security.EncryptFileWithMasterKey(plain, backup, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(plain)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	original := renameFile
	t.Cleanup(func() { renameFile = original })
	renameFile = func(from, to string) error {
		if to == path && (filepath.Base(from) == "restored.db" || filepath.Base(from) == "old.db") {
			return errors.New("injected restore and rollback failure")
		}
		return original(from, to)
	}
	err = RestorePurgeBackup(path, backup, testPurgeKey, storeNow)
	if err == nil {
		t.Fatal("double failure was not reported")
	}
	work, _ := filepath.Glob(filepath.Join(dir, purgeWorkPrefix+"*"))
	pre, _ := filepath.Glob(filepath.Join(dir, PreRestoreBackupPrefix+"*"+PurgeBackupSuffix))
	if len(work) != 1 || len(pre) != 1 {
		t.Fatalf("missing recovery: work=%v encrypted=%v", work, pre)
	}
	if !strings.Contains(err.Error(), work[0]) || !strings.Contains(err.Error(), pre[0]) {
		t.Fatalf("recovery paths missing: %v", err)
	}
	for _, p := range []string{backup, pre[0]} {
		if err := os.Chtimes(p, storeNow.Add(-8*24*time.Hour), storeNow.Add(-8*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SweepPurgeBackups(dir, storeNow); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(work[0], "old.db"), filepath.Join(work[0], purgeRecoveryMarker), pre[0]} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("sweep removed recovery %s: %v", p, err)
		}
	}
}

func TestSweepKeepsUnmarkedLegacyRecovery(t *testing.T) {
	dir := t.TempDir()
	work, err := newPurgeWorkDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(work, "old.db")
	if err := os.WriteFile(old, []byte("only remaining original"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SweepPurgeBackups(dir, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("legacy recovery removed: %v", err)
	}
}
