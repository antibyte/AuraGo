package flows

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func dependencyPublishNext(t *testing.T, s *Service, rec *FlowRecord) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	doc, err := rec.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Name = fmt.Sprintf("Revision %d", rec.LiveRevision+1)
	var revision int
	if rec.Kind == KindBlock {
		revision, _, err = s.SaveBlockDraft(ctx, rec.ID, doc, rec.DraftRevision)
	} else {
		revision, _, err = s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	}
	if err != nil {
		t.Fatal(err)
	}
	if rec.Kind == KindBlock {
		rec, _, err = s.PublishBlock(ctx, rec.ID, revision)
	} else {
		rec, _, err = s.Publish(ctx, rec.ID, revision)
	}
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func dependencyPin(t *testing.T, s *Service, id string, revision int) resolvedBlock {
	t.Helper()
	version, err := s.GetBlockVersion(context.Background(), id, revision)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(version.Doc)
	if err != nil {
		t.Fatal(err)
	}
	return resolvedBlock{Ref: blockRef{FlowID: id, Revision: revision}, Document: version.Doc, Contract: version.Contract, Hash: sha256.Sum256(data)}
}

// These fixtures exercise the pin lifecycle directly; execution admission has
// separate end-to-end tests using real component.call nodes.
func dependencyAttach(t *testing.T, s *Service, parent *FlowRecord, runID string, blocks []resolvedBlock) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if runID == "" {
		err = pinBlockVersionTx(ctx, tx, parent.ID, parent.LiveRevision, blocks)
	} else {
		err = pinBlockRunTx(ctx, tx, runID, blocks)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestBlockDependentsPageAcrossKindsAndPinnedRevisions(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Dependency fixture")
	if err != nil {
		t.Fatal(err)
	}
	block = dependencyPublishNext(t, s, block)
	block = dependencyPublishNext(t, s, block)
	pins := []resolvedBlock{dependencyPin(t, s, block.ID, 1), dependencyPin(t, s, block.ID, 2)}
	const canary = "dependency-payload-must-stay-private"
	want := make(map[BlockDependency]bool)
	for i := 0; i < 2; i++ {
		parent, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow(canary)})
		if err != nil {
			t.Fatal(err)
		}
		parent, _, err = s.Publish(ctx, parent.ID, parent.DraftRevision)
		if err != nil {
			t.Fatal(err)
		}
		runID := fmt.Sprintf("run_dependency_%d", i)
		if err := s.store.CreateRun(ctx, RunRecord{ID: runID, FlowID: parent.ID, Mode: ModeTest, Status: RunSuccess, StartedAt: storeNow, TriggerData: map[string]any{"private": canary}}, parent.Draft); err != nil {
			t.Fatal(err)
		}
		publishedPins, runPins := pins, pins
		if i == 1 {
			publishedPins, runPins = pins[:1], pins[1:]
		}
		dependencyAttach(t, s, parent, "", publishedPins)
		dependencyAttach(t, s, parent, runID, runPins)
		for _, pin := range publishedPins {
			want[BlockDependency{Kind: "published", FlowID: parent.ID, Revision: parent.LiveRevision, BlockRevision: pin.Ref.Revision}] = true
		}
		for _, pin := range runPins {
			want[BlockDependency{Kind: "run", FlowID: parent.ID, RunID: runID, BlockRevision: pin.Ref.Revision}] = true
		}
	}
	for _, revision := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("revision_%d", revision), func(t *testing.T) {
			seen := make(map[BlockDependency]bool)
			cursor := ""
			for pageNumber := 0; ; pageNumber++ {
				if pageNumber >= 10 {
					t.Fatal("dependency pagination never terminated")
				}
				page, err := s.BlockDependents(ctx, block.ID, revision, cursor, 1)
				if err != nil || len(page.Items) != 1 {
					t.Fatalf("dependency page = %+v, %v", page, err)
				}
				item := page.Items[0]
				if !want[item] || seen[item] || (revision != 0 && item.BlockRevision != revision) {
					t.Fatalf("unexpected or repeated dependency: %+v", item)
				}
				seen[item] = true
				data, err := json.Marshal(page)
				if err != nil || strings.Contains(string(data), canary) {
					t.Fatalf("dependency metadata exposed payload: %v", err)
				}
				if page.NextCursor == "" {
					break
				}
				if page.NextCursor == cursor {
					t.Fatal("dependency cursor did not advance")
				}
				cursor = page.NextCursor
			}
			for item := range want {
				if (revision == 0 || item.BlockRevision == revision) && !seen[item] {
					t.Fatalf("pagination lost a dependency: %+v", item)
				}
			}
			page, err := s.BlockDependents(ctx, block.ID, revision, "", 100)
			if err != nil || len(page.Items) != len(seen) || page.NextCursor != "" {
				t.Fatalf("single-page result differs: %+v, %v", page, err)
			}
		})
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, block.LiveRevision); !errors.Is(err, ErrBlockInUse) {
		t.Fatalf("pinned block deletion = %v", err)
	}
	if rec, err := s.GetBlock(ctx, block.ID); err != nil || !reflect.DeepEqual(rec, block) {
		t.Fatalf("refused delete changed block: %v", err)
	}
}

func TestBlockPinsSurviveVersionPruningUntilTheirOwnRetentionEnds(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Retained block")
	if err != nil {
		t.Fatal(err)
	}
	block = dependencyPublishNext(t, s, block)
	block = dependencyPublishNext(t, s, block)
	parent, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Retained parent")})
	if err != nil {
		t.Fatal(err)
	}
	parent, _, err = s.Publish(ctx, parent.ID, parent.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	const runID = "run_retained_dependency"
	if err := s.store.CreateRun(ctx, RunRecord{ID: runID, FlowID: parent.ID, Mode: ModeTest, Status: RunSuccess, StartedAt: storeNow}, parent.Draft); err != nil {
		t.Fatal(err)
	}
	dependencyAttach(t, s, parent, "", []resolvedBlock{dependencyPin(t, s, block.ID, 1)})
	dependencyAttach(t, s, parent, runID, []resolvedBlock{dependencyPin(t, s, block.ID, 2)})
	for block.LiveRevision < maxStoredVersions+3 {
		block = dependencyPublishNext(t, s, block)
	}
	for _, revision := range []int{1, 2} {
		if _, err := s.GetBlockVersion(ctx, block.ID, revision); err != nil {
			t.Fatalf("pinned revision %d was pruned: %v", revision, err)
		}
	}
	if _, err := s.GetBlockVersion(ctx, block.ID, 3); err == nil {
		t.Fatal("unreferenced old revision was not pruned")
	}
	for parent.LiveRevision < maxStoredVersions+1 {
		parent = dependencyPublishNext(t, s, parent)
	}
	block = dependencyPublishNext(t, s, block)
	if _, err := s.GetBlockVersion(ctx, block.ID, 1); err == nil {
		t.Fatal("pruned parent version still pins its child")
	}
	if _, err := s.GetBlockVersion(ctx, block.ID, 2); err != nil {
		t.Fatalf("retained run lost its independent child pin: %v", err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, block.LiveRevision); !errors.Is(err, ErrBlockInUse) {
		t.Fatalf("run-pinned block deletion = %v", err)
	}
	removed, err := s.store.PruneRunsByMode(ctx, 1, 0, 0, storeNow.Add(48*time.Hour))
	if err != nil || removed != 1 {
		t.Fatalf("run retention = %d, %v", removed, err)
	}
	block = dependencyPublishNext(t, s, block)
	if _, err := s.GetBlockVersion(ctx, block.ID, 2); err == nil {
		t.Fatal("expired run still pins its child")
	}
	page, err := s.BlockDependents(ctx, block.ID, 0, "", 100)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("released dependencies = %+v, %v", page, err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, block.LiveRevision); err != nil {
		t.Fatalf("unpinned block cannot be deleted: %v", err)
	}
}
