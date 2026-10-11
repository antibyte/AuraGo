package retrogames

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func newTestSyncer(t *testing.T, f Fetcher) (*syncer, *Store, *sleepRecorder) {
	t.Helper()
	st := newTestStore(t)
	rec := &sleepRecorder{}
	sy := &syncer{fetcher: f, store: st, now: func() time.Time { return testNow }, sleep: rec.sleep, logger: discardLogger(), progress: func(SourceProgress) {}}
	return sy, st, rec
}

func TestSyncFollowsCursorAndReplacesSources(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.add(PlatformMSDOS, "", Page{Items: []Item{testItem("d1", "One", "dosbox", 1990, 5)}, Cursor: "c1"}, nil)
	f.add(PlatformMSDOS, "c1", Page{Items: []Item{testItem("d2", "Two", "dosbox", 1991, 6), testItem("d1", "One dup", "dosbox", 1990, 5), testItem("../bad", "Bad", "dosbox", 1990, 1)}}, nil)
	sy, st, rec := newTestSyncer(t, f)
	if err := sy.run(context.Background(), Sources()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := countGames(t, st, PlatformMSDOS); n != 2 {
		t.Fatalf("msdos rows = %d, want 2", n)
	}
	for _, src := range Sources()[1:] {
		if countGames(t, st, src.Key) != 1 {
			t.Fatalf("%s not synced", src.Key)
		}
	}
	if got, want := rec.count(requestPause), f.callCount()-1; got != want {
		t.Fatalf("pauses = %d, want %d", got, want)
	}
}

func TestSyncRetriesTransientErrorsAndIsolatesFailures(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.add(PlatformArcade, "", Page{}, &StatusError{Status: http.StatusServiceUnavailable, RetryAfter: 3 * time.Second})
	f.add(PlatformZX, "", Page{}, &StatusError{Status: http.StatusBadRequest, Message: "bad query"})
	sy, st, rec := newTestSyncer(t, f)
	if err := st.ReplaceSource(context.Background(), PlatformZX, []Game{testGame("old_zx", "Old", PlatformZX, 1984, 1)}, testNow.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	err := sy.run(context.Background(), Sources())
	if err == nil || !strings.Contains(err.Error(), PlatformZX) || strings.Contains(err.Error(), PlatformArcade) {
		t.Fatalf("run err = %v", err)
	}
	if rec.count(3*time.Second) != 1 {
		t.Fatalf("Retry-After not honored: %v", rec.waits)
	}
	if countGames(t, st, PlatformArcade) != 1 {
		t.Fatal("arcade should succeed after retry")
	}
	if _, ok, _ := st.Game(context.Background(), "old_zx"); !ok {
		t.Fatal("failed source must keep old rows")
	}
	states, _ := st.SyncStates(context.Background())
	if !strings.Contains(states[PlatformZX].LastError, "bad query") {
		t.Fatalf("zx state = %+v", states[PlatformZX])
	}
}

func TestSyncGivesUpAfterThreeAttempts(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = func(query, cursor string) (Page, error) {
		return Page{}, &StatusError{Status: http.StatusBadGateway}
	}
	sy, _, rec := newTestSyncer(t, f)
	src, _ := SourceByKey(PlatformWin3)
	if _, err := sy.syncSource(context.Background(), src); err == nil {
		t.Fatal("expected failure")
	}
	if f.callCount() != 3 || rec.count(retryBackoff[0]) != 1 || rec.count(retryBackoff[1]) != 1 {
		t.Fatalf("calls=%d waits=%v", f.callCount(), rec.waits)
	}
}

func TestSyncShrinkGuardKeepsPreviousData(t *testing.T) {
	f := newFakeFetcher()
	var items []Item
	for i := 0; i < 4; i++ {
		items = append(items, testItem("n"+string(rune('a'+i)), "New", "dosbox", 1990, 1))
	}
	f.add(PlatformMSDOS, "", Page{Items: items}, nil)
	sy, st, _ := newTestSyncer(t, f)
	var old []Game
	for i := 0; i < 10; i++ {
		old = append(old, testGame("o"+string(rune('a'+i)), "Old", PlatformMSDOS, 1990, 1))
	}
	if err := st.ReplaceSource(context.Background(), PlatformMSDOS, old, testNow); err != nil {
		t.Fatal(err)
	}
	src, _ := SourceByKey(PlatformMSDOS)
	if _, err := sy.syncSource(context.Background(), src); !errors.Is(err, ErrShrinkGuard) {
		t.Fatalf("err = %v, want ErrShrinkGuard", err)
	}
	if countGames(t, st, PlatformMSDOS) != 10 {
		t.Fatal("previous rows must stay")
	}
}

func TestSyncEmptySourceFails(t *testing.T) {
	sy, _, _ := newTestSyncer(t, newFakeFetcher())
	src, _ := SourceByKey(PlatformApple2)
	if _, err := sy.syncSource(context.Background(), src); !errors.Is(err, errEmptySource) {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncStopsAtPageLimit(t *testing.T) {
	f := newFakeFetcher()
	n := 0
	f.fallback = func(query, cursor string) (Page, error) {
		n++
		return Page{Items: []Item{testItem("p"+strings.Repeat("x", n%50+1), "P", "dosbox", 1990, 1)}, Cursor: "more"}, nil
	}
	sy, _, _ := newTestSyncer(t, f)
	src, _ := SourceByKey(PlatformC64)
	if _, err := sy.syncSource(context.Background(), src); err == nil || !strings.Contains(err.Error(), "pages") {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncHonorsCancellation(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	sy, _, _ := newTestSyncer(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sy.run(ctx, Sources()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
