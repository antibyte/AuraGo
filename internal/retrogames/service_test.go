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
