package persondetect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var retentionNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

func addRetentionEvent(t *testing.T, st *EventStore, started time.Time, ended *time.Time, size int64) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	rel, thumb := frameRelPaths(started, "cam", id, 1)
	for _, p := range []string{rel, thumb} {
		if err := writeImageAtomic(st.Root(), p, []byte("frame")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: rel, ThumbPath: thumb, Size: size, CapturedAt: started, Width: 1, Height: 1, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	if ended != nil {
		if err := st.EndEvent(ctx, id, *ended); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestApplyRetentionDeletesExpiredEventsAndKeepsOpenOnes(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	oldStart := retentionNow.Add(-20 * 24 * time.Hour)
	oldEnd := oldStart.Add(time.Minute)
	expired := addRetentionEvent(t, st, oldStart, &oldEnd, 100)
	recentStart := retentionNow.Add(-2 * 24 * time.Hour)
	recentEnd := recentStart.Add(time.Minute)
	recent := addRetentionEvent(t, st, recentStart, &recentEnd, 100)
	open := addRetentionEvent(t, st, retentionNow.Add(-30*24*time.Hour), nil, 100)
	detail, err := st.Event(ctx, expired)
	if err != nil {
		t.Fatal(err)
	}
	rel, _, err := st.Frame(ctx, detail.Frames[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := applyRetention(ctx, st, retentionNow, 14, 2048<<20)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if res.DeletedEvents != 1 || res.FreedBytes != 100 || res.TotalBytes != 200 {
		t.Fatalf("result = %+v", res)
	}
	if _, err := st.Event(ctx, expired); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("expired event survived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Root(), filepath.FromSlash(rel))); !os.IsNotExist(err) {
		t.Fatalf("expired frame file survived: %v", err)
	}
	for _, id := range []int64{recent, open} {
		if _, err := st.Event(ctx, id); err != nil {
			t.Fatalf("event %d deleted: %v", id, err)
		}
	}
}

func TestApplyRetentionCapsStorageAtNinetyPercent(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	var ended []int64
	for i := 0; i < 5; i++ {
		start := retentionNow.Add(time.Duration(i-10) * time.Hour)
		end := start.Add(time.Minute)
		ended = append(ended, addRetentionEvent(t, st, start, &end, 300))
	}
	open := addRetentionEvent(t, st, retentionNow.Add(-20*time.Hour), nil, 300)
	res, err := applyRetention(ctx, st, retentionNow, 14, 1000)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if res.DeletedEvents != 3 || res.TotalBytes != 900 {
		t.Fatalf("result = %+v, want 3 deletions down to 900 bytes", res)
	}
	for i, id := range ended {
		_, err := st.Event(ctx, id)
		if i < 3 && !errors.Is(err, ErrUnknownEvent) {
			t.Fatalf("old event %d survived", id)
		}
		if i >= 3 && err != nil {
			t.Fatalf("newer event %d deleted: %v", id, err)
		}
	}
	if _, err := st.Event(ctx, open); err != nil {
		t.Fatalf("the open event must never be deleted: %v", err)
	}
}

func TestApplyRetentionKeepsTheArmLogOfTheLastDay(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	for _, age := range []time.Duration{25 * time.Hour, 23 * time.Hour} {
		if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: retentionNow.Add(-age), ChangedBy: ActorUser}); err != nil {
			t.Fatal(err)
		}
	}
	if res, err := applyRetention(ctx, st, retentionNow, 14, 2048<<20); err != nil || res.ArmLogDeleted != 0 {
		t.Fatalf("14-day retention = %+v, %v", res, err)
	}
	if res, err := applyRetention(ctx, st, retentionNow, 1, 2048<<20); err != nil || res.ArmLogDeleted != 1 {
		t.Fatalf("1-day retention = %+v, %v", res, err)
	}
	entries, _, _, err := st.ArmLogBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(entries) != 1 || !entries[0].At.Equal(retentionNow.Add(-23*time.Hour)) {
		t.Fatalf("remaining arm log = %+v, %v", entries, err)
	}
}

// writeAgedFile writes a one-byte file at rel below root, creating its
// directories, and sets its modification time.
func writeAgedFile(t *testing.T, root, rel string, modified time.Time) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, modified, modified); err != nil {
		t.Fatal(err)
	}
	return p
}

func lstatExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestApplyRetentionRemovesStaleFrameTemporariesAndTheDirectoriesTheyLeaveEmpty(t *testing.T) {
	st := newTestEventStore(t)
	root := st.Root()
	ctx := context.Background()
	now := time.Now()
	stale, fresh := now.Add(-61*time.Minute), now.Add(-59*time.Minute)
	at := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	// Stale temporaries: the only content of their camera and date directories,
	// next to a real image, and the only content of a whole year.
	writeAgedFile(t, root, "2026/10/11/cam-a/.frame-1.tmp", stale)
	writeAgedFile(t, root, "2026/10/11/cam-b/.frame-2.tmp", stale)
	writeAgedFile(t, root, "2026/10/13/cam-d/.frame-4.tmp", stale)
	writeAgedFile(t, root, "2027/01/01/cam-e/.frame-5.tmp", stale)
	// Everything else stays: a write that may still be running, other files however
	// old, and a directory that merely carries the name of a temporary.
	writeAgedFile(t, root, "2026/10/12/cam-c/.frame-3.tmp", fresh)
	writeAgedFile(t, root, "2026/10/11/cam-b/9_1.jpg", stale)
	writeAgedFile(t, root, "2026/10/11/cam-b/.frame-6.txt", stale)
	writeAgedFile(t, root, "2026/10/11/cam-b/frame-7.tmp", stale)
	if err := os.MkdirAll(at("2026/10/14/cam-f/.frame-8.tmp"), 0o750); err != nil {
		t.Fatal(err)
	}

	res, err := applyRetention(ctx, st, now, 14, 2048<<20)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if res.StaleTemporaries != 4 {
		t.Fatalf("result = %+v, want 4 stale temporaries removed", res)
	}
	gone := []string{
		"2026/10/11/cam-a/.frame-1.tmp", "2026/10/11/cam-a", // emptied camera directory
		"2026/10/11/cam-b/.frame-2.tmp",
		"2026/10/13/cam-d/.frame-4.tmp", "2026/10/13/cam-d", "2026/10/13", // camera and date directory
		"2027", // the whole year
	}
	for _, rel := range gone {
		if lstatExists(at(rel)) {
			t.Errorf("%s survived", rel)
		}
	}
	kept := []string{
		".", "2026/10", "2026/10/11", "2026/10/11/cam-b", // directories that still hold something
		"2026/10/11/cam-b/9_1.jpg", "2026/10/11/cam-b/.frame-6.txt", "2026/10/11/cam-b/frame-7.tmp",
		"2026/10/12/cam-c/.frame-3.tmp",
		"2026/10/14/cam-f/.frame-8.tmp",
	}
	for _, rel := range kept {
		if !lstatExists(at(rel)) {
			t.Errorf("%s was removed", rel)
		}
	}
	// A second pass has nothing left to do.
	if res, err := applyRetention(ctx, st, now, 14, 2048<<20); err != nil || res.StaleTemporaries != 0 {
		t.Fatalf("second pass = %+v, %v", res, err)
	}
	// The temporary that was fresh becomes stale with time.
	if res, err := applyRetention(ctx, st, now.Add(time.Hour), 14, 2048<<20); err != nil || res.StaleTemporaries != 1 {
		t.Fatalf("pass an hour later = %+v, %v", res, err)
	}
	if lstatExists(at("2026/10/12")) {
		t.Error("the directories of the aged temporary were not pruned")
	}
}

func TestApplyRetentionStaleSweepNeverEntersOrDeletesLinks(t *testing.T) {
	st := newTestEventStore(t)
	root := st.Root()
	ctx := context.Background()
	old := time.Now().Add(-2 * time.Hour)
	outside := t.TempDir()
	victim := writeAgedFile(t, outside, ".frame-victim.tmp", old)
	nested := writeAgedFile(t, outside, "sub/.frame-nested.tmp", old)
	before := listTree(outside)

	// Links at the top, deep in the tree, and carrying the name of a temporary.
	topLink := filepath.Join(root, "2025")
	symlinkOrSkip(t, outside, topLink)
	deep := filepath.Join(root, "2026", "10", "11")
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatal(err)
	}
	cameraLink := filepath.Join(deep, "cam-link")
	symlinkOrSkip(t, outside, cameraLink)
	namedDirLink := filepath.Join(deep, ".frame-dirlink.tmp")
	symlinkOrSkip(t, outside, namedDirLink)
	links := []string{topLink, cameraLink, namedDirLink}
	fileLink := filepath.Join(deep, ".frame-filelink.tmp")
	if err := os.Symlink(victim, fileLink); err == nil { // needs a privilege on Windows
		links = append(links, fileLink)
	}
	// The sweep goes on past the links.
	realTemp := writeAgedFile(t, root, "2026/10/11/cam-real/.frame-real.tmp", old)

	// Three hours on, every file made just now is stale, the links included.
	res, err := applyRetention(ctx, st, time.Now().Add(3*time.Hour), 14, 2048<<20)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if res.StaleTemporaries != 1 {
		t.Fatalf("result = %+v, want only the real temporary removed", res)
	}
	if lstatExists(realTemp) {
		t.Error("the stale temporary next to the links survived")
	}
	for _, link := range links {
		if !lstatExists(link) {
			t.Errorf("link %s was removed", link)
		}
	}
	for _, p := range []string{victim, nested} {
		if !lstatExists(p) {
			t.Errorf("%s behind a link was deleted", p)
		}
	}
	if after := listTree(outside); strings.Join(after, "|") != strings.Join(before, "|") {
		t.Fatalf("the tree behind the links changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestApplyRetentionSweepsThroughASymlinkedRoot(t *testing.T) {
	// A root that is a link to another disk is a supported setup. On Windows a
	// junction root is neither a directory nor a symlink to Go's Lstat, which is
	// why the sweep reads directories instead of using filepath.WalkDir(root).
	dataDir := t.TempDir()
	realRoot := t.TempDir()
	_, root := DetectionsPaths(dataDir)
	if err := os.MkdirAll(filepath.Dir(root), 0o750); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, realRoot, root)
	st, err := OpenEventStore(dataDir, pdTestLogger())
	if err != nil {
		t.Fatalf("OpenEventStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	old := time.Now().Add(-2 * time.Hour)
	temp := writeAgedFile(t, realRoot, "2026/10/11/cam/.frame-1.tmp", old)
	keep := writeAgedFile(t, realRoot, "2026/10/12/cam/.frame-2.tmp", time.Now())

	res, err := applyRetention(context.Background(), st, time.Now(), 14, 2048<<20)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if res.StaleTemporaries != 1 || lstatExists(temp) || lstatExists(filepath.Join(realRoot, "2026", "10", "11")) {
		t.Fatalf("result = %+v; the stale temporary or its directories survived below a linked root", res)
	}
	if !lstatExists(keep) || !lstatExists(realRoot) {
		t.Fatal("a fresh temporary or the linked root was removed")
	}
}

func TestRemoveStaleFrameTemporariesStopsWhenTheContextEnds(t *testing.T) {
	st := newTestEventStore(t)
	temp := writeAgedFile(t, st.Root(), "2026/10/11/cam/.frame-1.tmp", time.Now().Add(-2*time.Hour))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	removed, err := st.removeStaleFrameTemporaries(ctx, time.Now())
	if !errors.Is(err, context.Canceled) || removed != 0 || !lstatExists(temp) {
		t.Fatalf("removed %d, %v; want nothing removed and context.Canceled", removed, err)
	}
}

func TestRemoveStaleFrameTemporariesReportsWhatItCouldNotRemove(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not keep a file from being deleted on Windows")
	}
	st := newTestEventStore(t)
	old := time.Now().Add(-2 * time.Hour)
	locked := writeAgedFile(t, st.Root(), "2026/10/11/locked/.frame-1.tmp", old)
	writable := writeAgedFile(t, st.Root(), "2026/10/11/open/.frame-2.tmp", old)
	lockedDir := filepath.Dir(locked)
	if err := os.Chmod(lockedDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedDir, 0o750) })
	if probe := filepath.Join(lockedDir, "probe"); os.WriteFile(probe, nil, 0o600) == nil {
		_ = os.Remove(probe)
		t.Skip("directory permissions are not enforced here (running as root?)")
	}
	removed, err := st.removeStaleFrameTemporaries(context.Background(), time.Now())
	if removed != 1 || err == nil || !strings.Contains(err.Error(), "could not be removed") {
		t.Fatalf("removed %d, %v; want the writable one removed and an error for the locked one", removed, err)
	}
	if !lstatExists(locked) || lstatExists(writable) {
		t.Fatal("the sweep did not leave exactly the locked temporary behind")
	}
}
