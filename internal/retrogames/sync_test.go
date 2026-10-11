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
	if n := f.callsFor(PlatformZX); n != 1 {
		t.Fatalf("non-retryable zx error made %d calls, want 1", n)
	}
	if n := f.callsFor(PlatformArcade); n != 2 {
		t.Fatalf("arcade made %d calls, want 2", n)
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
		return Page{Items: []Item{testItem("p"+strings.Repeat("x", n%50+1), "P", "dosbox", 1990, 1)}, Cursor: "more" + strings.Repeat("y", n)}, nil
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

func TestSyncCancellationDuringRetryWaitIsNotRecorded(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.add(PlatformMSDOS, "", Page{}, &StatusError{Status: http.StatusServiceUnavailable})
	sy, st, _ := newTestSyncer(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sleeps := 0
	sy.sleep = func(ctx context.Context, d time.Duration) error {
		sleeps++
		cancel()
		return ctx.Err()
	}
	if err := sy.run(ctx, Sources()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if sleeps != 1 || f.callCount() != 1 {
		t.Fatalf("sleeps = %d, calls = %d, want 1 and 1", sleeps, f.callCount())
	}
	states, err := st.SyncStates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for key, state := range states {
		if state.LastError != "" {
			t.Fatalf("cancellation must not be recorded as a failure: %s = %+v", key, state)
		}
	}
	if len(states) != 0 {
		t.Fatalf("states = %+v, want none", states)
	}
}

func TestSyncAcceptShrinkReplacesSource(t *testing.T) {
	f := newFakeFetcher()
	var items []Item
	for i := 0; i < 4; i++ {
		items = append(items, testItem("n"+string(rune('a'+i)), "New", "dosbox", 1990, 1))
	}
	f.add(PlatformMSDOS, "", Page{Items: items}, nil)
	sy, st, _ := newTestSyncer(t, f)
	sy.acceptShrink = true
	var old []Game
	for i := 0; i < 10; i++ {
		old = append(old, testGame("o"+string(rune('a'+i)), "Old", PlatformMSDOS, 1990, 1))
	}
	if err := st.ReplaceSource(context.Background(), PlatformMSDOS, old, testNow); err != nil {
		t.Fatal(err)
	}
	src, _ := SourceByKey(PlatformMSDOS)
	n, err := sy.syncSource(context.Background(), src)
	if err != nil || n != 4 {
		t.Fatalf("syncSource = %d, %v; want 4, nil", n, err)
	}
	if got := countGames(t, st, PlatformMSDOS); got != 4 {
		t.Fatalf("rows = %d, want 4", got)
	}
	if _, ok, _ := st.Game(context.Background(), "oa"); ok {
		t.Fatal("old rows must be gone")
	}
	states, _ := st.SyncStates(context.Background())
	if states[PlatformMSDOS].ItemCount != 4 {
		t.Fatalf("state = %+v", states[PlatformMSDOS])
	}
}

// requireRateLimitedRun checks the outcome of a run that hit a rate limit:
// every source is reported and recorded as failed, and the error says so.
func requireRateLimitedRun(t *testing.T, err error, st *Store, failed map[string]string) {
	t.Helper()
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("run err = %v, want ErrRateLimited", err)
	}
	states, serr := st.SyncStates(context.Background())
	if serr != nil {
		t.Fatal(serr)
	}
	for _, src := range Sources() {
		if !strings.Contains(err.Error(), src.Key) {
			t.Fatalf("error %q does not list %s", err, src.Key)
		}
		if !strings.Contains(failed[src.Key], "rate limit") {
			t.Fatalf("progress for %s = %q, want a rate limit failure", src.Key, failed[src.Key])
		}
		if !strings.Contains(states[src.Key].LastError, "rate limit") {
			t.Fatalf("sync state of %s = %+v, want a rate limit failure", src.Key, states[src.Key])
		}
	}
}

func TestSyncRateLimitWithLongRetryAfterSkipsRemainingSources(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.add(PlatformMSDOS, "", Page{}, &StatusError{Status: http.StatusTooManyRequests, RetryAfter: 120 * time.Second})
	sy, st, rec := newTestSyncer(t, f)
	failed := map[string]string{}
	sy.progress = func(p SourceProgress) {
		if p.State == StateFailed {
			failed[p.Source] = p.Err
		}
	}
	if err := st.ReplaceSource(context.Background(), PlatformArcade, []Game{testGame("old_arcade", "Old", PlatformArcade, 1984, 1)}, testNow.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	err := sy.run(context.Background(), Sources())
	requireRateLimitedRun(t, err, st, failed)
	if f.callCount() != 1 || f.callsFor(PlatformMSDOS) != 1 {
		t.Fatalf("calls = %d (msdos %d), want exactly one", f.callCount(), f.callsFor(PlatformMSDOS))
	}
	if rec.count(120*time.Second) != 0 || rec.count(maxRetryWait) != 0 {
		t.Fatalf("a too long Retry-After must not be slept: %v", rec.waits)
	}
	if _, ok, _ := st.Game(context.Background(), "old_arcade"); !ok {
		t.Fatal("skipped source must keep old rows")
	}
}

func TestSyncRepeatedRateLimitSkipsRemainingSources(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	for i := 0; i < 3; i++ {
		f.add(PlatformMSDOS, "", Page{}, &StatusError{Status: http.StatusTooManyRequests})
	}
	sy, st, rec := newTestSyncer(t, f)
	failed := map[string]string{}
	sy.progress = func(p SourceProgress) {
		if p.State == StateFailed {
			failed[p.Source] = p.Err
		}
	}
	err := sy.run(context.Background(), Sources())
	requireRateLimitedRun(t, err, st, failed)
	if f.callCount() != 3 || f.callsFor(PlatformMSDOS) != 3 {
		t.Fatalf("calls = %d (msdos %d), want three", f.callCount(), f.callsFor(PlatformMSDOS))
	}
	if rec.count(retryBackoff[0]) != 1 || rec.count(retryBackoff[1]) != 1 {
		t.Fatalf("waits = %v", rec.waits)
	}
}

func TestSyncServerWaitAboveLimitIsRateLimited(t *testing.T) {
	src, _ := SourceByKey(PlatformWin3)
	t.Run("above the limit", func(t *testing.T) {
		f := newFakeFetcher()
		f.add(PlatformWin3, "", Page{}, &StatusError{Status: http.StatusServiceUnavailable, RetryAfter: maxRetryWait + time.Second})
		sy, _, rec := newTestSyncer(t, f)
		if _, err := sy.syncSource(context.Background(), src); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
		if f.callCount() != 1 || rec.count(maxRetryWait+time.Second) != 0 {
			t.Fatalf("calls = %d, waits = %v", f.callCount(), rec.waits)
		}
	})
	t.Run("exactly the limit", func(t *testing.T) {
		f := newFakeFetcher()
		f.add(PlatformWin3, "", Page{}, &StatusError{Status: http.StatusServiceUnavailable, RetryAfter: maxRetryWait})
		f.add(PlatformWin3, "", Page{Items: []Item{testItem("w1", "Win", "dosbox", 1993, 1)}}, nil)
		sy, _, rec := newTestSyncer(t, f)
		if n, err := sy.syncSource(context.Background(), src); err != nil || n != 1 {
			t.Fatalf("syncSource = %d, %v", n, err)
		}
		if rec.count(maxRetryWait) != 1 {
			t.Fatalf("waits = %v", rec.waits)
		}
	})
}

func TestSyncStoreFailureAbortsRun(t *testing.T) {
	t.Run("store already broken", func(t *testing.T) {
		f := newFakeFetcher()
		f.fallback = onePerSource
		sy, st, _ := newTestSyncer(t, f)
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
		if err := sy.run(context.Background(), Sources()); !errors.Is(err, errStore) {
			t.Fatalf("err = %v, want errStore", err)
		}
		if f.callCount() != 0 {
			t.Fatalf("calls = %d, want none before the store is known to work", f.callCount())
		}
	})
	t.Run("store breaks after the first download", func(t *testing.T) {
		f := newFakeFetcher()
		sy, st, _ := newTestSyncer(t, f)
		f.fallback = func(query, cursor string) (Page, error) {
			_ = st.Close()
			return onePerSource(query, cursor)
		}
		var failed []string
		sy.progress = func(p SourceProgress) {
			if p.State == StateFailed {
				failed = append(failed, p.Source)
			}
		}
		err := sy.run(context.Background(), Sources())
		if !errors.Is(err, errStore) {
			t.Fatalf("err = %v, want errStore", err)
		}
		if f.callCount() != 1 {
			t.Fatalf("calls = %d, want only the first source fetched", f.callCount())
		}
		if len(failed) != 1 || failed[0] != PlatformMSDOS {
			t.Fatalf("failed progress = %v, want only %s", failed, PlatformMSDOS)
		}
	})
}

func TestSyncRetryWaitReplacesRequestPause(t *testing.T) {
	f := newFakeFetcher()
	f.add(PlatformWin3, "", Page{}, &StatusError{Status: http.StatusServiceUnavailable})
	f.add(PlatformWin3, "", Page{Items: []Item{testItem("w1", "Win", "dosbox", 1993, 1)}}, nil)
	sy, _, rec := newTestSyncer(t, f)
	src, _ := SourceByKey(PlatformWin3)
	if n, err := sy.syncSource(context.Background(), src); err != nil || n != 1 {
		t.Fatalf("syncSource = %d, %v", n, err)
	}
	if rec.count(retryBackoff[0]) != 1 || rec.count(requestPause) != 0 {
		t.Fatalf("waits = %v, want the retry wait alone", rec.waits)
	}
}

func TestSyncRepeatedCursorFails(t *testing.T) {
	f := newFakeFetcher()
	f.add(PlatformMSDOS, "", Page{Items: []Item{testItem("d1", "One", "dosbox", 1990, 1)}, Cursor: "c1"}, nil)
	f.add(PlatformMSDOS, "c1", Page{Items: []Item{testItem("d2", "Two", "dosbox", 1991, 1)}, Cursor: "c1"}, nil)
	sy, _, _ := newTestSyncer(t, f)
	src, _ := SourceByKey(PlatformMSDOS)
	if _, err := sy.syncSource(context.Background(), src); err == nil || !strings.Contains(err.Error(), "repeated cursor") {
		t.Fatalf("err = %v", err)
	}
	if f.callCount() != 2 {
		t.Fatalf("calls = %d, want 2", f.callCount())
	}
}
