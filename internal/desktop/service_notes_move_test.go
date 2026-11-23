package desktop

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMoveNoteWithLinksSkipsNamesThatCannotBeWikilinks(t *testing.T) {
	for _, name := range []string{"C# Tips", "a|b", "a^b", "a[b", "a]b"} {
		t.Run(name, func(t *testing.T) {
			if name == "a|b" && runtime.GOOS == "windows" {
				t.Skip("\"|\" is not a valid Windows file name")
			}
			svc := testService(t)
			ctx := context.Background()
			wiki := "see [[Old]] and [x](Old.md)\n"
			writeIndexNote(t, svc, "Documents/Notes/a.md", wiki)
			writeIndexNote(t, svc, "Documents/Notes/viaurl.md", "only [x](Old.md)\n")
			writeIndexNote(t, svc, "Documents/Notes/Old.md", "# Old")
			src, _ := svc.ReadNote(ctx, "Documents/Notes/Old.md")
			res, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/Old.md", "Documents/Notes/"+name+".md", src.Version, SourceUser)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Failed) != 1 || res.Failed[0] != "Documents/Notes/a.md" {
				t.Fatalf("failed %v updated %v", res.Failed, res.Updated)
			}
			if a, _ := svc.ReadNote(ctx, "Documents/Notes/a.md"); a.Content != wiki {
				t.Fatalf("a.md must stay untouched, got %q", a.Content)
			}
			// A note that only uses Markdown links can still be updated: the URL is escaped.
			if len(res.Updated) != 1 || res.Updated[0] != "Documents/Notes/viaurl.md" {
				t.Fatalf("updated %v", res.Updated)
			}
			v, _ := svc.ReadNote(ctx, "Documents/Notes/viaurl.md")
			index, _ := svc.NotesIndex(ctx)
			link := indexEntry(t, index, "Documents/Notes/viaurl.md").Links[0]
			if link.Path != "Documents/Notes/"+name+".md" {
				t.Fatalf("rewritten link %q does not resolve: %+v", v.Content, link)
			}
		})
	}
}

func TestMoveNoteWithLinksKeepsMovedNotesRelativeLinks(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFileBytes(ctx, "Documents/Notes/.attachments/x.png", []byte("png"), SourceUser); err != nil {
		t.Fatal(err)
	}
	writeIndexNote(t, svc, "Documents/Notes/other.md", "# other")
	writeIndexNote(t, svc, "Documents/Notes/a.md", "![](.attachments/x.png) [o](other.md#h) [me](a.md) [[a]] [web](https://x.y/z.md) `[c](other.md)`\n")
	src, _ := svc.ReadNote(ctx, "Documents/Notes/a.md")
	res, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/a.md", "Documents/Notes/sub/b.md", src.Version, SourceUser)
	if err != nil || !res.Relocated || len(res.Failed) != 0 || len(res.Updated) != 0 {
		t.Fatalf("move: %+v %v", res, err)
	}
	moved, _ := svc.ReadNote(ctx, "Documents/Notes/sub/b.md")
	want := "![](../.attachments/x.png) [o](../other.md#h) [me](b.md) [[b]] [web](https://x.y/z.md) `[c](other.md)`\n"
	if moved.Content != want {
		t.Fatalf("moved note:\n%q\nwant\n%q", moved.Content, want)
	}
	index, _ := svc.NotesIndex(ctx)
	resolved := map[string]string{}
	for _, l := range indexEntry(t, index, "Documents/Notes/sub/b.md").Links {
		resolved[l.Target] = l.Path
	}
	if resolved["../.attachments/x.png"] != "Documents/Notes/.attachments/x.png" || resolved["../other.md#h"] != "Documents/Notes/other.md" || resolved["b.md"] != "Documents/Notes/sub/b.md" {
		t.Fatalf("links no longer point at the same targets: %v", resolved)
	}

	// A rename inside one folder leaves relative links alone.
	src, _ = svc.ReadNote(ctx, "Documents/Notes/sub/b.md")
	res, err = svc.MoveNoteWithLinks(ctx, "Documents/Notes/sub/b.md", "Documents/Notes/sub/c.md", src.Version, SourceUser)
	if err != nil || !res.Relocated {
		t.Fatalf("rename: %+v %v", res, err)
	}
	renamed, _ := svc.ReadNote(ctx, "Documents/Notes/sub/c.md")
	if wantRenamed := strings.NewReplacer("[me](b.md)", "[me](c.md)", "[[b]]", "[[c]]").Replace(want); renamed.Content != wantRenamed {
		t.Fatalf("renamed note:\n%q\nwant\n%q", renamed.Content, wantRenamed)
	}
}

func TestMoveNoteWithLinksNeverGrowsPastTheNoteLimit(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	svc := testServiceWithConfig(t, Config{Enabled: true, WorkspaceDir: root, DBPath: filepath.Join(t.TempDir(), "d.db"), MaxFileSizeMB: 10})
	ctx := context.Background()
	body := strings.Repeat("[[b]]\n", (MaxNoteBytes-100)/len("[[b]]\n"))
	writeIndexNote(t, svc, "Documents/Notes/a.md", body)
	writeIndexNote(t, svc, "Documents/Notes/b.md", "# B")
	writeIndexNote(t, svc, "Documents/Notes/x/b.md", "# B2")
	src, _ := svc.ReadNote(ctx, "Documents/Notes/b.md")
	res, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/b.md", "Documents/Notes/some/longer/folder/b.md", src.Version, SourceUser)
	if err != nil || len(res.Updated) != 0 || len(res.Failed) != 1 || res.Failed[0] != "Documents/Notes/a.md" {
		t.Fatalf("move: %+v %v", res, err)
	}
	a, err := svc.ReadNote(ctx, "Documents/Notes/a.md")
	if err != nil || a.Content != body {
		t.Fatalf("a.md must stay readable and unchanged: %v", err)
	}
}

func TestNotesIndexTreatsAttachmentsFolderAsAttachmentsOnly(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	writeIndexNote(t, svc, "Documents/Notes/.attachments/readme.md", "[[b]] #tag")
	writeIndexNote(t, svc, "Documents/Notes/sub/.attachments/deep.md", "[[b]]")
	writeIndexNote(t, svc, "Documents/Notes/b.md", "# B")
	index, err := svc.NotesIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Notes) != 1 || index.Notes[0].Path != "Documents/Notes/b.md" {
		t.Fatalf("notes: %+v", index.Notes)
	}
	if !sameStrings(index.Attachments, []string{"Documents/Notes/.attachments/readme.md", "Documents/Notes/sub/.attachments/deep.md"}) {
		t.Fatalf("attachments: %v", index.Attachments)
	}
	for _, f := range index.Folders {
		if strings.Contains(f, ".attachments") {
			t.Fatalf("folders: %v", index.Folders)
		}
	}
	src, _ := svc.ReadNote(ctx, "Documents/Notes/b.md")
	res, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/b.md", "Documents/Notes/c.md", src.Version, SourceUser)
	if err != nil || len(res.Updated) != 0 || len(res.Failed) != 0 {
		t.Fatalf("move: %+v %v", res, err)
	}
	if got, _ := svc.ReadNote(ctx, "Documents/Notes/.attachments/readme.md"); got.Content != "[[b]] #tag" {
		t.Fatalf("attachment rewritten: %q", got.Content)
	}
	if search, _ := svc.SearchNotes(ctx, NotesQuery{}); search.Total != 1 {
		t.Fatalf("search sees %d notes", search.Total)
	}
}

func TestMoveNoteWithLinksReportsUnfinishedNotesWhenCancelled(t *testing.T) {
	svc := testService(t)
	base := context.Background()
	ctx, cancel := context.WithCancel(base)
	defer cancel()
	writeIndexNote(t, svc, "Documents/Notes/a1.md", "[[b]]")
	writeIndexNote(t, svc, "Documents/Notes/a2.md", "[[b]]")
	writeIndexNote(t, svc, "Documents/Notes/b.md", "# B")
	src, _ := svc.ReadNote(base, "Documents/Notes/b.md")
	svc.beforeNoteLinkWrite = func(string) { cancel() }
	res, err := svc.MoveNoteWithLinks(ctx, "Documents/Notes/b.md", "Documents/Notes/c.md", src.Version, SourceUser)
	if err != nil {
		t.Fatalf("a cancelled rewrite must not hide the move: %v", err)
	}
	if !sameStrings(res.Failed, []string{"Documents/Notes/a1.md", "Documents/Notes/a2.md"}) {
		t.Fatalf("failed %v updated %v", res.Failed, res.Updated)
	}
	if _, err := svc.ReadNote(base, "Documents/Notes/c.md"); err != nil {
		t.Fatalf("move did not happen: %v", err)
	}
}

func TestAdversarialNoteDoesNotStallReadAndIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	svc := testServiceWithConfig(t, Config{Enabled: true, WorkspaceDir: root, DBPath: filepath.Join(t.TempDir(), "d.db"), MaxFileSizeMB: 10})
	ctx := context.Background()
	for i, unit := range []string{"](", "[[", "#", "`", "![[x"} {
		path := "Documents/Notes/adv" + string(rune('a'+i)) + ".md"
		writeIndexNote(t, svc, path, strings.Repeat(unit, (MaxNoteBytes-16)/len(unit)))
	}
	start := time.Now()
	if _, err := svc.SearchNotes(ctx, NotesQuery{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.NotesIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("took %v", elapsed)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.ReadNote(cancelled, "Documents/Notes/adva.md"); err == nil {
		t.Fatal("cancelled read must fail")
	}
}
