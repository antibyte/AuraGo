package persondetect

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

// setRetentionProgress replaces the retention test hook until the test ends.
func setRetentionProgress(t *testing.T, fn func(step, detail string)) {
	t.Helper()
	previous := retentionProgress
	t.Cleanup(func() { retentionProgress = previous })
	retentionProgress = fn
}

// makeUndeletable keeps the file p from being deleted until release is called:
// Windows holds it open (Go opens files without FILE_SHARE_DELETE, so the delete
// fails with a sharing violation), elsewhere its directory loses the write
// permission. The test is skipped where neither takes effect (root).
func makeUndeletable(t *testing.T, p string) (release func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		var once sync.Once
		release = func() { once.Do(func() { _ = f.Close() }) }
		t.Cleanup(release)
		if err := os.Remove(p); err == nil {
			t.Skip("an open file can be deleted here")
		}
		return release
	}
	dir := filepath.Dir(p)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	release = func() { _ = os.Chmod(dir, 0o750) }
	t.Cleanup(release)
	if probe := filepath.Join(dir, "probe"); os.WriteFile(probe, nil, 0o600) == nil {
		_ = os.Remove(probe)
		t.Skip("directory permissions are not enforced here (running as root?)")
	}
	return release
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
	if res.StaleTemporaries != 4 || res.StaleTemporaryFailures != 0 || res.StaleTemporaryError != nil {
		t.Fatalf("result = %+v, want 4 stale temporaries removed and no failure", res)
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
	sweep, err := st.removeStaleFrameTemporaries(ctx, now.Add(time.Hour))
	if err != nil || sweep.Removed != 1 || sweep.Failures != 0 {
		t.Fatalf("sweep an hour later = %+v, %v", sweep, err)
	}
	if lstatExists(at("2026/10/12")) {
		t.Error("the directories of the aged temporary were not pruned")
	}
}

func TestApplyRetentionSweepsOnTheWallClockNotTheServiceClock(t *testing.T) {
	// File times come from the file system's clock. A service clock that is ahead
	// of it (a forward step, a test harness) must not make the temporaries of
	// writes that are running right now look stale.
	st := newTestEventStore(t)
	root := st.Root()
	ctx := context.Background()
	wall := time.Now()
	inFlight := writeAgedFile(t, root, "2026/10/12/cam/.frame-live.tmp", wall.Add(-time.Minute))
	crashed := writeAgedFile(t, root, "2026/10/12/cam/.frame-crashed.tmp", wall.Add(-2*time.Hour))

	res, err := applyRetention(ctx, st, wall.Add(72*time.Hour), 14, 2048<<20)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	if res.StaleTemporaries != 1 || !lstatExists(inFlight) || lstatExists(crashed) {
		t.Fatalf("service clock 72 h ahead: result = %+v, in-flight exists %v, crashed exists %v", res, lstatExists(inFlight), lstatExists(crashed))
	}

	// And a service clock far behind does not keep an old temporary alive.
	crashedAgain := writeAgedFile(t, root, "2026/10/12/cam/.frame-crashed-again.tmp", wall.Add(-3*time.Hour))
	res, err = applyRetention(ctx, st, wall.Add(-72*time.Hour), 14, 2048<<20)
	if err != nil || res.StaleTemporaries != 1 || lstatExists(crashedAgain) || !lstatExists(inFlight) {
		t.Fatalf("service clock 72 h behind: result = %+v, %v", res, err)
	}
}

func TestApplyRetentionSweepFailureDoesNotFailThePass(t *testing.T) {
	// The caller applies TotalBytes only for a pass without an error, so one
	// temporary that cannot be deleted (antivirus, wrong owner, a read-only
	// remount) must not take the byte count of the event steps with it.
	st := newTestEventStore(t)
	ctx := context.Background()
	now := time.Now()
	oldStart := now.Add(-20 * 24 * time.Hour)
	oldEnd := oldStart.Add(time.Minute)
	expired := addRetentionEvent(t, st, oldStart, &oldEnd, 100)
	recentStart := now.Add(-time.Hour)
	recentEnd := recentStart.Add(time.Minute)
	recent := addRetentionEvent(t, st, recentStart, &recentEnd, 100)
	temp := writeAgedFile(t, st.Root(), "2031/03/11/locked/.frame-1.tmp", now.Add(-2*time.Hour))
	release := makeUndeletable(t, temp)

	res, err := applyRetention(ctx, st, now, 14, 2048<<20)
	if err != nil {
		t.Fatalf("a temporary that cannot be deleted failed the pass: %v", err)
	}
	if res.DeletedEvents != 1 || res.FreedBytes != 100 || res.TotalBytes != 100 {
		t.Fatalf("event results = %+v, want 1 deletion freeing 100 bytes and 100 bytes left", res)
	}
	if res.StaleTemporaries != 0 || res.StaleTemporaryFailures != 1 || res.StaleTemporaryError == nil {
		t.Fatalf("sweep results = %+v, want one failure and its error", res)
	}
	cause := res.StaleTemporaryError
	var pathErr *fs.PathError
	if !strings.Contains(cause.Error(), temp) || !errors.As(cause, &pathErr) {
		t.Fatalf("failure %q does not name the path and wrap the cause", cause)
	}
	if runtime.GOOS != "windows" && !errors.Is(cause, fs.ErrPermission) {
		t.Fatalf("failure %q does not wrap the permission error", cause)
	}
	t.Logf("reported cause: %v", cause)
	if _, err := st.Event(ctx, expired); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("the expired event survived: %v", err)
	}
	if _, err := st.Event(ctx, recent); err != nil {
		t.Fatalf("the recent event was deleted: %v", err)
	}

	// The failure is not remembered: once the file can be deleted, the next pass does.
	release()
	res, err = applyRetention(ctx, st, now, 14, 2048<<20)
	if err != nil || res.StaleTemporaries != 1 || res.StaleTemporaryFailures != 0 || res.StaleTemporaryError != nil || lstatExists(temp) {
		t.Fatalf("pass after the release = %+v, %v; temporary exists %v", res, err, lstatExists(temp))
	}
}

func TestApplyRetentionReadsTheTotalAgainAfterTheSizeStep(t *testing.T) {
	// Frames are added to open events while retention runs. The total the size
	// step computes by subtraction cannot know about them.
	st := newTestEventStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		start := retentionNow.Add(time.Duration(i-10) * time.Hour)
		end := start.Add(time.Minute)
		addRetentionEvent(t, st, start, &end, 300)
	}
	open := addRetentionEvent(t, st, retentionNow.Add(-20*time.Hour), nil, 300)
	var hooked bool
	setRetentionProgress(t, func(step, _ string) {
		if step != retentionBeforeTotal {
			return
		}
		hooked = true
		if _, err := st.AddFrame(ctx, open, StoredFrame{Seq: 2, Path: "x/2.jpg", ThumbPath: "x/2_thumb.jpg", Size: 50, CapturedAt: retentionNow, Width: 1, Height: 1}); err != nil {
			t.Errorf("AddFrame: %v", err)
		}
	})
	res, err := applyRetention(ctx, st, retentionNow, 14, 1000)
	if err != nil {
		t.Fatalf("applyRetention: %v", err)
	}
	stored, err := st.TotalBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 4 x 300 = 1200 > 1000: one deletion down to 900, plus the 50 bytes added meanwhile.
	if !hooked || res.DeletedEvents != 1 || stored != 950 || res.TotalBytes != stored {
		t.Fatalf("hooked %v, result = %+v, stored %d; want the stored 950 bytes", hooked, res, stored)
	}
}

func TestApplyRetentionStopsOnCancellationWithPartialResults(t *testing.T) {
	cancelAfter := func(cancel context.CancelFunc, step string, n int) func(step, detail string) {
		seen := 0
		return func(got, _ string) {
			if got == step {
				if seen++; seen == n {
					cancel()
				}
			}
		}
	}
	t.Run("age step", func(t *testing.T) {
		st := newTestEventStore(t)
		var ids []int64
		for i := 0; i < 5; i++ {
			start := retentionNow.Add(-time.Duration(30-i) * 24 * time.Hour)
			end := start.Add(time.Minute)
			ids = append(ids, addRetentionEvent(t, st, start, &end, 100))
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setRetentionProgress(t, cancelAfter(cancel, retentionEventRemoved, 2))
		res, err := applyRetention(ctx, st, retentionNow, 14, 2048<<20)
		if !errors.Is(err, context.Canceled) || res.DeletedEvents != 2 || res.FreedBytes != 200 {
			t.Fatalf("result = %+v, %v; want 2 deletions and context.Canceled", res, err)
		}
		for i, id := range ids {
			_, err := st.Event(context.Background(), id)
			if i < 2 && !errors.Is(err, ErrUnknownEvent) || i >= 2 && err != nil {
				t.Fatalf("event %d after the cancelled pass: %v", i, err)
			}
		}
	})
	t.Run("size step", func(t *testing.T) {
		st := newTestEventStore(t)
		for i := 0; i < 5; i++ {
			start := retentionNow.Add(time.Duration(i-10) * time.Hour)
			end := start.Add(time.Minute)
			addRetentionEvent(t, st, start, &end, 300)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setRetentionProgress(t, cancelAfter(cancel, retentionEventRemoved, 1))
		res, err := applyRetention(ctx, st, retentionNow, 14, 1000)
		if !errors.Is(err, context.Canceled) || res.DeletedEvents != 1 || res.FreedBytes != 300 {
			t.Fatalf("result = %+v, %v; want 1 deletion and context.Canceled", res, err)
		}
	})
	t.Run("sweep", func(t *testing.T) {
		// The events are done when the walk is cut short and stay counted.
		st := newTestEventStore(t)
		now := time.Now()
		oldStart := now.Add(-20 * 24 * time.Hour)
		oldEnd := oldStart.Add(time.Minute)
		addRetentionEvent(t, st, oldStart, &oldEnd, 100)
		recentStart := now.Add(-time.Hour)
		recentEnd := recentStart.Add(time.Minute)
		addRetentionEvent(t, st, recentStart, &recentEnd, 100)
		first := writeAgedFile(t, st.Root(), "2031/03/11/cam/.frame-1.tmp", now.Add(-2*time.Hour))
		second := writeAgedFile(t, st.Root(), "2031/03/12/cam/.frame-2.tmp", now.Add(-2*time.Hour))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setRetentionProgress(t, func(step, detail string) {
			if step == retentionSweepDirectory && strings.HasSuffix(filepath.ToSlash(detail), "/2031/03/12") {
				cancel()
			}
		})
		res, err := applyRetention(ctx, st, now, 14, 2048<<20)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if res.DeletedEvents != 1 || res.TotalBytes != 100 || res.StaleTemporaries != 1 {
			t.Fatalf("result = %+v, want the event results filled and the one temporary removed before the cut", res)
		}
		if lstatExists(first) || !lstatExists(second) {
			t.Fatalf("first exists %v, second exists %v; the walk should have stopped between them", lstatExists(first), lstatExists(second))
		}
	})
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
	} else {
		t.Logf("file symlinks are not available here (%v); a link to a file named like a temporary is not exercised", err)
	}
	// The sweep goes on past the links.
	realTemp := writeAgedFile(t, root, "2026/10/11/cam-real/.frame-real.tmp", old)

	// Three hours on, every file made just now is stale, the links included.
	sweep, err := st.removeStaleFrameTemporaries(ctx, time.Now().Add(3*time.Hour))
	if err != nil {
		t.Fatalf("removeStaleFrameTemporaries: %v", err)
	}
	if sweep.Removed != 1 || sweep.Failures != 0 {
		t.Fatalf("sweep = %+v, want only the real temporary removed", sweep)
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
	sweep, err := st.removeStaleFrameTemporaries(ctx, time.Now())
	if !errors.Is(err, context.Canceled) || sweep.Removed != 0 || !lstatExists(temp) {
		t.Fatalf("sweep = %+v, %v; want nothing removed and context.Canceled", sweep, err)
	}
}

// modeInfo is an fs.FileInfo that has nothing but a mode.
type modeInfo struct{ mode fs.FileMode }

func (m modeInfo) Name() string       { return "entry" }
func (m modeInfo) Size() int64        { return 0 }
func (m modeInfo) Mode() fs.FileMode  { return m.mode }
func (m modeInfo) ModTime() time.Time { return time.Time{} }
func (m modeInfo) IsDir() bool        { return m.mode.IsDir() }
func (m modeInfo) Sys() any           { return nil }

func TestSweepClassifiesEntriesLikeCheckElement(t *testing.T) {
	// Go reports a Windows junction as ModeIrregular without ModeDir, a
	// symlink as ModeSymlink, and a reparse-point directory that is no link
	// (OneDrive placeholder, tiered storage) as ModeDir|ModeIrregular, which
	// checkElement lets through. Only the links stay out of the sweep.
	cases := []struct {
		name       string
		mode       fs.FileMode
		link       bool
		descends   bool
		removeable bool
	}{
		{"directory", fs.ModeDir | 0o750, false, true, false},
		{"placeholder directory", fs.ModeDir | fs.ModeIrregular | 0o750, false, true, false},
		{"directory symlink", fs.ModeSymlink | 0o777, true, false, false},
		{"symlink that reports a directory bit", fs.ModeSymlink | fs.ModeDir, true, false, false},
		{"junction", fs.ModeIrregular, true, false, false},
		{"regular file", 0o640, false, false, true},
		{"file symlink", fs.ModeSymlink | 0o777, true, false, false},
		{"irregular file", fs.ModeIrregular | 0o640, true, false, false},
		{"named pipe", fs.ModeNamedPipe | 0o640, false, false, false},
		{"socket", fs.ModeSocket | 0o640, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := modeInfo{c.mode}
			if got := isLinkLike(c.mode); got != c.link {
				t.Errorf("isLinkLike = %v, want %v", got, c.link)
			}
			if got := sweepDescendsInto("p", info); got != c.descends {
				t.Errorf("sweepDescendsInto = %v, want %v", got, c.descends)
			}
			if got := sweepRemovableType("p", info); got != c.removeable {
				t.Errorf("sweepRemovableType = %v, want %v", got, c.removeable)
			}
		})
	}
}

func TestRemoveFileErrNamesWhyAFileStayed(t *testing.T) {
	st := newTestEventStore(t)
	root := st.Root()
	if err := writeImageAtomic(root, "2026/10/11/cam/1_1.jpg", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := st.removeFileErr("2026/10/11/cam/1_1.jpg"); err != nil {
		t.Fatalf("removing a file: %v", err)
	}
	if lstatExists(filepath.Join(root, "2026")) {
		t.Error("the emptied directories were not pruned")
	}
	if err := st.removeFileErr("2026/10/11/cam/1_1.jpg"); err != nil {
		t.Fatalf("a file that is already gone is no error: %v", err)
	}
	if err := st.removeFileErr("../escape.jpg"); err == nil {
		t.Fatal("a path outside the root was accepted")
	}
	outside := t.TempDir()
	victim := writeAgedFile(t, outside, "cam/1_1.jpg", time.Now())
	if err := os.MkdirAll(filepath.Join(root, "2026"), 0o750); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, filepath.Join(root, "2026", "10"))
	if err := st.removeFileErr("2026/10/cam/1_1.jpg"); err == nil || !lstatExists(victim) {
		t.Fatalf("a path through a link: err = %v, victim exists %v; want an error and the file kept", err, lstatExists(victim))
	}
	if runtime.GOOS != "windows" {
		locked := writeAgedFile(t, root, "2027/01/01/cam/2_1.jpg", time.Now())
		dir := filepath.Dir(locked)
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })
		if probe := filepath.Join(dir, "probe"); os.WriteFile(probe, nil, 0o600) == nil {
			_ = os.Remove(probe)
			return // permissions are not enforced here (root)
		}
		if err := st.removeFileErr("2027/01/01/cam/2_1.jpg"); !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("remove below a read-only directory: %v, want a permission error", err)
		}
		st.removeFile("2027/01/01/cam/2_1.jpg") // the old entry point keeps ignoring it
		if !lstatExists(locked) {
			t.Fatal("removeFile deleted a file in a read-only directory")
		}
	}
}
