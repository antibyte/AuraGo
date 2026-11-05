package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"aurago/internal/security"
)

func TestLibrarySelectiveCASAndFolderLifecycle(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	flow, err := s.CreateFlow(ctx, CreateRequest{Name: "Library test"})
	if err != nil {
		t.Fatal(err)
	}
	folder, err := s.CreateLibraryFolder(ctx, "  A folder  ")
	if err != nil || folder.Name != "A folder" {
		t.Fatalf("folder %+v %v", folder, err)
	}
	tags := make([]string, 16)
	for i := range tags {
		tags[i] = fmt.Sprintf("%02d", i) + strings.Repeat("<", 38)
	}
	meta, err := s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: 0, SetFolder: true, FolderID: &folder.ID, SetTags: true, Tags: tags})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFlowLibrary(ctx, flow.ID)
	if err != nil || len(got.Tags) != 16 || got.Revision != 1 {
		t.Fatalf("escaped tag roundtrip: %+v %v", got, err)
	}
	if err := s.DeleteLibraryFolder(ctx, folder.ID, folder.Revision); !errors.Is(err, ErrLibraryFolderNotEmpty) {
		t.Fatalf("nonempty delete: %v", err)
	}
	if _, err := s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: 0, SetTags: true, Tags: []string{}}); !errors.Is(err, ErrLibraryConflict) {
		t.Fatalf("stale write: %v", err)
	}
	meta, err = s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: meta.Revision, SetFolder: true})
	if err != nil || meta.FolderID != nil || len(meta.Tags) != 16 {
		t.Fatalf("clear folder: %+v %v", meta, err)
	}
	if err := s.DeleteLibraryFolder(ctx, folder.ID, folder.Revision); err != nil {
		t.Fatal(err)
	}
	meta, err = s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: meta.Revision, SetTags: true, Tags: []string{}})
	if err != nil || meta.Revision != 3 || len(meta.Tags) != 0 {
		t.Fatalf("clear tags: %+v %v", meta, err)
	}
	if _, err := s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: 3, SetTags: true, Tags: nil}); !errors.Is(err, ErrLibraryInvalid) {
		t.Fatalf("null tags: %v", err)
	}
	if _, err := s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: 3, SetTags: true, Tags: []string{"same", "same"}}); !errors.Is(err, ErrLibraryInvalid) {
		t.Fatalf("duplicate tags: %v", err)
	}
	if _, err := s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: 3, SetTags: true, Tags: []string{"before [redacted] after"}}); !errors.Is(err, ErrLibrarySensitive) {
		t.Fatalf("marker: %v", err)
	}
}

func TestLibraryConcurrentCAS(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	flow, err := s.CreateFlow(ctx, CreateRequest{Name: "CAS"})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, tag := range []string{"one", "two"} {
		wg.Add(1)
		go func(tag string) {
			defer wg.Done()
			<-start
			_, e := s.UpdateFlowLibrary(ctx, flow.ID, LibraryUpdate{BaseRevision: 0, SetTags: true, Tags: []string{tag}})
			results <- e
		}(tag)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrLibraryConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflict)
	}
}

func TestLibraryNotesPublishCASAndPrivacy(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Note test")
	secret := "scope_private_secret_9031"
	text := strings.Repeat("x", 110) + secret + "\nsecond line"
	pub, _, err := s.PublishWithNote(ctx, parent.ID, parent.DraftRevision, &text)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := s.UpdateFlowLibrary(ctx, parent.ID, LibraryUpdate{BaseRevision: 0, SetTags: true, Tags: []string{secret}})
	if err != nil {
		t.Fatal(err)
	}
	folder, err := s.CreateLibraryFolder(ctx, "Privacy folder")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.PublishWithNote(ctx, parent.ID, parent.DraftRevision, nil); err != nil {
		t.Fatalf("heal without note: %v", err)
	}
	if _, _, err = s.PublishWithNote(ctx, parent.ID, parent.DraftRevision, &text); err != nil {
		t.Fatalf("same note: %v", err)
	}
	different := "changed"
	if _, _, err = s.PublishWithNote(ctx, parent.ID, parent.DraftRevision, &different); !errors.Is(err, ErrVersionNoteConflict) {
		t.Fatalf("changed note on heal: %v", err)
	}
	versions, err := s.ListVersions(ctx, parent.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions %+v %v", versions, err)
	}
	cleanup := security.RegisterScopedSensitiveExact(secret)
	moved, err := s.UpdateFlowLibrary(ctx, parent.ID, LibraryUpdate{BaseRevision: initial.Revision, SetFolder: true, FolderID: &folder.ID})
	if err != nil || !moved.TagsRedacted || moved.TagsEditable {
		cleanup()
		t.Fatalf("folder move lost masked tags: %+v %v", moved, err)
	}
	versions[0].RefreshNotePrivacy()
	if !versions[0].NoteRedacted || versions[0].NoteEditable || strings.Contains(versions[0].NotePreview, secret[:8]) {
		cleanup()
		t.Fatalf("preview leaked: %+v", versions[0])
	}
	note, err := s.GetVersionNote(ctx, parent.ID, KindFlow, pub.LiveRevision)
	if err != nil || !note.Redacted || note.Editable {
		cleanup()
		t.Fatalf("note privacy: %+v %v", note, err)
	}
	f := safeFolder(LibraryFolder{Name: secret})
	f.RefreshPrivacy()
	m := safeLibrary(LibraryMetadata{Tags: []string{secret}})
	m.RefreshPrivacy()
	note.RefreshPrivacy()
	if f.Editable || !f.Redacted || m.TagsEditable || !m.TagsRedacted || note.Editable || !note.Redacted {
		cleanup()
		t.Fatal("double refresh lost redaction")
	}
	cleanup()
	versions[0].RefreshNotePrivacy()
	reread, err := s.GetFlowLibrary(ctx, parent.ID)
	if err != nil || len(reread.Tags) != 1 || reread.Tags[0] != secret {
		t.Fatalf("folder move overwrote hidden tag: %+v %v", reread, err)
	}
	if !versions[0].NoteRedacted || versions[0].NoteEditable || strings.Contains(versions[0].NotePreview, secret[:8]) {
		t.Fatalf("released scoped secret reappeared: %+v", versions[0])
	}
	if _, err = s.SetVersionNote(ctx, parent.ID, KindFlow, pub.LiveRevision, 1, "before [redacted] after"); !errors.Is(err, ErrLibrarySensitive) {
		t.Fatalf("marker accepted: %v", err)
	}
	clean, err := s.SetVersionNote(ctx, parent.ID, KindFlow, pub.LiveRevision, 1, "")
	if err != nil || clean.Revision != 2 {
		t.Fatalf("clear note: %+v %v", clean, err)
	}
	if _, err = s.SetVersionNote(ctx, parent.ID, KindFlow, pub.LiveRevision, 0, "stale"); !errors.Is(err, ErrVersionNoteConflict) {
		t.Fatalf("ABA: %v", err)
	}
	if _, err = s.GetVersionNote(ctx, block.ID, KindFlow, block.LiveRevision); !errors.Is(err, ErrWrongFlowKind) {
		t.Fatalf("kind: %v", err)
	}
}
