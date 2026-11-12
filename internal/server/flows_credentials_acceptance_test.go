package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
	"aurago/internal/security"
)

const (
	accHeader = "HDRTOKEN-0123456789abcdef"
	accQuery  = "QRYTOKEN-0123456789abcdef"
	accBody   = "BODYTOKEN-0123456789abcdef"
	accKey    = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

const accFlowJSON = `{"schema":1,"name":"Wetter","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"req","type":"http.request","type_version":1,"label":"Anfrage","position":{"x":300,"y":0},
  "params":{"method":"POST","url":"https://api.example.com/v1/items",
   "headers":{"Authorization":"Bearer HDRTOKEN-0123456789abcdef"},"body":"{\"client_secret\":\"BODYTOKEN-0123456789abcdef\"}"}},
 {"id":"n_cccccccc","key":"req2","type":"http.request","type_version":1,"label":"Zweite Anfrage","position":{"x":600,"y":0},
  "params":{"method":"GET","url":"https://api.example.com/v1/items?api_key=QRYTOKEN-0123456789abcdef"}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}},
  {"id":"e_bbbbbbbb","source":{"node":"n_bbbbbbbb","port":"out"},"target":{"node":"n_cccccccc","port":"in"}}]}`

// A node holds one auth_secret, so the header token and the query token sit in two nodes of the same flow.

// filesHolding returns, for each needle, the files under root (relative, slash-separated, sorted) that hold it.
func filesHolding(t *testing.T, root string, needles ...string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	skipped := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			skipped++
			return nil
		}
		for _, n := range needles {
			if bytes.Contains(data, []byte(n)) {
				rel, _ := filepath.Rel(root, path)
				out[n] = append(out[n], filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, files := range out {
		sort.Strings(files)
	}
	if skipped > 0 {
		t.Logf("%d unreadable files skipped in the byte search", skipped)
	}
	return out
}

func countWhere(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestCredentialHygieneAcceptance(t *testing.T) {
	security.RetainProcessMasterKey(accKey)
	t.Cleanup(func() { security.RetainProcessMasterKey("") })
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	dataDir := filepath.Dir(s.flowsDBPath())
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		return flowsCall(t, s, method, "/api/desktop/flows"+path, token, body)
	}

	// 1. A flow with a token in a header, a query parameter and a body field, published twice.
	doc, err := flows.ParseFlow([]byte(accFlowJSON))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := s.Flows.CreateFlow(ctx, flows.CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	id := rec.ID
	found := map[string]bool{}
	for _, f := range flows.FindLiteralCredentials(rec.Draft) {
		found[f.Location] = true
	}
	if !found["header"] || !found["url_query"] || !found["body_field"] {
		t.Fatalf("findings by location = %v, want header, url_query and body_field (B1's rule)", found)
	}
	// The test server has no api_request tool, so the service Publish would refuse the http.request node
	// (NODE_UNAVAILABLE); the store publish is what the other flows server tests use.
	first, err := s.Flows.Store().Publish(ctx, id, rec.DraftRevision, time.Now()) // revision 1
	if err != nil {
		t.Fatalf("Publish 1: %v", err)
	}
	first.Draft.Name = "Wetter 2"
	draft, _, err := s.Flows.SaveDraft(ctx, id, first.Draft, first.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Flows.Store().Publish(ctx, id, draft, time.Now()); err != nil { // revision 2, live for now
		t.Fatalf("Publish 2: %v", err)
	}

	// 2, 3 and 4. History from before the redaction existed (a finished run, its steps, test data), a
	// migration snapshot that holds all of it, and revision 2 pinned by a waiting run. The rows are written
	// the way an old AuraGo wrote them: in clear.
	raw, err := sql.Open("sqlite", s.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`PRAGMA busy_timeout = 10000`); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
	params := `{"url":"https://api.example.com/v1/items?api_key=` + accQuery + `","headers":{"Authorization":"Bearer ` + accHeader + `"},"body":"{\"client_secret\":\"` + accBody + `\"}"}`
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO flow_runs (id, flow_id, revision, mode, status, started_at, trigger_data_json) VALUES ('run_old', ?, 1, 'live', 'success', ?, ?)`, []any{id, ts, `{"api_key":"` + accQuery + `"}`}},
		{`INSERT INTO flow_runs (id, flow_id, revision, mode, status, started_at) VALUES ('run_wait', ?, 2, 'live', 'waiting', ?)`, []any{id, ts}},
		{`INSERT INTO flow_run_steps (run_id, seq, node_id, status, params_json, output_json) VALUES ('run_old', 1, 'n_bbbbbbbb', 'success', ?, '{}')`, []any{params}},
		{`INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at) VALUES (?, 'n_aaaaaaaa', 'trigger', ?, ?)`, []any{id, `{"token":"` + accQuery + `"}`, ts}},
	} {
		if _, err := raw.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	const snapName = ".flows-v8-backup-abc123"
	if err := os.MkdirAll(filepath.Join(dataDir, snapName), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`VACUUM INTO ?`, filepath.Join(dataDir, snapName, "flows.db")); err != nil {
		t.Fatal(err)
	}

	// 5. Move the header and the query token through the move endpoint (B.3); the body field cannot move.
	cur, err := s.Flows.GetFlow(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	const hdrNode, qryNode = "n_bbbbbbbb", "n_cccccccc"
	move := func(node, location, path, name string) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"revision":%d,"node_id":%q,"location":%q,"path":%q,"secret_name":%q,"replace":false}`, cur.DraftRevision, node, location, path, name)
		return call(http.MethodPost, "/"+id+"/credentials/move", body)
	}
	// The body field cannot move (the node's auth_secret is still free here, so only the location refuses).
	if w := move(hdrNode, "body_field", "/client_secret", "wetter_body"); w.Code != http.StatusUnprocessableEntity || flowsBody(t, w)["code"] != "FLOW_CREDENTIAL_NOT_MOVABLE" {
		t.Fatalf("moving a body field = %d %s, want 422 FLOW_CREDENTIAL_NOT_MOVABLE", w.Code, w.Body.String())
	}
	// The answer carries the document, so it still shows the tokens not moved yet; it must not show the moved one.
	for _, m := range [][5]string{{hdrNode, "header", "Authorization", "wetter_header", accHeader}, {qryNode, "url_query", "api_key", "wetter_query", accQuery}} {
		w := move(m[0], m[1], m[2], m[3])
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), m[4]) {
			t.Fatalf("move %s = %d %s", m[1], w.Code, w.Body.String())
		}
		if cur, err = s.Flows.GetFlow(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	// Amendment 16: the body token is removed from the draft, as "Wert entfernen" does.
	for _, n := range cur.Draft.Nodes {
		if n.ID == hdrNode {
			delete(n.Params, "body")
		}
	}
	if _, _, err := s.Flows.SaveDraft(ctx, id, cur.Draft, cur.DraftRevision); err != nil {
		t.Fatal(err)
	}
	if cur, err = s.Flows.GetFlow(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Flows.Store().Publish(ctx, id, cur.DraftRevision, time.Now()); err != nil { // revision 3, live
		t.Fatalf("Publish 3: %v", err)
	}

	// 5. Purge: one job, the chosen snapshot, nothing running.
	if w := call(http.MethodPost, "/credentials/purge", `{"snapshots":["`+snapName+`"]}`); w.Code != http.StatusAccepted {
		t.Fatalf("purge = %d %s", w.Code, w.Body.String())
	}
	var state map[string]any
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		state = flowsBody(t, call(http.MethodGet, "/credentials/purge", ""))["state"].(map[string]any)
		if state["status"] != "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the purge did not finish: %v", state)
		}
	}
	if state["status"] != "done" || state["clean"] != true || state["versions_deleted"] != float64(1) || state["versions_pinned"] != float64(1) {
		t.Fatalf("purge state = %v, want done, clean, 1 deleted and 1 pinned", state)
	}
	backup := filepath.Join(dataDir, state["backup"].(string))

	// 7. The pinned revision survives and cannot be rolled back to; nothing else holds the header or the query token.
	versions, err := s.Flows.ListVersions(ctx, id)
	if err != nil || len(versions) != 2 || versions[0].Revision != 3 || versions[1].Revision != 2 {
		t.Fatalf("versions = %+v, %v, want revisions 3 and 2", versions, err)
	}
	if w := call(http.MethodPost, "/"+id+"/versions", fmt.Sprintf(`{"revision":2,"base_revision":%d}`, cur.DraftRevision)); w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_VERSION_PURGED" {
		t.Fatalf("rollback to the pinned marked version = %d %s", w.Code, w.Body.String())
	}
	for _, tok := range []string{accHeader, accQuery} {
		if n := countWhere(t, raw, `SELECT COUNT(*) FROM flow_versions WHERE instr(json, ?) > 0 AND revision <> 2`, tok); n != 0 {
			t.Fatalf("%s is in %d versions besides the pinned one", tok, n)
		}
		for _, col := range [][2]string{{"flow_runs", "trigger_data_json"}, {"flow_run_steps", "params_json"}, {"flow_test_data", "json"}, {"flows", "draft_json"}, {"flows", "live_json"}} {
			if n := countWhere(t, raw, `SELECT COUNT(*) FROM `+col[0]+` WHERE instr(`+col[1]+`, ?) > 0`, tok); n != 0 {
				t.Fatalf("%s is still in %s.%s", tok, col[0], col[1])
			}
		}
	}
	if n := countWhere(t, raw, `SELECT COUNT(*) FROM flow_runs r WHERE instr(r.trigger_data_json, ?) > 0`, accBody) +
		countWhere(t, raw, `SELECT COUNT(*) FROM flow_run_steps WHERE instr(params_json, ?) > 0`, accBody); n != 0 {
		t.Fatalf("the body token is still in the run history (%d rows)", n)
	}
	if _, err := os.Stat(filepath.Join(dataDir, snapName)); !os.IsNotExist(err) {
		t.Fatalf("the migration snapshot survived: %v", err)
	}

	// 8. The encrypted backup holds no plaintext, and 9. decrypts into a database that opens and holds the old versions.
	enc, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{accHeader, accQuery, accBody} {
		if bytes.Contains(enc, []byte(tok)) {
			t.Fatalf("the encrypted backup holds %s in clear", tok)
		}
	}
	plain := filepath.Join(t.TempDir(), "restored.db")
	if err := security.DecryptFileWithMasterKey(backup, plain, flows.PurgeBackupLabel, accKey); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var integrity string
	if err := restored.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("the decrypted backup does not open cleanly: %q, %v", integrity, err)
	}
	if n := countWhere(t, restored, `SELECT COUNT(*) FROM flow_versions`); n != 3 {
		t.Fatalf("the backup holds %d versions, want all 3", n)
	}
	if n := countWhere(t, restored, `SELECT COUNT(*) FROM flow_versions WHERE instr(json, ?) > 0`, accHeader); n == 0 {
		t.Fatal("the backup lost the old header token: it is not a real backup")
	}

	// 7. The waiting run ends: revision 2 is deleted at once.
	if err := s.Flows.Store().FinishRun(ctx, "run_wait", flows.RunResult{Status: flows.RunSuccess, FinishedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if versions, err = s.Flows.ListVersions(ctx, id); err != nil || len(versions) != 1 || versions[0].Revision != 3 {
		t.Fatalf("versions after the run ended = %+v, %v, want revision 3 only", versions, err)
	}

	// 6. A byte search of the data directory (flows.db, its WAL and SHM, every other file) after the pinned
	// run ended and the prune ran (amendment 14): none of the three tokens is anywhere. The store stays open
	// and is not checkpointed here, so the search sees exactly what is on disk; only the raw test
	// connection is closed. The encrypted backup is excluded and checked above.
	_ = raw.Close()
	held := filesHolding(t, dataDir, accHeader, accQuery, accBody)
	for tok, files := range held {
		var plainFiles []string
		for _, f := range files {
			if !strings.HasPrefix(filepath.Base(f), ".flows-purge-backup-") || !strings.HasSuffix(f, ".enc") {
				plainFiles = append(plainFiles, f)
			}
		}
		if len(plainFiles) != 0 {
			t.Fatalf("token %s is still on disk in %v", tok, plainFiles)
		}
	}
	for _, f := range []string{"flows.db", "flows.db-wal", "flows.db-shm"} {
		if _, err := os.Stat(filepath.Join(dataDir, f)); err != nil {
			t.Logf("byte search: %s not present (%v)", f, err)
		} else {
			t.Logf("byte search: %s holds none of the three tokens", f)
		}
	}

	// The report agrees: nothing is left to the owner.
	report := flowsBody(t, call(http.MethodGet, "/credentials/scan?fresh=1", ""))["report"].(map[string]any)
	if drafts, _ := report["drafts"].([]any); len(drafts) != 0 || report["runs"] != float64(0) || report["versions_purgeable"] != float64(0) || report["versions_pinned"] != float64(0) {
		t.Fatalf("report after the purge = %v", report)
	}
	if snaps, _ := report["snapshots"].([]any); len(snaps) != 0 {
		t.Fatalf("snapshots listed after the purge: %v", snaps)
	}
}
