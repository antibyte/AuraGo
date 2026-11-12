package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func waitPurge(t *testing.T, svc *Service) PurgeState {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if st := svc.CredentialPurgeState(); st.Status != "running" {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("the purge did not finish: %+v", svc.CredentialPurgeState())
		}
	}
}

// tokenRows lists "table.column" once for every row whose column holds token.
func tokenRows(t *testing.T, s *Store, token string) []string {
	t.Helper()
	var hits []string
	for _, c := range [][2]string{{"flow_runs", "doc_json"}, {"flow_runs", "trigger_data_json"}, {"flow_run_steps", "params_json"},
		{"flow_run_steps", "output_json"}, {"flow_test_data", "json"}, {"flow_versions", "json"}, {"flows", "draft_json"}, {"flows", "live_json"}} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+c[0]+` WHERE instr(`+c[1]+`, ?) > 0`, token).Scan(&n); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			hits = append(hits, c[0]+"."+c[1])
		}
	}
	return hits
}

func TestPurgeJobBlacksOutDeletesAndVerifies(t *testing.T) {
	ctx := context.Background()
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	dir, _ := store.dir()
	const snap = ".flows-v7-backup-abc123"
	if err := os.MkdirAll(filepath.Join(dir, snap), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, snap, "flows.db"), []byte(fixtureQueryToken), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartCredentialPurge([]string{snap}); err != nil {
		t.Fatal(err)
	}
	st := waitPurge(t, svc)
	if st.Status != "done" || !st.Clean || len(st.Remaining) != 0 {
		t.Fatalf("state = %+v", st)
	}
	if st.VersionsDeleted != 1 || st.VersionsPinned != 1 || st.RowsRedacted < 3 {
		t.Fatalf("deleted %d pinned %d redacted %d, want 1, 1 and at least 3", st.VersionsDeleted, st.VersionsPinned, st.RowsRedacted)
	}
	if got := fmt.Sprint(versionRevisions(t, store, fixtureFlowID)); got != "[3 2]" {
		t.Fatalf("versions = %s, want [3 2]: revision 1 deleted, revision 2 pinned by the waiting run", got)
	}
	if _, err := os.Stat(filepath.Join(dir, snap)); !os.IsNotExist(err) {
		t.Fatalf("the snapshot is still there: %v", err)
	}
	for _, token := range []string{fixtureQueryToken, strings.TrimPrefix(fixtureHeaderToken, "Bearer ")} {
		if hits := tokenRows(t, store, token); fmt.Sprint(hits) != "[flow_versions.json]" {
			t.Fatalf("%s is still in %v, want only the pinned version", token, hits)
		}
	}
	var params string
	if err := store.db.QueryRow(`SELECT params_json FROM flow_run_steps WHERE run_id = 'run_old'`).Scan(&params); err != nil ||
		!strings.Contains(params, CredentialRedactionMarker) || !strings.Contains(params, "api.example.com") {
		t.Fatalf("step params = %q, %v: the token should be the marker and the host should stay", params, err)
	}

	// The encrypted backup holds no plaintext and restores to a database that still has the old versions.
	enc := filepath.Join(dir, st.Backup)
	data, err := os.ReadFile(enc)
	if err != nil || bytes.Contains(data, []byte(fixtureQueryToken)) {
		t.Fatalf("backup unreadable or holds plaintext: %v", err)
	}
	plain := filepath.Join(t.TempDir(), "restored.db")
	if err := security.DecryptFileWithMasterKey(enc, plain, PurgeBackupLabel, testPurgeKey); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var n int
	if err := restored.QueryRow(`SELECT COUNT(*) FROM flow_versions`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("the backup holds %d versions, %v, want 3", n, err)
	}

	// The waiting run ends: revision 2 goes at once and no byte of the token is left in the files.
	if err := store.FinishRun(ctx, "run_wait", RunResult{Status: RunSuccess, FinishedAt: storeNow}); err != nil {
		t.Fatal(err)
	}
	if hits := tokenRows(t, store, fixtureQueryToken); len(hits) != 0 {
		t.Fatalf("the token is still in %v after the pinned run ended", hits)
	}
	for _, name := range []string{"flows.db", "flows.db-wal"} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil && bytes.Contains(b, []byte(fixtureQueryToken)) {
			t.Fatalf("%s still holds the token in bytes", name)
		}
	}
}

func TestPurgeRefusesWhileARunIsRunningAndHoldsNewRunsWhileItWorks(t *testing.T) {
	fx := newRunnerFixture(t, RunnerConfig{})
	svc, _ := newPurgeService(t)
	svc.runner, svc.store = fx.r, fx.store // the purge works on the fixture's runner and store
	seedCredentialStore(t, fx.store)
	a := fx.flow(t, "flow_holdaaaaaa", ConcurrencyQueue)
	b := fx.flow(t, "flow_holdbbbbbb", ConcurrencyQueue)
	fx.start(t, a, ModeLive, "a")
	fx.gate.waitStarted(t, "a")
	if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrPurgeRunsActive) {
		t.Fatalf("purge while a run runs = %v, want ErrPurgeRunsActive", err)
	}
	fx.gate.release("a")
	fx.waitFinished(t)

	block := make(chan struct{})
	svc.purge.stepHook = func(step PurgeStep) {
		if step == PurgeStepBackup {
			<-block
		}
	}
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		_, err := svc.StartCredentialPurge(nil)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrPurgeRunsActive) || time.Now().After(deadline) {
			t.Fatalf("StartCredentialPurge = %v", err)
		}
	}
	if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrPurgeBusy) {
		t.Fatalf("a second purge = %v, want ErrPurgeBusy", err)
	}
	if res := fx.start(t, b, ModeLive, "b"); res.Status != StartQueued {
		t.Fatalf("a run started during the purge has status %s, want queued", res.Status)
	}
	fx.gate.assertNotStarted(t)
	close(block)
	if st := waitPurge(t, svc); st.Status != "done" {
		t.Fatalf("state = %+v", st)
	}
	fx.gate.waitStarted(t, "b") // the hold ended with the job
	fx.gate.release("b")
	fx.waitFinished(t)
}

func TestPurgeStopsBeforeDestroyingAnythingWithoutAWorkingKeyAndIsIdempotent(t *testing.T) {
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	key := "not hex"
	svc.cfg.PurgeKey = func() string { return key }
	if _, err := svc.StartCredentialPurge(nil); err != nil {
		t.Fatal(err)
	}
	if st := waitPurge(t, svc); st.Status != "failed" || st.FailedStep != PurgeStepBackup || st.Error == "" {
		t.Fatalf("state with a key that cannot encrypt = %+v, want failed at backup", st)
	}
	if hits := tokenRows(t, store, fixtureQueryToken); len(hits) == 0 {
		t.Fatal("a failed backup must not have changed anything")
	}
	key = ""
	if _, err := svc.StartCredentialPurge(nil); !errors.Is(err, ErrPurgeNoKey) {
		t.Fatalf("no key = %v, want ErrPurgeNoKey", err)
	}
	key = testPurgeKey
	run := func() PurgeState {
		t.Helper()
		if _, err := svc.StartCredentialPurge(nil); err != nil {
			t.Fatal(err)
		}
		return waitPurge(t, svc)
	}
	if first := run(); first.Status != "done" || first.RowsRedacted == 0 {
		t.Fatalf("first run = %+v", first)
	}
	if second := run(); second.Status != "done" || second.VersionsDeleted != 0 || second.RowsRedacted != 0 || !second.Clean {
		t.Fatalf("a second run changed something: %+v", second)
	}
}

func TestPurgeSnapshotNamesAreValidatedAgainstThePatternAndTheDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".flows-v7-backup-abc123"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".flows-v8-backup-file1"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := validateSnapshotNames(dir, []string{".flows-v7-backup-abc123", ".flows-v7-backup-abc123"}); err != nil || len(got) != 1 {
		t.Fatalf("a valid name = %v, %v", got, err)
	}
	for _, bad := range []string{"../.flows-v7-backup-abc123", ".flows-v7-backup-abc123/..", "flows.db", `..\.flows-v7-backup-abc123`, ".flows-v7-backup-missing", ".flows-v8-backup-file1", ""} {
		if _, err := validateSnapshotNames(dir, []string{bad}); !errors.Is(err, ErrPurgeSnapshot) {
			t.Fatalf("%q = %v, want ErrPurgeSnapshot", bad, err)
		}
		if _, err := deleteSnapshots(dir, []string{bad}); bad != ".flows-v7-backup-missing" && bad != ".flows-v8-backup-file1" && !errors.Is(err, ErrPurgeSnapshot) {
			t.Fatalf("deleteSnapshots(%q) = %v, want ErrPurgeSnapshot", bad, err)
		}
	}
}
