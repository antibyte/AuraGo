package flows

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// publishRevisions creates flow id and publishes it n times: revisions 1..n, revision n live.
func publishRevisions(t *testing.T, s *Store, id string, n int) {
	t.Helper()
	ctx := context.Background()
	f := sampleFlow(id)
	if _, err := s.CreateFlow(ctx, f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	draft := 1
	for i := 0; i < n; i++ {
		if i > 0 {
			f.Name = fmt.Sprintf("v%d", i)
			rev, err := s.SaveDraft(ctx, id, f, draft, storeNow)
			if err != nil {
				t.Fatalf("SaveDraft: %v", err)
			}
			draft = rev
		}
		if _, err := s.Publish(ctx, id, draft, storeNow); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
}

func versionRevisions(t *testing.T, s *Store, id string) []int {
	t.Helper()
	list, err := s.ListVersions(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := []int{}
	for _, v := range list {
		out = append(out, v.Revision)
	}
	return out
}

func markCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM flow_version_purge`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMarkedVersionsGoOnceNothingPinsThem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const id = "flow_bbbbbbbbbb"
	publishRevisions(t, s, id, 4)
	// A live run that is waiting still executes revision 2.
	if err := s.CreateRun(ctx, RunRecord{ID: "run_pinned", FlowID: id, Revision: 2, Mode: ModeLive, Status: RunWaiting,
		StartedAt: storeNow, TriggerNode: "n_aaaaaaaa", TriggerType: "manual"}, nil); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	// The live revision is never marked.
	if err := s.MarkVersionsForPurge(ctx, id, []int{1, 2, 3, 4}, storeNow); err != nil {
		t.Fatal(err)
	}
	if got := markCount(t, s); got != 3 {
		t.Fatalf("marks = %d, want 3 (revision 4 is live)", got)
	}
	n, err := s.PrunePurgedVersions(ctx, id)
	if err != nil || n != 2 {
		t.Fatalf("PrunePurgedVersions = %d, %v, want 2 (revisions 1 and 3)", n, err)
	}
	if got := fmt.Sprint(versionRevisions(t, s, id)); got != "[4 2]" {
		t.Fatalf("versions = %s, want [4 2]: the pinned one survives", got)
	}
	// Ending the run unpins revision 2 and deletes it at once.
	if err := s.FinishRun(ctx, "run_pinned", RunResult{Status: RunSuccess, FinishedAt: storeNow}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(versionRevisions(t, s, id)); got != "[4]" {
		t.Fatalf("versions after the run ended = %s, want [4]", got)
	}
	if got := markCount(t, s); got != 0 {
		t.Fatalf("marks left = %d, want 0 (they go with their version)", got)
	}
}

func TestRollbackRefusesAMarkedVersion(t *testing.T) {
	svc, _ := newServiceFixture(t, newFakeClock(storeNow))
	ctx := context.Background()
	rec := c16Publish(t, svc, simpleFlow("Gruss"))
	doc := simpleFlow("Gruss 2")
	doc.ID = rec.ID
	rev, _, err := svc.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Publish(ctx, rec.ID, rev); err != nil { // revisions 1 and 2, 2 is live
		t.Fatal(err)
	}
	if err := svc.Store().MarkVersionsForPurge(ctx, rec.ID, []int{1}, storeNow); err != nil {
		t.Fatal(err)
	}
	cur, err := svc.GetFlow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RestoreVersion(ctx, rec.ID, 1, cur.DraftRevision); !errors.Is(err, ErrVersionPurged) {
		t.Fatalf("RestoreVersion of a marked version = %v, want ErrVersionPurged", err)
	}
	if _, err := svc.Store().GetVersionForCopy(ctx, rec.ID, 2); err != nil {
		t.Fatalf("GetVersionForCopy of an unmarked version: %v", err)
	}
}
