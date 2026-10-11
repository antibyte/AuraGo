package retrogames

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *testClock) Set(t time.Time)     { c.mu.Lock(); c.now = t; c.mu.Unlock() }
func (c *testClock) Add(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

func newTestService(t *testing.T, f Fetcher) (*Service, *testClock, string) {
	t.Helper()
	clock := &testClock{now: testNow}
	dir := t.TempDir()
	svc := NewService(Deps{Logger: discardLogger(), Fetcher: f, Now: clock.Now, Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	svc.Configure(Settings{Enabled: true, AgentAccess: true, DataDir: dir})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	})
	return svc, clock, dir
}

func waitIdle(t *testing.T, svc *Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.WaitIdle(ctx); err != nil {
		t.Fatalf("WaitIdle: %v", err)
	}
}

func TestServiceDisabledRefusesEverythingWithoutTouchingDisk(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(Deps{Logger: discardLogger(), Fetcher: newFakeFetcher()})
	svc.Configure(Settings{Enabled: false, DataDir: dir})
	ctx := context.Background()
	if _, err := svc.Search(ctx, Query{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Search err = %v", err)
	}
	if err := svc.StartSync(); !errors.Is(err, ErrDisabled) {
		t.Fatalf("StartSync err = %v", err)
	}
	if _, err := svc.HallOfFame(ctx, ""); !errors.Is(err, ErrDisabled) {
		t.Fatalf("HallOfFame err = %v", err)
	}
	st, err := svc.Status(ctx)
	if err != nil || st.Enabled {
		t.Fatalf("Status = %+v err=%v", st, err)
	}
	if _, err := os.Stat(filepath.Join(dir, dbFileName)); !os.IsNotExist(err) {
		t.Fatalf("disabled service created the DB: %v", err)
	}
	if svc.AgentAccess() {
		t.Fatal("AgentAccess must be false while disabled")
	}
}

func TestServiceLazySyncOnFirstSearch(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.add(PlatformMSDOS, "", Page{Items: []Item{testItem("msdos_Prince_of_Persia_1990", "Prince of Persia", "dosbox", 1990, 99)}}, nil)
	svc, _, _ := newTestService(t, f)
	ctx := context.Background()
	if _, err := svc.Search(ctx, Query{Text: "prince"}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	res, err := svc.Search(ctx, Query{Text: "prince"})
	if err != nil || res.Total != 1 {
		t.Fatalf("after sync: %+v err=%v", res, err)
	}
	st, _ := svc.Status(ctx)
	if !st.Enabled || st.Syncing || st.TotalGames != 8 || len(st.Platforms) != 8 {
		t.Fatalf("status = %+v", st)
	}
	for _, p := range st.Platforms {
		if p.State != StateDone || p.Count != 1 || p.LastSuccessAt != testNow.Unix() {
			t.Fatalf("platform %+v", p)
		}
	}
	calls := f.callCount()
	if _, err := svc.Search(ctx, Query{}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	if f.callCount() != calls {
		t.Fatal("fresh index must not resync")
	}
}

func TestServiceSingleFlight(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.block = make(chan struct{})
	svc, _, _ := newTestService(t, f)
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartSync(); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("second StartSync err = %v", err)
	}
	st, _ := svc.Status(context.Background())
	if !st.Syncing {
		t.Fatal("status must report syncing")
	}
	close(f.block)
	waitIdle(t, svc)
}

func TestServiceAutoRetryHoldOff(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = func(query, cursor string) (Page, error) { return Page{}, errors.New("offline") }
	svc, clock, _ := newTestService(t, f)
	ctx := context.Background()
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	calls := f.callCount()
	clock.Add(10 * time.Minute)
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	if f.callCount() != calls {
		t.Fatal("automatic retry within the hold-off")
	}
	if err := svc.StartSync(); err != nil {
		t.Fatalf("manual sync must ignore the hold-off: %v", err)
	}
	waitIdle(t, svc)
	calls = f.callCount()
	clock.Add(61 * time.Minute)
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	if f.callCount() == calls {
		t.Fatal("automatic retry after the hold-off expected")
	}
}

func TestServiceGameAndLibrary(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = func(query, cursor string) (Page, error) { return Page{}, errors.New("offline") }
	svc, _, _ := newTestService(t, f)
	ctx := context.Background()
	g, err := svc.Game(ctx, "msdos_Prince_of_Persia_1990")
	if err != nil || g.Title != "Prince of Persia" {
		t.Fatalf("hall of fame fallback = %+v err=%v", g, err)
	}
	if _, err := svc.Game(ctx, "../x"); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("invalid id err = %v", err)
	}
	if _, err := svc.Game(ctx, "unknown_game"); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("unknown err = %v", err)
	}
	if err := svc.SetFavorite(ctx, "unknown_game", true); !errors.Is(err, ErrUnknownGame) {
		t.Fatalf("favorite unknown err = %v", err)
	}
	if err := svc.SetFavorite(ctx, "msdos_Prince_of_Persia_1990", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordPlay(ctx, "msdos_Prince_of_Persia_1990"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetBroken(ctx, "arcade_tron", true); err != nil {
		t.Fatal(err)
	}
	lib, err := svc.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 1 || !lib.Favorites[0].Available || len(lib.Recent) != 1 || len(lib.Broken) != 1 {
		t.Fatalf("library = %+v", lib)
	}
	if err := svc.SetFavorite(ctx, "gone_game", false); err != nil {
		t.Fatalf("removing an unknown favorite must succeed: %v", err)
	}
}

func TestServiceLibraryAvailability(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, clock, _ := newTestService(t, f)
	ctx := context.Background()
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	if err := svc.SetFavorite(ctx, "zx_default", true); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.fallback = func(query, cursor string) (Page, error) {
		src, _ := SourceByKey(PlatformZX)
		if query == src.Query() {
			return Page{Items: []Item{testItem("zx_other", "Other", "spectrum", 1984, 1)}}, nil
		}
		return onePerSource(query, cursor)
	}
	f.mu.Unlock()
	clock.Add(time.Hour)
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	lib, _ := svc.Library(ctx)
	if len(lib.Favorites) != 1 || lib.Favorites[0].Available {
		t.Fatalf("removed game must be unavailable: %+v", lib.Favorites)
	}
	if err := svc.SetFavorite(ctx, "zx_default", true); err != nil {
		t.Fatalf("re-favoriting from the snapshot must work: %v", err)
	}
}

func TestServiceAutomaticSyncOnlyFetchesStaleSources(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	zx, _ := SourceByKey(PlatformZX)
	f.add(PlatformZX, "", Page{}, errors.New("zx offline"))
	svc, clock, _ := newTestService(t, f)
	ctx := context.Background()
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
	clock.Add(61 * time.Minute)
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	f.mu.Lock()
	calls := append([]string(nil), f.calls...)
	f.mu.Unlock()
	if len(calls) != 1 || calls[0] != zx.Query()+"|" {
		t.Fatalf("automatic retry fetched %v, want only the failed zx source", calls)
	}
	st, _ := svc.Status(ctx)
	for _, p := range st.Platforms {
		if p.Count != 1 {
			t.Fatalf("platform %s count %d", p.Key, p.Count)
		}
	}
}

func TestServiceConfigureDisableCancelsSync(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	f.block = make(chan struct{})
	svc, _, dir := newTestService(t, f)
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		svc.Configure(Settings{Enabled: false, DataDir: dir})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Configure(disabled) did not cancel the sync")
	}
	if err := svc.StartSync(); !errors.Is(err, ErrDisabled) {
		t.Fatalf("StartSync after disable = %v", err)
	}
}

func TestServiceShutdownClosesForGood(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, _, dir := newTestService(t, f)
	ctx := context.Background()
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	calls := f.callCount()

	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Shutdown(sctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	refused := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrDisabled) {
			t.Fatalf("%s after Shutdown: err = %v, want ErrDisabled", name, err)
		}
	}
	_, err := svc.Search(ctx, Query{})
	refused("Search", err)
	refused("StartSync", svc.StartSync())
	_, err = svc.HallOfFame(ctx, "")
	refused("HallOfFame", err)
	_, err = svc.Game(ctx, "msdos_Prince_of_Persia_1990")
	refused("Game", err)
	_, err = svc.Library(ctx)
	refused("Library", err)
	refused("SetFavorite", svc.SetFavorite(ctx, "msdos_Prince_of_Persia_1990", true))
	refused("SetBroken", svc.SetBroken(ctx, "arcade_tron", true))
	refused("RecordPlay", svc.RecordPlay(ctx, "arcade_tron"))
	if svc.Enabled() || svc.AgentAccess() {
		t.Fatal("a shut-down service must report itself off")
	}
	if st, err := svc.Status(ctx); err != nil || st.Enabled {
		t.Fatalf("Status after Shutdown = %+v err=%v", st, err)
	}

	// Configure only records the settings; nothing is reopened or fetched.
	svc.Configure(Settings{Enabled: true, AgentAccess: true, DataDir: dir})
	refused("StartSync after Configure", svc.StartSync())
	_, err = svc.Search(ctx, Query{})
	refused("Search after Configure", err)
	if f.callCount() != calls {
		t.Fatalf("a shut-down service fetched %d more pages", f.callCount()-calls)
	}
}

func TestServiceShutdownBeforeUseTouchesNoDisk(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, _, dir := newTestService(t, f)
	other := t.TempDir()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Shutdown(sctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	svc.Configure(Settings{Enabled: true, DataDir: other})
	if _, err := svc.Search(context.Background(), Query{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Search err = %v", err)
	}
	if err := svc.StartSync(); !errors.Is(err, ErrDisabled) {
		t.Fatalf("StartSync err = %v", err)
	}
	for _, d := range []string{dir, other} {
		if _, err := os.Stat(filepath.Join(d, dbFileName)); !os.IsNotExist(err) {
			t.Fatalf("shut-down service created a DB in %s: %v", d, err)
		}
	}
	if n := f.callCount(); n != 0 {
		t.Fatalf("shut-down service fetched %d pages", n)
	}
}

func TestServiceDisableWhileRequestsHoldTheStore(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, _, dir := newTestService(t, f)
	ctx := context.Background()
	if _, err := svc.Search(ctx, Query{}); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, svc)
	held, err := svc.openStore()
	if err != nil {
		t.Fatal(err)
	}

	svc.Configure(Settings{Enabled: false, DataDir: dir})
	if _, err := svc.Library(ctx); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Library after disable = %v, want ErrDisabled", err)
	}
	if _, err := svc.Search(ctx, Query{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Search after disable = %v, want ErrDisabled", err)
	}

	// A request that fetched the store before the disable fails with the raw
	// driver error from the closed handle; storeErr must hide it.
	_, rawErr := held.Library(ctx)
	if rawErr == nil {
		t.Fatal("the store held across the disable must be closed")
	}
	if err := svc.storeErr(rawErr); !errors.Is(err, ErrDisabled) {
		t.Fatalf("storeErr(%v) = %v, want ErrDisabled", rawErr, err)
	}

	// While the service is on, store errors pass through unchanged.
	svc.Configure(Settings{Enabled: true, DataDir: dir})
	boom := errors.New("boom")
	if err := svc.storeErr(boom); err != boom {
		t.Fatalf("storeErr passed %v, want the original error", err)
	}
	if err := svc.storeErr(nil); err != nil {
		t.Fatalf("storeErr(nil) = %v", err)
	}
	res, err := svc.Search(ctx, Query{})
	if err != nil || res.Total != 8 {
		t.Fatalf("Search after re-enable: %+v err=%v", res, err)
	}
}

func TestServiceConcurrentOpensShareOneStore(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, _, _ := newTestService(t, f)
	const callers = 16
	stores := make([]*Store, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st, err := svc.openStore()
			if err != nil {
				t.Errorf("openStore %d: %v", i, err)
				return
			}
			stores[i] = st
		}(i)
	}
	wg.Wait()
	for i, st := range stores {
		if st == nil || st != stores[0] {
			t.Fatalf("caller %d got a different store", i)
		}
	}
}

func TestServiceEnsureFreshRemembersFreshness(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, clock, dir := newTestService(t, f)
	ctx := context.Background()
	freshUntil := func() time.Time {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.freshUntil
	}

	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	if got := freshUntil(); !got.IsZero() {
		t.Fatalf("freshUntil = %v while the first sync ran", got)
	}
	_, _ = svc.Search(ctx, Query{})
	if got, want := freshUntil(), testNow.Add(staleAfter); !got.Equal(want) {
		t.Fatalf("freshUntil = %v, want %v", got, want)
	}

	// While the cache holds, a source that went stale behind the service's
	// back is not looked at: the check does not even read the database.
	calls := f.callCount()
	st, err := svc.openStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE sync_state SET last_success_at = 0 WHERE source = ?`, PlatformZX); err != nil {
		t.Fatal(err)
	}
	clock.Add(time.Hour)
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	if f.callCount() != calls {
		t.Fatal("a check inside freshUntil must not start a sync")
	}

	// Past freshUntil the sources are looked at again and the sync starts.
	clock.Add(staleAfter)
	_, _ = svc.Search(ctx, Query{})
	waitIdle(t, svc)
	if f.callCount() == calls {
		t.Fatal("a check past freshUntil must start a sync")
	}
	if got := freshUntil(); !got.IsZero() {
		t.Fatalf("freshUntil = %v after a sync started, want it reset", got)
	}

	// Changing the settings drops the cache too.
	_, _ = svc.Search(ctx, Query{})
	if freshUntil().IsZero() {
		t.Fatal("freshUntil must be set again after a clean check")
	}
	svc.Configure(Settings{Enabled: true, AgentAccess: true, DataDir: dir})
	if got := freshUntil(); !got.IsZero() {
		t.Fatalf("freshUntil = %v after Configure, want it reset", got)
	}
}
