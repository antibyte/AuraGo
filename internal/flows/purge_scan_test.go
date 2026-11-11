package flows

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fixtureHeaderToken = "Bearer HDRTOKEN-0123456789abcdef"
	fixtureQueryToken  = "QRYTOKEN-0123456789abcdef"
	fixtureFlowID      = "flow_eeeeeeeeee"
)

// credentialDoc is a trigger and an http.request; typed puts a query token and an Authorization header in it.
func credentialDoc(id, name string, typed bool) *Flow {
	b := newFlow(name)
	tr := b.node("start", "test.trigger", nil)
	params := map[string]any{"method": "GET", "url": "https://api.example.com/v1/items"}
	if typed {
		params["url"] = "https://api.example.com/v1/items?api_key=" + fixtureQueryToken
		params["headers"] = map[string]any{"Authorization": fixtureHeaderToken}
	}
	req := b.node("req", "http.request", params)
	b.edge(tr, PortOut, req)
	f := b.build()
	f.ID = id
	return f
}

func newPurgeService(t *testing.T) (*Service, *Store) {
	t.Helper()
	store := openTestStore(t)
	svc := NewService(store, catalogRegistry(t, fullEnv()), &Services{Tools: &fakeTools{}, Clock: newFakeClock(storeNow), Location: time.UTC},
		newFakeBridge(), ServiceConfig{PurgeKey: func() string { return testPurgeKey }}, discardLogger())
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	return svc, store
}

// seedCredentialStore models an owner who typed a token, published twice, then moved it into the vault
// and published again: revisions 1 and 2 hold the token, revision 3 (live) and the draft are clean. A
// waiting run pins revision 2, and old rows from before the redaction existed hold the token in clear.
func seedCredentialStore(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	f := credentialDoc(fixtureFlowID, "Alt", true)
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, fixtureFlowID, 1, storeNow); err != nil {
		t.Fatal(err)
	}
	f.Name = "Alt 2"
	rev, err := s.SaveDraft(ctx, fixtureFlowID, f, 1, storeNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, fixtureFlowID, rev, storeNow); err != nil {
		t.Fatal(err)
	}
	if rev, err = s.SaveDraft(ctx, fixtureFlowID, credentialDoc(fixtureFlowID, "Neu", false), rev, storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, fixtureFlowID, rev, storeNow); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"run_old", "run_wait"} {
		revision, status := 1, RunSuccess
		if id == "run_wait" {
			revision, status = 2, RunWaiting
		}
		if err := s.CreateRun(ctx, RunRecord{ID: id, FlowID: fixtureFlowID, Revision: revision, Mode: ModeLive, Status: status,
			StartedAt: storeNow, TriggerNode: "n_aaaaaaaa", TriggerType: "manual"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	steps := `{"url":"https://api.example.com/v1/items?api_key=` + fixtureQueryToken + `","headers":{"Authorization":"` + fixtureHeaderToken + `"}}`
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE flow_runs SET trigger_data_json = ? WHERE id = 'run_old'`, []any{`{"api_key":"` + fixtureQueryToken + `"}`}},
		{`INSERT INTO flow_run_steps (run_id, seq, node_id, status, params_json, output_json) VALUES ('run_old', 1, 'n_bbbbbbbb', 'success', ?, ?)`,
			[]any{steps, `{"body":"echo ` + fixtureQueryToken + `"}`}},
		{`INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at) VALUES (?, 'n_aaaaaaaa', 'trigger', ?, 'x')`,
			[]any{fixtureFlowID, `{"token":"` + fixtureQueryToken + `"}`}},
	} {
		if _, err := s.db.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheRedactionMarkerIsNeverAFinding(t *testing.T) {
	got := literalCredentialsInValue(map[string]any{
		"token":   CredentialRedactionMarker,
		"headers": map[string]any{"Authorization": CredentialRedactionMarker},
		"url":     "https://api.example.com/v1?api_key=" + CredentialRedactionMarker,
	})
	if len(got) != 0 {
		t.Fatalf("the redaction marker counts as a credential: %v (the purge could never converge)", got)
	}
}

func TestCredentialScanReportsEveryPlaceAPlaintextCredentialSits(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	seedCredentialStore(t, s)
	// A second flow whose owner did not move the credentials: draft and live both hold them.
	other := credentialDoc("flow_ffffffffff", "Unberuehrt", true)
	if _, err := s.CreateFlow(ctx, other, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, other.ID, 1, storeNow); err != nil {
		t.Fatal(err)
	}
	dir, err := s.dir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".flows-v7-backup-abc123", ".flows-v8-backup-def456"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "flows.db"), []byte("SQLite "+fixtureQueryToken), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	enc := filepath.Join(dir, PurgeBackupPrefix+"20260101T000000.000Z"+PurgeBackupSuffix)
	if err := os.WriteFile(enc, []byte("cipher"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, purgeTempPrefix+"1"), []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}

	sc, err := s.scanCredentials(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := listPurgeFiles(dir, &sc.report); err != nil {
		t.Fatal(err)
	}
	rep := sc.report
	if len(rep.Drafts) != 1 || rep.Drafts[0].FlowID != "flow_ffffffffff" || rep.Drafts[0].Findings < 2 || len(rep.Live) != 1 {
		t.Fatalf("drafts = %+v, live = %+v, want only the unmoved flow", rep.Drafts, rep.Live)
	}
	if rep.VersionsPurgeable != 1 || rep.VersionsPinned != 1 {
		t.Fatalf("versions purgeable %d pinned %d, want 1 and 1 (revision 1 free, revision 2 pinned by the waiting run)", rep.VersionsPurgeable, rep.VersionsPinned)
	}
	if rep.Runs != 1 || rep.Steps != 1 || rep.TestRows != 1 {
		t.Fatalf("runs %d steps %d test rows %d, want 1 each", rep.Runs, rep.Steps, rep.TestRows)
	}
	if len(rep.Snapshots) != 2 || rep.Snapshots[0].Name != ".flows-v7-backup-abc123" || rep.Snapshots[0].Bytes == 0 {
		t.Fatalf("snapshots = %+v", rep.Snapshots)
	}
	if len(rep.Backups) != 1 || !rep.Backups[0].ExpiresAt.After(rep.Backups[0].ModTime) || len(rep.Leftovers) != 1 {
		t.Fatalf("backups = %+v leftovers = %+v", rep.Backups, rep.Leftovers)
	}
	if _, ok := sc.rows[fixtureQueryToken]; !ok {
		t.Fatal("the token found in the run rows is not in the literal set")
	}
	if blob, _ := jsonForTest(rep); strings.Contains(blob, "HDRTOKEN") || strings.Contains(blob, "QRYTOKEN") {
		t.Fatal("the report carries a credential value")
	}
}

func jsonForTest(v any) (string, error) {
	data, err := json.Marshal(v)
	return string(data), err
}

func TestCredentialScanIsCachedAndFreshOnRequest(t *testing.T) {
	ctx := context.Background()
	svc, store := newPurgeService(t)
	seedCredentialStore(t, store)
	first, err := svc.CredentialScan(ctx)
	if err != nil || first.Runs != 1 {
		t.Fatalf("first scan = %+v, %v", first, err)
	}
	if _, err := store.db.Exec(`DELETE FROM flow_runs WHERE id = 'run_old'`); err != nil {
		t.Fatal(err)
	}
	if cached, _ := svc.CredentialScan(ctx); cached.Runs != 1 {
		t.Fatalf("the cached report changed: %+v", cached)
	}
	if fresh, err := svc.CredentialScanFresh(ctx); err != nil || fresh.Runs != 0 {
		t.Fatalf("fresh scan = %+v, %v", fresh, err)
	}
	if again, _ := svc.CredentialScan(ctx); again.Runs != 0 {
		t.Fatalf("a fresh scan must refill the cache: %+v", again)
	}
}
