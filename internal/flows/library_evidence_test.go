package flows

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestLibraryEditsPreserveDocumentsPinsAndExactScenarioEvidence(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Library evidence")
	parent, _, err := s.Publish(ctx, parent.ID, parent.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	keys := &scenarioTestKeys{}
	scenario, err := s.SaveScenario(ctx, parent.ID, parent.DraftRevision, 0, Scenario{
		Name: "Exact pinned greeting", TriggerNode: parent.Draft.Nodes[0].ID,
		TriggerData: map[string]any{"sample": false}, At: storeNow,
	}, keys)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveTriggerSample(ctx, parent.ID, parent.Draft.Nodes[0].ID, map[string]any{"saved": 0}); err != nil {
		t.Fatal(err)
	}
	before := libraryMigrationData(t, s.store.db)
	beforeMission, ok := bridge.mission(parent.MissionID)
	if !ok {
		t.Fatal("published parent mission missing")
	}
	assertUnchanged := func() {
		t.Helper()
		if !reflect.DeepEqual(before, libraryMigrationData(t, s.store.db)) {
			t.Fatal("library edits changed executable documents, revisions, timestamps, pins or test evidence")
		}
		afterMission, ok := bridge.mission(parent.MissionID)
		if !ok || !reflect.DeepEqual(beforeMission, afterMission) || len(bridge.startedRuns()) != 0 {
			t.Fatal("library edits altered or executed the parent mission")
		}
		got, err := s.GetScenario(ctx, parent.ID, scenario.ID, keys)
		if err != nil || got.Stale || got.Version != scenario.Version {
			t.Fatalf("library edits invalidated exact scenario: stale=%v, version=%d, %v", got.Stale, got.Version, err)
		}
		result, err := s.SimulateScenario(ctx, parent.ID, scenario.ID, parent.DraftRevision, scenario.Version, keys)
		if err != nil || result.Mode != ModeSimulation || result.Status != RunSuccess {
			t.Fatalf("original revision evidence no longer replays: %+v, %v", result, err)
		}
	}
	folder, err := s.CreateLibraryFolder(ctx, "Home automations")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := s.UpdateFlowLibrary(ctx, parent.ID, LibraryUpdate{BaseRevision: 0, SetFolder: true, FolderID: &folder.ID, SetTags: true, Tags: []string{"Morning", "家庭"}})
	if err != nil {
		t.Fatal(err)
	}
	folder, err = s.UpdateLibraryFolder(ctx, folder.ID, "Daily routines", folder.Revision)
	if err != nil {
		t.Fatal(err)
	}
	parentNote, err := s.SetVersionNote(ctx, parent.ID, KindFlow, parent.LiveRevision, 0, "Initial review\nThe local greeting is verified.")
	if err != nil {
		t.Fatal(err)
	}
	blockNote, err := s.SetVersionNote(ctx, block.ID, KindBlock, block.LiveRevision, 0, "Reusable greeting contract")
	if err != nil {
		t.Fatal(err)
	}
	assertUnchanged()

	other, err := s.CreateLibraryFolder(ctx, "Reviewed")
	if err != nil {
		t.Fatal(err)
	}
	meta, err = s.UpdateFlowLibrary(ctx, parent.ID, LibraryUpdate{BaseRevision: meta.Revision, SetFolder: true, FolderID: &other.ID})
	if err != nil || !reflect.DeepEqual(meta.Tags, []string{"Morning", "家庭"}) {
		t.Fatalf("folder-only edit lost tags: %+v, %v", meta, err)
	}
	if err := s.DeleteLibraryFolder(ctx, folder.ID, folder.Revision); err != nil {
		t.Fatal(err)
	}
	meta, err = s.UpdateFlowLibrary(ctx, parent.ID, LibraryUpdate{BaseRevision: meta.Revision, SetFolder: true, SetTags: true, Tags: []string{}})
	if err != nil || meta.FolderID != nil || len(meta.Tags) != 0 || meta.Revision != 3 {
		t.Fatalf("explicit clear did not retain CAS: %+v, %v", meta, err)
	}
	if err := s.DeleteLibraryFolder(ctx, other.ID, other.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetVersionNote(ctx, parent.ID, KindFlow, parent.LiveRevision, parentNote.Revision, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetVersionNote(ctx, block.ID, KindBlock, block.LiveRevision, blockNote.Revision, "Updated description only"); err != nil {
		t.Fatal(err)
	}
	assertUnchanged()
}

func TestLibraryNotesFollowPinnedVersionRetentionAndDeletion(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	if _, err := s.SetVersionNote(ctx, block.ID, KindBlock, block.LiveRevision, 0, "Pinned original contract"); err != nil {
		t.Fatal(err)
	}
	parent, _ := blockParent(t, s, block, "Pinned annotation")
	text := "Parent keeps the original block revision"
	parent, _, err := s.PublishWithNote(ctx, parent.ID, parent.DraftRevision, &text)
	if err != nil {
		t.Fatal(err)
	}
	folder, err := s.CreateLibraryFolder(ctx, "Retained empty folder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateFlowLibrary(ctx, parent.ID, LibraryUpdate{BaseRevision: 0, SetFolder: true, FolderID: &folder.ID}); err != nil {
		t.Fatal(err)
	}
	publishNext := func(index int) {
		t.Helper()
		doc, err := block.Draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		doc.Description = fmt.Sprintf("Contract review %d", index)
		draft, _, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
		if err != nil {
			t.Fatal(err)
		}
		note := fmt.Sprintf("Reviewed version %d", index)
		block, _, err = s.PublishBlockWithNote(ctx, block.ID, draft, &note)
		if err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < maxStoredVersions+3; index++ {
		publishNext(index)
	}
	if note, err := s.GetVersionNote(ctx, block.ID, KindBlock, 1); err != nil || note.Note != "Pinned original contract" {
		t.Fatalf("retention lost the pinned version note: %+v, %v", note, err)
	}
	if _, err := s.GetVersionNote(ctx, block.ID, KindBlock, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unpinned old version was not pruned: %v", err)
	}
	var orphaned int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_version_notes n LEFT JOIN flow_versions v ON v.flow_id=n.flow_id AND v.revision=n.revision WHERE v.flow_id IS NULL`).Scan(&orphaned); err != nil || orphaned != 0 {
		t.Fatalf("version pruning left orphan notes: %d, %v", orphaned, err)
	}
	if err := s.DeleteFlow(ctx, parent.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"flow_library_metadata", "flow_version_notes"} {
		var remaining int
		if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE flow_id=?`, parent.ID).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("deleted parent retained %s rows: %d, %v", table, remaining, err)
		}
	}
	folders, err := s.ListLibraryFolders(ctx)
	if err != nil || len(folders) != 1 || folders[0].ID != folder.ID || folders[0].FlowCount != 0 {
		t.Fatalf("deleting a flow lost its now-empty folder: %+v, %v", folders, err)
	}
	publishNext(maxStoredVersions + 3)
	if _, err := s.GetVersionNote(ctx, block.ID, KindBlock, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("released original revision was not pruned: %v", err)
	}
	if err := s.DeleteBlock(ctx, block.ID, block.DraftRevision, block.LiveRevision); err != nil {
		t.Fatal(err)
	}
	var notes int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_version_notes`).Scan(&notes); err != nil || notes != 0 {
		t.Fatalf("block deletion retained notes: %d, %v", notes, err)
	}
	if err := s.DeleteLibraryFolder(ctx, folder.ID, folder.Revision); err != nil {
		t.Fatal(err)
	}
}
