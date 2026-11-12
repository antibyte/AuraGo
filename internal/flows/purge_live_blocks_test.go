package flows

import (
	"context"
	"fmt"
	"testing"
)

const liveBlockToken = "BLKTOKEN-0123456789abcdef"

// tokenBlockWithNewerRevision returns a block whose revision 1 holds a typed token and whose revision 2
// (live) is clean, so revision 1 is a purge candidate.
func tokenBlockWithNewerRevision(t *testing.T, s *Service) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	draft, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	draft.Nodes[1].Params["fields"] = []any{map[string]any{"name": "greeting", "type": "string", "required": true}, map[string]any{"name": "api_key", "type": "string", "required": true}}
	draft.Nodes[1].Params["values"] = map[string]any{"greeting": "Hello {{block_input.data.name}}", "api_key": liveBlockToken}
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, draft, block.DraftRevision)
	if err != nil {
		t.Fatalf("save block with token: %v %+v", err, issues)
	}
	withToken, issues, err := s.PublishBlock(ctx, block.ID, rev)
	if err != nil {
		t.Fatalf("publish block with token: %v %+v", err, issues)
	}
	return withToken
}

func publishCleanBlockRevision(t *testing.T, s *Service, block *FlowRecord) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	draft, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	draft.Nodes[1].Params["values"] = map[string]any{"greeting": "Hi {{block_input.data.name}}", "api_key": "{{block_input.data.name}}"}
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, draft, block.DraftRevision)
	if err != nil {
		t.Fatalf("save clean block: %v %+v", err, issues)
	}
	pub, issues, err := s.PublishBlock(ctx, block.ID, rev)
	if err != nil {
		t.Fatalf("publish clean block: %v %+v", err, issues)
	}
	return pub
}

func blockRevisionExists(t *testing.T, s *Store, id string, revision int) (exists, marked bool) {
	t.Helper()
	var n, m int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM flow_versions WHERE flow_id=? AND revision=?`, id, revision).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM flow_version_purge WHERE flow_id=? AND revision=?`, id, revision).Scan(&m); err != nil {
		t.Fatal(err)
	}
	return n > 0, m > 0
}

// A block revision with a token that the live revision of a parent still pins is not marked: the purge
// never stops a live flow. It stays in the report (live_blocks) and the purge is still Clean, because the
// dialog explains it the same way as a live document: it is listed under what stays and why, and the
// verify leaves its literal out of the flows.db files like the literal of a live document.
func TestPurgeKeepsABlockRevisionThatALiveParentPins(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	s.cfg.PurgeKey = func() string { return testPurgeKey }
	ctx := context.Background()
	block := tokenBlockWithNewerRevision(t, s)
	r := block.LiveRevision
	parent, _ := blockParent(t, s, block, "Live parent")
	if _, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision); err != nil {
		t.Fatalf("publish parent: %v %+v", err, issues)
	}
	if err := s.SetEnabled(ctx, parent.ID, true); err != nil {
		t.Fatal(err)
	}
	publishCleanBlockRevision(t, s, block)

	if _, err := s.StartCredentialPurge(nil); err != nil {
		t.Fatal(err)
	}
	st := waitPurge(t, s)
	if st.Status != "done" || !st.Clean || len(st.Remaining) != 0 {
		t.Fatalf("purge state = %+v, want done and clean (a held block revision is explained like a live document)", st)
	}
	exists, marked := blockRevisionExists(t, s.store, block.ID, r)
	if !exists || marked {
		t.Fatalf("held block revision: exists=%v marked=%v, want present and not marked", exists, marked)
	}
	if _, err := s.RunNow(ctx, parent.ID); err != nil {
		t.Fatalf("a new run of the live parent must be admitted: %v", err)
	}
	report, err := s.CredentialScanFresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprint([]CredentialBlockRef{{BlockID: block.ID, Revision: r, ParentIDs: []string{parent.ID}}})
	if got := fmt.Sprint(report.LiveBlocks); got != want || !report.NeedsAttention() {
		t.Fatalf("live_blocks = %s, want %s (needs attention %v)", got, want, report.NeedsAttention())
	}
	if report.VersionsPurgeable != 0 || report.VersionsPinned != 0 {
		t.Fatalf("a held block revision is not a purgeable or pinned version: %+v", report)
	}
}

// A block revision that only an old parent version pins is marked as before, and the pin keeps it alive.
func TestPurgeStillMarksABlockRevisionThatOnlyAnOldParentVersionPins(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	s.cfg.PurgeKey = func() string { return testPurgeKey }
	ctx := context.Background()
	block := tokenBlockWithNewerRevision(t, s)
	r := block.LiveRevision
	parent, call := blockParent(t, s, block, "Moved on parent")
	if _, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision); err != nil {
		t.Fatalf("publish parent: %v %+v", err, issues)
	}
	clean := publishCleanBlockRevision(t, s, block)

	// The parent moves to the clean block revision and is published again: revision r is only pinned by an old parent version.
	rec, err := s.GetFlow(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := rec.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	for i := range draft.Nodes {
		if draft.Nodes[i].ID == call {
			draft.Nodes[i].Params["revision"] = float64(clean.LiveRevision)
		}
	}
	rev, issues, err := s.SaveDraft(ctx, parent.ID, draft, rec.DraftRevision)
	if err != nil {
		t.Fatalf("save parent: %v %+v", err, issues)
	}
	if _, issues, err := s.Publish(ctx, parent.ID, rev); err != nil {
		t.Fatalf("publish parent again: %v %+v", err, issues)
	}

	if _, err := s.StartCredentialPurge(nil); err != nil {
		t.Fatal(err)
	}
	if st := waitPurge(t, s); st.Status != "done" {
		t.Fatalf("purge state = %+v", st)
	}
	exists, marked := blockRevisionExists(t, s.store, block.ID, r)
	if !exists || !marked {
		t.Fatalf("block revision pinned only by an old parent version: exists=%v marked=%v, want kept by the pin and marked", exists, marked)
	}
	report, err := s.CredentialScanFresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.LiveBlocks) != 0 {
		t.Fatalf("live_blocks = %+v, want none", report.LiveBlocks)
	}
}
