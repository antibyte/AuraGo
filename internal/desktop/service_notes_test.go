package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNotesAgentCreateOnlyAndConcurrentVersions(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	first, err := svc.CreateNote(ctx, "Meeting", "# Meeting\n\nOriginal", NotesDirectory, SourceUser)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		run  func() error
	}{
		{"write", func() error { return svc.WriteFile(ctx, first.Path, "changed", SourceAgent) }},
		{"binary", func() error { return svc.WriteFileBytes(ctx, first.Path, []byte("changed"), SourceAgent) }},
		{"conditional", func() error {
			_, err := svc.WriteFileBytesConditional(ctx, first.Path, []byte("changed"), SourceAgent, CheckNoteVersion(first.Version, false))
			return err
		}},
		{"move", func() error { return svc.MovePath(ctx, first.Path, NotesDirectory+"/moved.md", SourceAgent) }},
		{"move parent", func() error { return svc.MovePath(ctx, "Documents", "Elsewhere", SourceAgent) }},
		{"delete parent", func() error { return svc.DeletePath(ctx, "Documents", SourceAgent) }},
		{"copy over note", func() error { return svc.CopyPath(ctx, first.Path, NotesDirectory+"/copy.md", SourceAgent) }},
		{"sidecar", func() error { return svc.WriteFile(ctx, NotesDirectory+"/notes.meta.json", "{}", SourceAgent) }},
		{"normalized path", func() error { return svc.WriteFile(ctx, NotesDirectory+"/../Notes/meeting.md", "changed", SourceAgent) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, ErrAgentNoteMutation) {
				t.Fatalf("mutation allowed: %v", err)
			}
		})
	}
	var wg sync.WaitGroup
	paths := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			note, err := svc.CreateNote(ctx, "Meeting", "# New", NotesDirectory, SourceAgent)
			if err != nil {
				t.Error(err)
				return
			}
			paths <- note.Path
		}()
	}
	wg.Wait()
	close(paths)
	seen := map[string]bool{}
	for path := range paths {
		if seen[path] || path == first.Path {
			t.Fatal("overwrote note", path)
		}
		seen[path] = true
	}
	if len(seen) != 8 {
		t.Fatal("missing concurrent creations")
	}
	read, err := svc.ReadNote(ctx, first.Path)
	if err != nil || read.Content != first.Content {
		t.Fatal("original changed", err)
	}
	if _, err := svc.WriteFileBytesConditional(ctx, first.Path, []byte("# Updated"), SourceUser, CheckNoteVersion(first.Version, false)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.WriteFileBytesConditional(ctx, first.Path, []byte("# Stale"), SourceUser, CheckNoteVersion(first.Version, false)); !errors.Is(err, ErrNoteConflict) {
		t.Fatal("stale update", err)
	}
	if err := svc.MovePathConditional(ctx, first.Path, NotesTrashDirectory+"/meeting.md", SourceUser, CheckNoteVersion(NoteVersion([]byte("# Updated")), false)); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePath(ctx, "Trash", SourceAgent); !errors.Is(err, ErrAgentNoteMutation) {
		t.Fatal("trash parent unprotected", err)
	}
}

func TestNotesSharedSearchAndSymlinks(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	note, err := svc.CreateNote(ctx, "Bericht", "---\ntitle: Übersicht\ntags: [projekt, büro]\ncustom: preserved\n---\n# Bericht\nÄnderungen für München.\n", NotesDirectory+"/Arbeit", SourceAgent)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []NotesQuery{{Query: "MÜNCHEN änderungen"}, {Tag: "BÜRO"}, {Query: "übersicht", Folder: NotesDirectory + "/Arbeit"}} {
		result, err := svc.SearchNotes(ctx, q)
		if err != nil || len(result.Notes) != 1 || result.Notes[0].Path != note.Path {
			t.Fatalf("search %+v: %+v %v", q, result, err)
		}
	}
	if err := svc.WriteFile(ctx, note.Path, "overwrite", SourceAgent); !errors.Is(err, ErrAgentNoteMutation) {
		t.Fatal(err)
	}
	target := filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(note.Path))
	alias := filepath.Join(svc.Config().WorkspaceDir, "alias.md")
	if err := os.Symlink(target, alias); err == nil {
		if err := svc.WriteFile(ctx, "alias.md", "overwrite", SourceAgent); err == nil {
			t.Fatal("symlink bypass")
		}
	}
}
