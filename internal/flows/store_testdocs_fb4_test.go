package flows

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// E4: test runs of an unchanged draft share one stored document; documents no run refers to are
// pruned, and deleting the flow removes them.
func TestFB4TestRunsShareOneStoredDocument(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaafc")
	add := func(id string, doc *Flow, at time.Duration) {
		t.Helper()
		rec := RunRecord{ID: id, FlowID: f.ID, Mode: ModeTest, TriggerNode: f.Nodes[0].ID, Status: RunSuccess, StartedAt: storeNow.Add(at)}
		if err := s.CreateRun(ctx, rec, doc); err != nil {
			t.Fatal(err)
		}
	}
	add("run_aaaaaaaafca", f, 0)
	add("run_aaaaaaaafcb", f, time.Second)
	if n := countRows(t, s, "flow_test_docs"); n != 1 {
		t.Fatalf("stored documents = %d, want 1 for two runs of the same draft", n)
	}
	var legacy int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_runs WHERE doc_json != ''`).Scan(&legacy); err != nil || legacy != 0 {
		t.Fatalf("runs still carry their own document copy: %d, %v", legacy, err)
	}
	for _, id := range []string{"run_aaaaaaaafca", "run_aaaaaaaafcb"} {
		if doc, err := s.GetRunDoc(ctx, id); err != nil || doc.Name != f.Name {
			t.Fatalf("GetRunDoc(%s) = %+v, %v", id, doc, err)
		}
	}
	other, err := f.Clone()
	if err != nil {
		t.Fatal(err)
	}
	other.Name = "Other"
	add("run_aaaaaaaafcc", other, 2*time.Second)
	if n := countRows(t, s, "flow_test_docs"); n != 2 {
		t.Fatalf("stored documents = %d, want 2", n)
	}
	if _, err := s.PruneRunsByMode(ctx, 0, 10, 1, storeNow.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, "flow_test_docs"); n != 1 {
		t.Fatalf("documents after pruning = %d, want only the one the remaining run uses", n)
	}
	if doc, err := s.GetRunDoc(ctx, "run_aaaaaaaafcc"); err != nil || doc.Name != "Other" {
		t.Fatalf("GetRunDoc(remaining) = %+v, %v", doc, err)
	}
	if err := s.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, "flow_test_docs"); n != 0 {
		t.Fatalf("documents after deleting the flow = %d", n)
	}
}

// E4: a flows.db from before doc_hash gets the column on open; its test runs keep their own
// document copy and new test runs use the shared table.
func TestFB4OpenStoreMigratesDatabaseFromBeforeDocHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flows.db")
	ctx := context.Background()
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	f := createStoredFlow(t, s, "flow_aaaaaaaafe")
	legacy := RunRecord{ID: "run_aaaaaaaafea", FlowID: f.ID, Mode: ModeTest, TriggerNode: f.Nodes[0].ID, Status: RunSuccess, StartedAt: storeNow}
	if err := s.CreateRun(ctx, legacy, nil); err != nil {
		t.Fatal(err)
	}
	doc, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// Turn the database into the old release's shape: no doc_hash column, no flow_test_docs.
	for _, stmt := range []string{
		`DROP INDEX idx_flow_runs_doc_hash`,
		`ALTER TABLE flow_runs DROP COLUMN doc_hash`,
		`DROP TABLE flow_test_docs`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := s.db.Exec(`UPDATE flow_runs SET doc_json = ? WHERE id = ?`, string(doc), legacy.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatalf("OpenStore on the old schema: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	var cols int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('flow_runs') WHERE name = 'doc_hash'`).Scan(&cols); err != nil || cols != 1 {
		t.Fatalf("doc_hash column count = %d, %v", cols, err)
	}
	if got, err := s.GetRunDoc(ctx, legacy.ID); err != nil || got.Name != f.Name {
		t.Fatalf("GetRunDoc(legacy) = %+v, %v", got, err)
	}
	fresh := RunRecord{ID: "run_aaaaaaaafeb", FlowID: f.ID, Mode: ModeTest, TriggerNode: f.Nodes[0].ID, Status: RunSuccess, StartedAt: storeNow.Add(time.Second)}
	if err := s.CreateRun(ctx, fresh, f); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, s, "flow_test_docs"); n != 1 {
		t.Fatalf("stored documents = %d, want 1 for the new test run", n)
	}
	if got, err := s.GetRunDoc(ctx, fresh.ID); err != nil || got.Name != f.Name {
		t.Fatalf("GetRunDoc(new) = %+v, %v", got, err)
	}
}
