package flows

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestSweepDeletesEveryKindOfPlaintextLeftoverAndAPurgeAfterACrashIsClean(t *testing.T) {
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	dir, _ := store.dir()
	work := filepath.Join(dir, purgeWorkPrefix+"crashed")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(work, "flows.db"),
		filepath.Join(dir, purgeTempPrefix+"abc"),
		filepath.Join(dir, "flows.db"+preRestoreTempExt),
		filepath.Join(dir, "flows.db"+restoreTempExt),
		filepath.Join(dir, ".enc-123.part"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("x "+fixtureQueryToken), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	enc := filepath.Join(dir, PurgeBackupPrefix+"20260101T000000.000Z"+PurgeBackupSuffix)
	if err := os.WriteFile(enc, []byte("cipher"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A purge sweeps the leftovers first, so it can reach Clean although the crash left a token in clear.
	if _, err := svc.StartCredentialPurge(nil); err != nil {
		t.Fatal(err)
	}
	if st := waitPurge(t, svc); st.Status != "done" || !st.Clean {
		t.Fatalf("state after a crash leftover = %+v, want clean", st)
	}
	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("%s survived: %v", f, err)
		}
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Fatalf("the work directory survived: %v", err)
	}
	if _, err := os.Stat(enc); err != nil {
		t.Fatalf("a young encrypted backup was swept: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, purgeWorkPrefix+"*")); len(left) != 0 {
		t.Fatalf("the purge left work directories: %v", left)
	}
}

func TestSweepPurgeBackupsDeletesLeftoversDirectly(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, purgeWorkPrefix+"zz")
	_ = os.MkdirAll(work, 0o700)
	_ = os.WriteFile(filepath.Join(work, "x"), []byte("p"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, ".enc-9.part"), []byte("p"), 0o600)
	deleted, err := SweepPurgeBackups(dir, time.Now())
	if err != nil || len(deleted) != 2 {
		t.Fatalf("deleted = %v, %v", deleted, err)
	}
}

func TestPurgeWorkDirIsPrivateAndGoneAfterTheBackup(t *testing.T) {
	_, store := newPurgeService(t)
	seedCredentialStore(t, store)
	dir, _ := store.dir()
	work, err := newPurgeWorkDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(work)
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(work); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("work dir mode = %v, %v, want 0700", info.Mode().Perm(), err)
		}
	}
	if _, err := store.writePurgeBackup(context.Background(), dir, testPurgeKey, storeNow); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, purgeWorkPrefix+"*"))
	if len(left) != 1 || left[0] != work { // only the one this test made
		t.Fatalf("work directories after the backup = %v", left)
	}
}

func TestRestoreSwapFailurePutsTheOldDatabaseBack(t *testing.T) {
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
	if _, err := s.CreateFlow(ctx, sampleFlow("flow_dddddddddd"), "", storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	orig := renameFile
	defer func() { renameFile = orig }()
	renameFile = func(from, to string) error {
		if from == path { // a WAL appears next to the database, as after a crash
			if err := os.WriteFile(path+"-wal", []byte("stalewal"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if to == path && strings.HasSuffix(from, "restored.db") {
			return errors.New("injected rename failure")
		}
		return orig(from, to)
	}
	if err := RestorePurgeBackup(path, backup, testPurgeKey, storeNow); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("RestorePurgeBackup = %v, want the injected failure", err)
	}
	renameFile = orig
	if b, err := os.ReadFile(path + "-wal"); err != nil || string(b) != "stalewal" {
		t.Fatalf("the old WAL is not back: %q, %v", b, err)
	}
	_ = os.Remove(path + "-wal") // not a real WAL
	reopened, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.GetFlow(ctx, "flow_dddddddddd"); err != nil {
		t.Fatalf("the old database is not usable: %v", err)
	}
	if pre, _ := filepath.Glob(filepath.Join(dir, PreRestoreBackupPrefix+"*"+PurgeBackupSuffix)); len(pre) != 1 {
		t.Fatalf("pre-restore copies = %v, want one", pre)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, purgeWorkPrefix+"*")); len(left) != 0 {
		t.Fatalf("work directories left: %v", left)
	}
}

func TestVerifyFindsSnapshotsInADirectoryWithGlobCharacters(t *testing.T) {
	ctx := context.Background()
	base := filepath.Join(t.TempDir(), "AuraGo [dev]")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(base, "flows.db"), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snap := filepath.Join(base, ".flows-v7-backup-abc123")
	if err := os.MkdirAll(snap, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snap, "flows.db"), []byte("SQLite "+fixtureQueryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	remaining, err := store.verifyPurge(ctx, filepath.Join(base, "flows.db"), base, []string{fixtureQueryToken})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Where != ".flows-v7-backup-abc123/flows.db" {
		t.Fatalf("remaining = %+v, want the snapshot file", remaining)
	}
}

func TestVerifySearchesSnapshotsWithTheFullLiteralSetEvenForAKeptToken(t *testing.T) {
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	ctx := context.Background()
	// The token also sits in the draft: a kept document, so it stays in flows.db and is not a finding there.
	var rev int
	if err := store.db.QueryRow(`SELECT draft_revision FROM flows WHERE id = ?`, fixtureFlowID).Scan(&rev); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveDraft(ctx, fixtureFlowID, credentialDoc(fixtureFlowID, "Typed again", true), rev, storeNow); err != nil {
		t.Fatal(err)
	}
	dir, _ := store.dir()
	const snap = ".flows-v7-backup-abc123"
	if err := os.MkdirAll(filepath.Join(dir, snap), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, snap, "flows.db"), []byte(fixtureQueryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartCredentialPurge(nil); err != nil { // the snapshot is not chosen
		t.Fatal(err)
	}
	st := waitPurge(t, svc)
	if st.Status != "done" || st.Clean {
		t.Fatalf("state = %+v, want done but not clean", st)
	}
	found := false
	for _, r := range st.Remaining {
		if r.Where == snap+"/flows.db" {
			found = true
		}
		if r.Where == "flows.db" {
			t.Fatalf("the kept token was reported in flows.db: %+v", st.Remaining)
		}
	}
	if !found {
		t.Fatalf("remaining = %+v, want the unchosen snapshot named", st.Remaining)
	}
}
