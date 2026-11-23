package desktop

import (
	"context"
	"strings"
	"testing"
)

func writeIndexNote(t *testing.T, svc *Service, path, content string) {
	t.Helper()
	if err := svc.WriteFile(context.Background(), path, content, SourceUser); err != nil {
		t.Fatal(err)
	}
}

func indexEntry(t *testing.T, index NotesIndexResult, path string) NoteIndexEntry {
	t.Helper()
	for _, n := range index.Notes {
		if n.Path == path {
			return n
		}
	}
	t.Fatalf("note %s missing from index", path)
	return NoteIndexEntry{}
}

func TestNotesIndexLinksTagsAndCache(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	writeIndexNote(t, svc, "Documents/Notes/a.md", "# A\n[[b]] #tag")
	writeIndexNote(t, svc, "Documents/Notes/sub/b.md", "# B")
	if err := svc.WriteFileBytes(ctx, "Documents/Notes/.attachments/x.png", []byte("png"), SourceUser); err != nil {
		t.Fatal(err)
	}
	index, err := svc.NotesIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Notes) != 2 {
		t.Fatalf("notes: %+v", index.Notes)
	}
	a := indexEntry(t, index, "Documents/Notes/a.md")
	if a.Title != "A" || len(a.Tags) != 1 || a.Tags[0] != "tag" || len(a.Links) != 1 || a.Links[0].Target != "b" || a.Links[0].Path != "Documents/Notes/sub/b.md" || a.Links[0].Kind != "wiki" {
		t.Fatalf("entry a: %+v", a)
	}
	if len(index.Attachments) != 1 || index.Attachments[0] != "Documents/Notes/.attachments/x.png" {
		t.Fatalf("attachments: %v", index.Attachments)
	}
	hasSub := false
	for _, f := range index.Folders {
		hasSub = hasSub || f == "Documents/Notes/sub"
		if strings.Contains(f, ".attachments") {
			t.Fatalf("attachments folder listed: %v", index.Folders)
		}
	}
	if !hasSub {
		t.Fatalf("folders: %v", index.Folders)
	}
	hits := svc.notesIndexCacheHits
	if _, err := svc.NotesIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if svc.notesIndexCacheHits != hits+2 {
		t.Fatalf("cache hits %d -> %d", hits, svc.notesIndexCacheHits)
	}
	writeIndexNote(t, svc, "Documents/Notes/a.md", "# A2\nchanged text here #other")
	index, _ = svc.NotesIndex(ctx)
	if a = indexEntry(t, index, "Documents/Notes/a.md"); a.Title != "A2" || len(a.Links) != 0 {
		t.Fatalf("stale cache: %+v", a)
	}
	if err := svc.DeletePath(ctx, "Documents/Notes/sub/b.md", SourceUser); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.NotesIndex(ctx); err != nil || len(svc.notesIndexCache) != 1 {
		t.Fatalf("deleted entry kept: %v %d", err, len(svc.notesIndexCache))
	}
}

func TestSearchNotesInlineTag(t *testing.T) {
	svc := testService(t)
	writeIndexNote(t, svc, "Documents/Notes/a.md", "# A\n[[b]] #tag")
	writeIndexNote(t, svc, "Documents/Notes/c.md", "# C\nno tag")
	result, err := svc.SearchNotes(context.Background(), NotesQuery{Tag: "tag"})
	if err != nil || result.Total != 1 || result.Notes[0].Path != "Documents/Notes/a.md" {
		t.Fatalf("inline tag search: %+v %v", result, err)
	}
}

func TestMoveNoteWithLinksRewritesReferences(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	writeIndexNote(t, svc, "Documents/Notes/a.md", "# A\n[[b]] and [[b#Part|shown]] and [md](sub/b.md#x) and [[Alias]]\n```\n[[b]]\n```\n")
	writeIndexNote(t, svc, "Documents/Notes/sub/b.md", "---\naliases: [Alias]\n---\n# B")
	writeIndexNote(t, svc, "Documents/Notes/sub/inner.md", "[up](b.md) [[b.md]]")
	writeIndexNote(t, svc, "Documents/Notes/other.md", "nothing")
	source, err := svc.ReadNote(ctx, "Documents/Notes/sub/b.md")
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/sub/b.md", "Documents/Notes/c d.md", source.Version, SourceUser)
	if err != nil || len(result.Updated) != 2 || len(result.Failed) != 0 {
		t.Fatalf("move: %+v %v", result, err)
	}
	a, _ := svc.ReadNote(ctx, "Documents/Notes/a.md")
	want := "# A\n[[c d]] and [[c d#Part|shown]] and [md](c%20d.md#x) and [[Alias]]\n```\n[[b]]\n```\n"
	if a.Content != want {
		t.Fatalf("a.md:\n%q\nwant\n%q", a.Content, want)
	}
	inner, _ := svc.ReadNote(ctx, "Documents/Notes/sub/inner.md")
	if inner.Content != "[up](../c%20d.md) [[c d.md]]" {
		t.Fatalf("inner.md: %q", inner.Content)
	}
	if _, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/c d.md", "Documents/Notes/e.md", "\"stale\"", SourceUser); err == nil {
		t.Fatal("stale version must fail before any rewrite")
	}
	if a2, _ := svc.ReadNote(ctx, "Documents/Notes/a.md"); a2.Content != want {
		t.Fatal("failed move rewrote links")
	}
	if _, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/c d.md", "Documents/Notes/e.md", source.Version, SourceAgent); err == nil {
		t.Fatal("agents cannot move notes")
	}
}

func TestMoveNoteWithLinksReportsConcurrentChange(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	writeIndexNote(t, svc, "Documents/Notes/a.md", "[[b]]")
	writeIndexNote(t, svc, "Documents/Notes/b.md", "# B")
	source, _ := svc.ReadNote(ctx, "Documents/Notes/b.md")
	svc.beforeNoteLinkWrite = func(path string) {
		if err := svc.WriteFile(ctx, path, "[[b]] edited meanwhile", SourceUser); err != nil {
			t.Error(err)
		}
	}
	result, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/b.md", "Documents/Notes/c.md", source.Version, SourceUser)
	if err != nil || len(result.Updated) != 0 || len(result.Failed) != 1 || result.Failed[0] != "Documents/Notes/a.md" {
		t.Fatalf("concurrent change: %+v %v", result, err)
	}
	if a, _ := svc.ReadNote(ctx, "Documents/Notes/a.md"); a.Content != "[[b]] edited meanwhile" {
		t.Fatalf("concurrent edit lost: %q", a.Content)
	}
}
