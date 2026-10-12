package retrogames

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// shortCtx returns a context that ends quickly: long enough for a write that
// does not wait, far too short for one that waits for the write lock.
func shortCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	return ctx
}

func TestStoreWritesWaitForTheWriteLock(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	g := testGame("a", "Alpha", PlatformMSDOS, 1990, 1)

	unlock, err := st.lockWrites(ctx)
	if err != nil {
		t.Fatalf("lockWrites: %v", err)
	}
	writes := map[string]func(context.Context) error{
		"SetFavorite": func(ctx context.Context) error { return st.SetFavorite(ctx, g, true, testNow) },
		"SetBroken":   func(ctx context.Context) error { return st.SetBroken(ctx, g, true, testNow) },
		"RecordPlay":  func(ctx context.Context) error { return st.RecordPlay(ctx, g, testNow) },
		"RecordSyncFailure": func(ctx context.Context) error {
			return st.RecordSyncFailure(ctx, PlatformMSDOS, testNow, "boom")
		},
		"ReplaceSource": func(ctx context.Context) error { return st.ReplaceSource(ctx, PlatformMSDOS, []Game{g}, testNow) },
	}
	// Every write waits for the in-process lock and gives up with the
	// context; none reaches SQLite (and its 30 s busy timeout).
	for name, write := range writes {
		start := time.Now()
		if err := write(shortCtx(t)); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%s while the write lock is held: err = %v, want context.DeadlineExceeded", name, err)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("%s waited %v instead of honoring the context", name, elapsed)
		}
	}
	// Reads never take the lock.
	if _, err := st.Library(shortCtx(t)); err != nil {
		t.Fatalf("Library while the write lock is held: %v", err)
	}
	if _, err := st.SyncStates(shortCtx(t)); err != nil {
		t.Fatalf("SyncStates while the write lock is held: %v", err)
	}

	unlock()
	unlock() // idempotent: a second call must not free a lock someone else holds
	for name, write := range writes {
		if err := write(ctx); err != nil {
			t.Fatalf("%s after unlock: %v", name, err)
		}
	}
	lib, err := st.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 1 || len(lib.Broken) != 1 || len(lib.Recent) != 1 {
		t.Fatalf("library after the writes = %+v", lib)
	}

	// A canceled context is refused before waiting, even with a free lock.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := st.lockWrites(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("lockWrites(canceled) err = %v", err)
	}
	if n := len(st.writeSem); n != 0 {
		t.Fatalf("write lock held after a refused acquire: %d", n)
	}
}

// blockFirstReplace makes the first ReplaceSource of st stop inside its
// transaction (write lock held, rows written, not committed). entered closes
// once it is there; calling the returned func lets it (and every later
// ReplaceSource) commit. The func is safe to call more than once and runs at
// test end.
func blockFirstReplace(t *testing.T, st *Store) (entered <-chan struct{}, proceed func()) {
	t.Helper()
	in := make(chan struct{})
	gate := make(chan struct{})
	var enterOnce, proceedOnce sync.Once
	st.beforeReplaceCommit = func() {
		enterOnce.Do(func() { close(in) })
		<-gate
	}
	proceed = func() { proceedOnce.Do(func() { close(gate) }) }
	t.Cleanup(proceed)
	return in, proceed
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

type preparedChange struct {
	commit  func(context.Context) error
	release func()
	err     error
}

func prepareAsync(svc *Service, ctx context.Context, kind LibraryKind, id string) <-chan preparedChange {
	out := make(chan preparedChange, 1)
	go func() {
		commit, release, err := svc.PrepareLibraryChange(ctx, kind, id, true)
		out <- preparedChange{commit: commit, release: release, err: err}
	}()
	return out
}

func TestServicePrepareWaitsForTheSyncBeforeTheGate(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, _, _ := newTestService(t, f)
	ctx := context.Background()
	st, err := svc.openStore()
	if err != nil {
		t.Fatal(err)
	}
	entered, proceed := blockFirstReplace(t, st)
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, entered, "the sync to stop inside ReplaceSource")

	// The sync holds the write lock mid-transaction: preparing waits.
	got := prepareAsync(svc, ctx, LibraryFavorite, "arcade_tron")
	select {
	case p := <-got:
		t.Fatalf("prepare returned while the sync held the write lock: err=%v", p.err)
	case <-time.After(150 * time.Millisecond):
	}

	proceed()
	var p preparedChange
	select {
	case p = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("prepare did not return after the sync committed")
	}
	if p.err != nil || p.commit == nil || p.release == nil {
		t.Fatalf("prepare: commit=%v release=%v err=%v", p.commit != nil, p.release != nil, p.err)
	}
	defer p.release()

	// The lock is already held, so the write does not wait for the sync,
	// which is now waiting for this change instead.
	start := time.Now()
	if err := p.commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("commit waited %v", elapsed)
	}
	p.release()
	waitIdle(t, svc)

	lib, err := svc.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 1 || lib.Favorites[0].Identifier != "arcade_tron" {
		t.Fatalf("favorites = %+v", lib.Favorites)
	}
	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.TotalGames != len(Sources()) {
		t.Fatalf("the sync did not finish every source after the change: %+v", status)
	}
}

func TestServicePrepareHonorsCancellationWhileWaiting(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = onePerSource
	svc, _, _ := newTestService(t, f)
	st, err := svc.openStore()
	if err != nil {
		t.Fatal(err)
	}
	entered, proceed := blockFirstReplace(t, st)
	if err := svc.StartSync(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, entered, "the sync to stop inside ReplaceSource")

	// A client that leaves (or a revoked desktop run) ends the wait at once.
	cctx, cancel := context.WithCancel(context.Background())
	got := prepareAsync(svc, cctx, LibraryFavorite, "arcade_tron")
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case p := <-got:
		if !errors.Is(p.err, context.Canceled) || p.commit != nil || p.release != nil {
			t.Fatalf("canceled prepare: commit=%v release=%v err=%v", p.commit != nil, p.release != nil, p.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a canceled prepare kept waiting for the write lock")
	}

	// A deadline ends it the same way.
	dctx, dcancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer dcancel()
	if commit, release, err := svc.PrepareLibraryChange(dctx, LibraryBroken, "arcade_tron", true); !errors.Is(err, context.DeadlineExceeded) || commit != nil || release != nil {
		t.Fatalf("expired prepare: commit=%v release=%v err=%v", commit != nil, release != nil, err)
	}

	// Neither abandoned wait took the lock: once the sync is done a change
	// goes through.
	proceed()
	waitIdle(t, svc)
	ctx := context.Background()
	if err := svc.SetFavorite(ctx, "arcade_tron", true); err != nil {
		t.Fatalf("SetFavorite after the canceled waits: %v", err)
	}
	if n := len(st.writeSem); n != 0 {
		t.Fatalf("write lock still held: %d", n)
	}
}

func TestServicePrepareReleaseIsIdempotent(t *testing.T) {
	f := newFakeFetcher()
	f.fallback = func(query, cursor string) (Page, error) { return Page{}, errors.New("offline") }
	svc, _, _ := newTestService(t, f)
	ctx := context.Background()
	prepare := func(ctx context.Context, kind LibraryKind, id string) (func(context.Context) error, func(), error) {
		t.Helper()
		return svc.PrepareLibraryChange(ctx, kind, id, true)
	}
	mustPrepare := func(kind LibraryKind, id string) (func(context.Context) error, func()) {
		t.Helper()
		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		commit, release, err := prepare(pctx, kind, id)
		if err != nil {
			t.Fatalf("prepare %s: %v", id, err)
		}
		return commit, release
	}
	lockHeld := func() bool {
		t.Helper()
		_, _, err := prepare(shortCtx(t), LibraryFavorite, "arcade_tron")
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("probe prepare: %v", err)
		}
		return err != nil
	}

	// A change that is dropped without a commit frees the lock on release,
	// and a second release is a no-op.
	dropped, releaseDropped := mustPrepare(LibraryFavorite, "arcade_tron")
	releaseDropped()
	releaseDropped()
	commitB, releaseB := mustPrepare(LibraryBroken, "arcade_joust")
	defer releaseB()
	// A stale release must not free the lock the next change holds.
	releaseDropped()
	if !lockHeld() {
		t.Fatal("a repeated release freed the lock another change holds")
	}
	// A released change can no longer write.
	if err := dropped(ctx); err == nil {
		t.Fatal("commit after release must fail")
	}

	// Commit writes and frees the lock at once; the later release is a no-op
	// that must not free the next holder's lock either.
	if err := commitB(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := commitB(ctx); err == nil {
		t.Fatal("a second commit of the same change must fail")
	}
	_, releaseC := mustPrepare(LibraryFavorite, "arcade_tron")
	releaseB()
	if !lockHeld() {
		t.Fatal("release after commit freed the lock another change holds")
	}
	releaseC()
	if lockHeld() {
		t.Fatal("lock still held after every change was released")
	}

	lib, err := svc.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Favorites) != 0 || len(lib.Broken) != 1 || lib.Broken[0].Identifier != "arcade_joust" {
		t.Fatalf("library = %+v, want only the committed broken marker", lib)
	}
}

func TestServiceTeardownWhileAPrepareHoldsTheWriteLock(t *testing.T) {
	for _, mode := range []string{"disable", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			f := newFakeFetcher()
			f.fallback = onePerSource
			svc, _, dir := newTestService(t, f)
			ctx := context.Background()
			commit, release, err := svc.PrepareLibraryChange(ctx, LibraryFavorite, "arcade_tron", true)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			// The sync reaches its first write and waits for the held lock.
			if err := svc.StartSync(); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				status, err := svc.Status(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if status.Platforms[0].State == StateRunning && status.Platforms[0].Fetched > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("the sync never reached its first write: %+v", status.Platforms[0])
				}
				time.Sleep(5 * time.Millisecond)
			}

			// Neither teardown waits for the held lock.
			tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			switch mode {
			case "disable":
				svc.Configure(Settings{Enabled: false, DataDir: dir})
				if err := svc.WaitIdle(tctx); err != nil {
					t.Fatalf("WaitIdle after disabling with a held write lock: %v", err)
				}
			default:
				if err := svc.Shutdown(tctx); err != nil {
					t.Fatalf("Shutdown with a held write lock: %v", err)
				}
			}
			if err := commit(ctx); !errors.Is(err, ErrDisabled) {
				t.Fatalf("commit after %s: err = %v, want ErrDisabled", mode, err)
			}
		})
	}
}
