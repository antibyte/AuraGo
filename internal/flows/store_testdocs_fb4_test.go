package flows

import (
	"context"
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
