package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrashNotesRetainsProtectionAndSubdirectories(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	for _, original := range []string{"Documents/Notes/work/note.md", "Documents/Notes/folder"} {
		t.Run(original, func(t *testing.T) {
			file := original
			if !strings.HasSuffix(file, ".md") {
				file += "/nested/note.md"
			}
			if err := svc.WriteFile(ctx, file, "original", SourceUser); err != nil {
				t.Fatal(err)
			}
			moves, err := svc.TrashPaths(ctx, []string{original}, false, nil)
			if err != nil || len(moves) != 1 {
				t.Fatalf("trash: %v, %v", moves, err)
			}
			trashed := moves[0].Path
			if !strings.HasPrefix(trashed, "Trash/Notes/") || !strings.HasSuffix(trashed, strings.TrimPrefix(original, "Documents/Notes/")) {
				t.Fatal(trashed)
			}
			if err := svc.DeletePath(ctx, trashed, SourceAgent); !errors.Is(err, ErrAgentNoteMutation) {
				t.Fatalf("agent delete: %v", err)
			}
			if err := svc.MovePath(ctx, trashed, "Documents/stolen.md", SourceAgent); !errors.Is(err, ErrAgentNoteMutation) {
				t.Fatalf("agent move: %v", err)
			}
			restored, err := svc.TrashPaths(ctx, []string{trashed}, true, nil)
			if err != nil || len(restored) != 1 || restored[0].Path != original {
				t.Fatalf("restore: %v, %v", restored, err)
			}
			if err := svc.WriteFile(ctx, file, "agent overwrite", SourceAgent); !errors.Is(err, ErrAgentNoteMutation) {
				t.Fatalf("agent write: %v", err)
			}
		})
	}
}

func TestTrashPreflightsWholeSelection(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Desktop/keep.txt", "keep", SourceUser); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"Documents", "Documents/Notes", "Trash", "Trash/Notes", "missing.txt"} {
		if moves, err := svc.TrashPaths(ctx, []string{"Desktop/keep.txt", bad}, false, nil); err == nil || len(moves) != 0 {
			t.Fatalf("accepted %s: %v %v", bad, moves, err)
		}
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, "Desktop/keep.txt")); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.MovePathTo(ctx, "Documents", "Trash/Documents", SourceUser, nil); err == nil {
		t.Fatal("generic move bypassed notes trash")
	}
}

func TestTrashRestoreRequiresCurrentDestinationVersion(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	if err := svc.WriteFile(ctx, "Documents/Notes/a.md", "original", SourceUser); err != nil {
		t.Fatal(err)
	}
	moves, err := svc.TrashPaths(ctx, []string{"Documents/Notes/a.md"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	src := moves[0].Path
	if err := svc.WriteFile(ctx, "Documents/Notes/a.md", "newer", SourceUser); err != nil {
		t.Fatal(err)
	}
	_, err = svc.TrashPaths(ctx, []string{src}, true, nil)
	var conflict *PathConflict
	if !errors.As(err, &conflict) || conflict.Version != NoteVersion([]byte("newer")) {
		t.Fatalf("missing conflict: %v", err)
	}
	if err = svc.WriteFile(ctx, "Documents/Notes/a.md", "latest", SourceUser); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.TrashPaths(ctx, []string{src}, true, map[string]TrashResolution{src: {Version: conflict.Version}}); err == nil {
		t.Fatal("stale replacement succeeded")
	}
	copyMoves, err := svc.TrashPaths(ctx, []string{src}, true, map[string]TrashResolution{src: {Copy: true}})
	if err != nil || copyMoves[0].Path != "Documents/Notes/a (1).md" {
		t.Fatalf("copy: %v %v", copyMoves, err)
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "Documents/Notes/a.md"))
	if err != nil || string(data) != "latest" {
		t.Fatalf("destination lost: %s %v", data, err)
	}
}

func TestDesktopLegacyTransfersNeverReplaceDestinations(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	for name, data := range map[string]string{"Documents/from.txt": "source", "Documents/to.txt": "destination"} {
		if err := svc.WriteFile(ctx, name, data, SourceUser); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.MovePath(ctx, "Documents/from.txt", "Documents/to.txt", SourceUser); err == nil {
		t.Fatal("move replaced existing file")
	}
	if err := svc.CopyPath(ctx, "Documents/from.txt", "Documents/to.txt", SourceUser); err == nil {
		t.Fatal("copy replaced existing file")
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, "Documents/to.txt"))
	if err != nil || string(data) != "destination" {
		t.Fatalf("destination changed: %s %v", data, err)
	}
}
