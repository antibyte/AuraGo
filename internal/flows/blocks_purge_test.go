package flows

import (
	"context"
	"errors"
	"testing"
)

// A block revision the credential purge marked is not readable, exportable or newly pinnable, while
// the pins that already exist keep it alive.
func TestMarkedBlockRevisionIsRefusedForReadsExportAndNewPinsButExistingPinsKeepIt(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	oldRevision := block.LiveRevision
	oldParent, _ := blockParent(t, s, block, "Pinned before the mark")
	if _, issues, err := s.Publish(ctx, oldParent.ID, oldParent.DraftRevision); err != nil {
		t.Fatalf("publish before the mark: %v %+v", err, issues)
	}
	newParent, _ := blockParent(t, s, block, "Pinned after the mark")

	// A newer block revision goes live, so the old one may be marked.
	draft, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	draft.Nodes[1].Params["values"] = map[string]any{"greeting": "Hi {{block_input.data.name}}"}
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, draft, block.DraftRevision)
	if err != nil {
		t.Fatalf("save block: %v %+v", err, issues)
	}
	if _, issues, err := s.PublishBlock(ctx, block.ID, rev); err != nil {
		t.Fatalf("publish block: %v %+v", err, issues)
	}
	if _, err := s.GetBlockVersion(ctx, block.ID, oldRevision); err != nil {
		t.Fatalf("an unmarked revision must be readable: %v", err)
	}
	if err := s.store.MarkVersionsForPurge(ctx, block.ID, []int{oldRevision}, storeNow); err != nil {
		t.Fatal(err)
	}

	if _, err := s.GetBlockVersion(ctx, block.ID, oldRevision); !errors.Is(err, ErrVersionPurged) {
		t.Fatalf("GetBlockVersion of a marked revision = %v, want ErrVersionPurged", err)
	}
	if _, err := s.ExportLocalPack(ctx, newParent.ID); !errors.Is(err, ErrVersionPurged) {
		t.Fatalf("pack export with a marked block = %v, want ErrVersionPurged", err)
	}
	if _, _, err := s.Publish(ctx, newParent.ID, newParent.DraftRevision); !errors.Is(err, ErrVersionPurged) || !errors.Is(err, ErrBlockUnavailable) {
		t.Fatalf("publish that would pin a marked block = %v, want ErrVersionPurged wrapping ErrBlockUnavailable", err)
	}
	var pins int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_block_version_pins WHERE block_flow_id=? AND block_revision=?`, block.ID, oldRevision).Scan(&pins); err != nil || pins != 1 {
		t.Fatalf("pins on the marked revision = %d, %v, want only the earlier one", pins, err)
	}

	// The earlier pin (a published parent, or a waiting run's run pin) keeps the revision through the
	// prune, and a pinned run reads its block through its own pin, not through the loaders.
	if _, err := s.store.PrunePurgedVersions(ctx, ""); err != nil {
		t.Fatal(err)
	}
	var kept int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_versions WHERE flow_id=? AND revision=?`, block.ID, oldRevision).Scan(&kept); err != nil || kept != 1 {
		t.Fatalf("the pinned marked revision was deleted: %d, %v", kept, err)
	}
}
