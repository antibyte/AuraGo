package localwiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A hung storage directory (an unreachable network share) must never stall
// the manager's callers: the server calls Configure on every config
// publication while it holds its config lock, Status on every poll and
// Shutdown with a deadline. These tests hold the storage I/O of a load or a
// cleanup, or the free-space measurement, in a seam and check that everything
// else answers at once.

// ioGate blocks every storage I/O section that reaches the beforeStorageIO
// seam until it is released.
type ioGate struct {
	entered chan struct{}
	blocked chan struct{}
	release func()
}

func blockStorageIO(t *testing.T, m *Manager) *ioGate {
	t.Helper()
	g := &ioGate{entered: make(chan struct{}, 16), blocked: make(chan struct{})}
	g.release = sync.OnceFunc(func() { close(g.blocked) })
	// Cleanups run last-registered first: the gate opens before the
	// manager's Shutdown cleanup runs.
	t.Cleanup(g.release)
	m.beforeStorageIO = func() {
		select {
		case g.entered <- struct{}{}:
		default:
		}
		<-g.blocked
	}
	return g
}

func (g *ioGate) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the storage I/O seam was not reached")
	}
}

// promptly runs fn and fails the test unless it returns within two seconds.
func promptly[T any](t *testing.T, what string, fn func() T) T {
	t.Helper()
	result := make(chan T, 1)
	go func() { result <- fn() }()
	select {
	case value := <-result:
		return value
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not return while the storage I/O was stuck", what)
		var zero T
		return zero
	}
}

// checkResponsive runs the calls the server makes while storage I/O is stuck.
// Install and Delete must refuse with ErrBusy instead of waiting.
func checkResponsive(t *testing.T, env *testEnv, settings Settings) Status {
	t.Helper()
	m := env.manager
	promptly(t, "Configure", func() bool { m.Configure(settings); return true })
	status := promptly(t, "Status", m.Status)
	if err := promptly(t, "Install", func() error { return m.Install(context.Background(), InstallRequest{}) }); !errors.Is(err, ErrBusy) {
		t.Fatalf("Install while the storage I/O is stuck = %v, want ErrBusy", err)
	}
	if err := promptly(t, "Delete", m.Delete); !errors.Is(err, ErrBusy) {
		t.Fatalf("Delete while the storage I/O is stuck = %v, want ErrBusy", err)
	}
	promptly(t, "Acquire", func() bool {
		_, release, ok := m.Acquire()
		release()
		return ok
	})
	promptly(t, "Settings", m.Settings)
	return status
}

// shutdownHonoursItsDeadline checks that Shutdown returns its context's error
// at once while a background goroutine is stuck in storage I/O, and that a
// later Shutdown succeeds once the I/O finished.
func shutdownHonoursItsDeadline(t *testing.T, m *Manager, gate *ioGate) {
	t.Helper()
	err := promptly(t, "Shutdown", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		return m.Shutdown(ctx)
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown while the loop is stuck = %v, want the context's error", err)
	}
	gate.release()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown after the I/O finished = %v", err)
	}
}

func TestManagerStaysResponsiveWhileTheFirstLoadIsStuck(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	m := env.manager
	gate := blockStorageIO(t, m)
	promptly(t, "Start", func() bool { m.Start(context.Background()); return true })
	gate.waitEntered(t)

	status := checkResponsive(t, env, env.settings())
	if !status.Loading || status.Readable || status.ErrorCode != CodeBusy {
		t.Fatalf("status during the stuck first load = %+v", status)
	}
	shutdownHonoursItsDeadline(t, m, gate)
}

func TestManagerStaysResponsiveWhileAReloadIsStuck(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	m := env.manager
	gate := blockStorageIO(t, m) // the loop is idle; Configure's signal orders this write before the next load
	other := filepath.Join(t.TempDir(), "elsewhere")
	moved := env.settings()
	moved.DataDir = other
	promptly(t, "Configure", func() bool { m.Configure(moved); return true })
	gate.waitEntered(t)

	// The reload is visible: loading, while the previous edition stays served.
	status := checkResponsive(t, env, moved)
	if !status.Loading || !status.Readable || status.State != StateReady || status.Edition == nil ||
		status.ErrorCode != CodeBusy || status.DataDir != other {
		t.Fatalf("status during the stuck reload = %+v", status)
	}
	if _, release, ok := m.Acquire(); !ok {
		t.Fatal("the previous edition went offline during the reload")
	} else {
		release()
	}
	shutdownHonoursItsDeadline(t, m, gate)
}

// Once a stuck reload finishes, the status stops reporting it.
func TestManagerStatusReportsAReloadUntilItFinishes(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	m := env.manager
	gate := blockStorageIO(t, m)
	moved := env.settings()
	moved.DataDir = filepath.Join(t.TempDir(), "elsewhere")
	m.Configure(moved)
	gate.waitEntered(t)
	if status := m.Status(); !status.Loading || !status.Readable {
		t.Fatalf("status during the reload = %+v", status)
	}
	gate.release()
	status := env.waitFor("the reload", func(s Status) bool { return !s.Loading })
	if status.State != StateNotInstalled || status.Readable || status.ErrorCode != "" || status.DataDir != moved.DataDir {
		t.Fatalf("status after the reload = %+v", status)
	}
}

// An Install whose own cleanup is stuck holds no lock: the other calls answer,
// and a second Install or a Delete is refused while the cleanup runs.
func TestManagerStaysResponsiveWhileAnInstallCleanupIsStuck(t *testing.T) {
	env := newTestEnv(t)
	env.start()
	m := env.manager
	gate := blockStorageIO(t, m)
	first := make(chan error, 1)
	go func() { first <- m.Install(context.Background(), InstallRequest{}) }()
	gate.waitEntered(t)

	// A cleanup of the loaded directory is no reload: nothing is loading.
	if status := checkResponsive(t, env, env.settings()); status.Loading {
		t.Fatalf("a stuck cleanup was reported as loading: %+v", status)
	}
	if err := promptly(t, "Shutdown", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return m.Shutdown(ctx)
	}); err != nil {
		t.Fatalf("Shutdown must not wait for a request stuck in storage I/O: %v", err)
	}
	gate.release()
	select {
	case <-first:
	case <-time.After(10 * time.Second):
		t.Fatal("the stuck Install did not finish after the I/O was released")
	}
}

// discardPending removes files without holding mu.
func TestManagerStaysResponsiveWhileDiscardingADownload(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	served := env.kiwix.addEdition("wikipedia_en_all_nopic_2026-10", fixtureZIMBytes(t))
	target := downloadTargetFor(served, "en")
	pending := &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served)}
	if err := writeDownload(env.dir, pending); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(env.dir, target.FileName+".part")
	if err := os.WriteFile(part, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()
	m := env.manager
	gate := blockStorageIO(t, m)
	discarded := make(chan struct{})
	go func() {
		m.discardPending(env.dir, pending)
		close(discarded)
	}()
	gate.waitEntered(t)

	checkResponsive(t, env, env.settings())
	gate.release()
	select {
	case <-discarded:
	case <-time.After(10 * time.Second):
		t.Fatal("discardPending did not finish after the I/O was released")
	}
	if fileExists(part) || fileExists(filepath.Join(env.dir, downloadFileName)) || m.Status().State != StateReady {
		t.Fatal("the released cleanup did not discard the download")
	}
}

// Status reads the free space from the background measurement and never
// waits for it; a measurement that hangs longer than the limit makes the free
// space unknown instead of showing an outdated value.
func TestManagerStatusNeverWaitsForAStuckFreeSpaceProbe(t *testing.T) {
	env := newTestEnv(t)
	var stuck atomic.Bool
	entered := make(chan struct{}, 16)
	blocked := make(chan struct{})
	release := sync.OnceFunc(func() { close(blocked) })
	m := NewManager(Deps{
		HTTPClient: env.kiwix.server.Client(),
		FreeDiskBytes: func(string) (int64, error) {
			if stuck.Load() {
				select {
				case entered <- struct{}{}:
				default:
				}
				<-blocked
			}
			return 1 << 40, nil
		},
		Now:            env.clock.Now,
		CatalogBaseURL: env.kiwix.server.URL,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	})
	t.Cleanup(release)
	env.manager = m
	m.diskProbeLimit = 50 * time.Millisecond
	m.Configure(env.settings())
	env.start()
	if free := m.Status().FreeBytes; free != 1<<40 {
		t.Fatalf("free space after the first measurement = %d", free)
	}

	stuck.Store(true)
	m.signalProbe()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the free-space measurement did not start")
	}
	time.Sleep(4 * m.diskProbeLimit)
	if free := promptly(t, "Status", m.Status).FreeBytes; free != -1 {
		t.Fatalf("free space while the measurement hangs = %d, want -1 (unknown)", free)
	}
	other := env.settings()
	other.DataDir = filepath.Join(t.TempDir(), "elsewhere")
	promptly(t, "Configure", func() bool { m.Configure(other); return true })
	if free := promptly(t, "Status", m.Status).FreeBytes; free != -1 {
		t.Fatalf("free space of a directory not measured yet = %d, want -1", free)
	}
	if err := promptly(t, "Shutdown", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return m.Shutdown(ctx)
	}); err != nil {
		t.Fatalf("Shutdown must not wait for a hanging free-space measurement: %v", err)
	}
	release()
}

// A load can run in a request goroutine (Install loads a directory changed a
// moment ago), where net/http recovers a panic. The storage-I/O mark must not
// survive the panic, or every later load, Install and Delete would answer busy
// until a restart. The hook panics once, inside the load's first storage I/O;
// the directory is changed without waking the loop, so that load runs here.
// The loop may load the directory afterwards (the release wakes it), so
// Install is retried while it reports busy.
func TestManagerReleasesTheStorageIOMarkWhenALoadPanics(t *testing.T) {
	env := newTestEnv(t)
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	other := filepath.Join(t.TempDir(), "elsewhere")
	env.start()
	m := env.manager
	var exploded atomic.Bool
	m.beforeStorageIO = func() {
		if exploded.CompareAndSwap(false, true) {
			panic("storage I/O exploded")
		}
	}
	m.mu.Lock()
	m.settings.DataDir = other
	m.mu.Unlock()
	env.dir = other

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the load did not panic")
			}
		}()
		m.tryLoadIfStale()
	}()

	deadline := time.Now().Add(10 * time.Second)
	for {
		err := m.Install(context.Background(), InstallRequest{})
		if err == nil {
			break
		}
		if !errors.Is(err, ErrBusy) || time.Now().After(deadline) {
			t.Fatalf("Install after the panic = %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status := env.waitIdle(StateReady); status.Edition == nil || status.DataDir != other || status.Loading {
		t.Fatalf("status after the install = %+v", status)
	}
	if err := m.Delete(); err != nil {
		t.Fatalf("Delete after the panic = %v", err)
	}
}

// A release runs once per mark: a second call, or a call after a later holder
// took the mark, changes nothing.
func TestStorageIOReleaseNeverEndsALaterMark(t *testing.T) {
	env := newTestEnv(t)
	env.start()
	m := env.manager
	first, ok := m.beginStorageIO(env.dir)
	if !ok {
		t.Fatal("the idle manager refused a cleanup")
	}
	if _, ok := m.beginStorageIO(env.dir); ok {
		t.Fatal("a second cleanup started while the first one runs")
	}
	first()
	second, ok := m.beginStorageIO(env.dir)
	if !ok {
		t.Fatal("the released mark was not cleared")
	}
	first() // again, now that another cleanup holds the mark
	m.mu.Lock()
	held := m.ioToken != 0
	m.mu.Unlock()
	if !held {
		t.Fatal("a stale release ended a later cleanup's mark")
	}
	second()
	second()
	if _, ok := m.beginStorageIO(env.dir); !ok {
		t.Fatal("the mark was not released")
	}
}
