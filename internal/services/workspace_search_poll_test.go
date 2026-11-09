package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceSearchPollReusesUnchangedContent(t *testing.T) {
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ctx := context.Background()
	ws := cfg.Directories.WorkspaceDir
	writeWorkspaceSearchFixture(t, ws, "a.txt", "needle old\n")
	writeWorkspaceSearchFixture(t, ws, "b.txt", "unchanged\n")
	if err := svc.Rescan(ctx); err != nil {
		t.Fatal(err)
	}
	before := svc.index.files["workdir/a.txt"]
	bytesBefore := svc.Status().IndexedContentBytes
	if err := svc.TrackAccess("a.txt", "read"); err != nil {
		t.Fatal(err)
	}
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	after := svc.index.files["workdir/a.txt"]
	if after == before || &after.lines[0] != &before.lines[0] || after.hash != before.hash {
		t.Fatal("poll must reuse immutable text in a new file record")
	}
	if before.accessCount != 0 || after.accessCount != 1 || svc.Status().IndexedContentBytes != bytesBefore {
		t.Fatal("poll changed an old record or lost access/content accounting")
	}
	writeWorkspaceSearchFixture(t, ws, "a.txt", "needle new\n")
	changedAt := before.modTime.Add(time.Second)
	if err := os.Chtimes(filepath.Join(ws, "a.txt"), changedAt, changedAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(ws, "b.txt")); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceSearchFixture(t, ws, "c.txt", "added\n")
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if got := svc.index.files["workdir/a.txt"].lines[0]; got != "needle new" {
		t.Fatalf("changed content = %q", got)
	}
	if svc.index.files["workdir/b.txt"] != nil || svc.index.files["workdir/c.txt"] == nil {
		t.Fatal("poll did not discover additions and deletions")
	}
	if before.lines[0] != "needle old" {
		t.Fatal("poll mutated a previous snapshot")
	}
}

func TestWorkspaceSearchRescanDetectsPreservedMetadata(t *testing.T) {
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ctx := context.Background()
	path := filepath.Join(cfg.Directories.WorkspaceDir, "a.txt")
	writeWorkspaceSearchFixture(t, cfg.Directories.WorkspaceDir, "a.txt", "first-version\n")
	if err := svc.Rescan(ctx); err != nil {
		t.Fatal(err)
	}
	before := svc.index.files["workdir/a.txt"]
	writeWorkspaceSearchFixture(t, cfg.Directories.WorkspaceDir, "a.txt", "other-version\n")
	if err := os.Chtimes(path, before.modTime, before.modTime); err != nil {
		t.Fatal(err)
	}
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rescan(ctx); err != nil {
		t.Fatal(err)
	}
	if got := svc.index.files["workdir/a.txt"].lines[0]; got != "other-version" {
		t.Fatalf("explicit rescan retained stale text: %q", got)
	}
	if before.lines[0] != "first-version" {
		t.Fatal("explicit rescan mutated a previous snapshot")
	}
}

func TestWorkspaceSearchPollRechecksBudgetsExcludesAndBinaryContent(t *testing.T) {
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ctx := context.Background()
	ws := cfg.Directories.WorkspaceDir
	const size = 700 * 1024
	writeWorkspaceSearchFixture(t, ws, "a.txt", strings.Repeat("a", size))
	writeWorkspaceSearchFixture(t, ws, "b.txt", strings.Repeat("b", size))
	if err := svc.Rescan(ctx); err != nil {
		t.Fatal(err)
	}
	cfg.WorkspaceSearch.MaxIndexSizeMB = 1
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if status := svc.Status(); status.ContentFiles != 1 || status.IndexedContentBytes != size || svc.index.files["workdir/b.txt"].indexedContent {
		t.Fatalf("cached content exceeded the new budget: %+v", status)
	}
	cfg.WorkspaceSearch.Exclude = append(cfg.WorkspaceSearch.Exclude, "a.txt")
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if svc.index.files["workdir/a.txt"] != nil || !svc.index.files["workdir/b.txt"].indexedContent {
		t.Fatal("poll did not reapply excludes and release the content budget")
	}
	before := svc.index.files["workdir/b.txt"]
	writeWorkspaceSearchFixture(t, ws, "b.txt", "\x00"+strings.Repeat("b", size-1))
	if err := os.Chtimes(filepath.Join(ws, "b.txt"), before.modTime, before.modTime); err != nil {
		t.Fatal(err)
	}
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if svc.index.files["workdir/b.txt"] != nil || svc.Status().ContentFiles != 0 {
		t.Fatal("poll reused text after the file became binary")
	}
}

func TestWorkspaceSearchPollDropsUnreadableContent(t *testing.T) {
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ctx := context.Background()
	path := filepath.Join(cfg.Directories.WorkspaceDir, "a.txt")
	writeWorkspaceSearchFixture(t, cfg.Directories.WorkspaceDir, "a.txt", "private needle\n")
	if err := svc.Rescan(ctx); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	if f, err := os.Open(path); err == nil {
		_ = f.Close()
		t.Skip("current account can still read mode-000 files")
	} else if !os.IsPermission(err) {
		t.Fatal(err)
	}
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if svc.Status().ContentFiles != 0 {
		t.Fatal("poll retained cached content after read access was revoked")
	}
}

func TestWorkspaceSearchPrefixReportsReadErrors(t *testing.T) {
	if binary, err := looksLikeBinaryWorkspaceFile(filepath.Join(t.TempDir(), "missing.txt")); binary || !os.IsNotExist(err) {
		t.Fatalf("missing-file probe = %v, %v", binary, err)
	}
}

func TestWorkspaceSearchPollDropsSymlinkReplacement(t *testing.T) {
	svc, cfg := newWorkspaceSearchTestService(t, 2)
	ctx := context.Background()
	path := filepath.Join(cfg.Directories.WorkspaceDir, "a.txt")
	writeWorkspaceSearchFixture(t, cfg.Directories.WorkspaceDir, "a.txt", "original\n")
	if err := svc.Rescan(ctx); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := svc.rescan(ctx, false); err != nil {
		t.Fatal(err)
	}
	if svc.index.files["workdir/a.txt"] != nil {
		t.Fatal("poll retained content after replacement with a symlink")
	}
}

func BenchmarkWorkspaceSearchUnchangedPoll(b *testing.B) {
	svc, cfg := newWorkspaceSearchTestService(b, 10)
	content := strings.Repeat("A neutral workspace document for the idle indexing benchmark.\n", 8192)
	const files = 128
	for i := range files {
		writeWorkspaceSearchFixture(b, cfg.Directories.WorkspaceDir, fmt.Sprintf("document-%03d.txt", i), content)
	}
	ctx := context.Background()
	if err := svc.Rescan(ctx); err != nil {
		b.Fatal(err)
	}
	if got := svc.Status(); got.ContentFiles != files || got.IndexedContentBytes != int64(files*len(content)) {
		b.Fatalf("unexpected index: %+v", got)
	}
	b.ReportAllocs()
	b.SetBytes(int64(files * len(content)))
	b.ResetTimer()
	for b.Loop() {
		if err := svc.rescan(ctx, false); err != nil {
			b.Fatal(err)
		}
	}
}
