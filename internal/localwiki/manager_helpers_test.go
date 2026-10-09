package localwiki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

// testEnv is a manager wired to a fake Kiwix server, a fake clock and a
// controllable free-space reading.
type testEnv struct {
	t       *testing.T
	kiwix   *fakeKiwix
	dir     string
	clock   *fakeClock
	free    atomic.Int64
	freeErr atomic.Bool
	manager *Manager
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	env := &testEnv{
		t:     t,
		kiwix: newFakeKiwix(t),
		dir:   filepath.Join(t.TempDir(), "wikipedia"),
		clock: &fakeClock{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)},
	}
	env.free.Store(1 << 40)
	env.manager = env.newManager()
	env.manager.Configure(env.settings())
	return env
}

func (e *testEnv) newManager() *Manager {
	m := NewManager(Deps{
		HTTPClient: e.kiwix.server.Client(),
		FreeDiskBytes: func(string) (int64, error) {
			if e.freeErr.Load() {
				return 0, errors.New("statfs failed")
			}
			return e.free.Load(), nil
		},
		Now:            e.clock.Now,
		CatalogBaseURL: e.kiwix.server.URL,
	})
	e.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	})
	return m
}

func (e *testEnv) settings() Settings {
	return Settings{Enabled: true, AgentAccess: true, Language: "de", SystemLanguage: "de", Variant: VariantNoPic, DataDir: e.dir, UpdateCheck: true}
}

// start starts the manager and waits for its first load and its first
// free-space measurement, which run in the background.
func (e *testEnv) start() {
	e.t.Helper()
	e.manager.Start(context.Background())
	select {
	case <-e.manager.firstLoad:
	case <-time.After(15 * time.Second):
		e.t.Fatal("the first load after Start did not finish")
	}
	e.waitDiskProbe(0)
}

// waitDiskProbe waits until a free-space measurement newer than after was
// recorded and returns its sequence number.
func (e *testEnv) waitDiskProbe(after uint64) uint64 {
	e.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		e.manager.mu.Lock()
		seq := e.manager.disk.seq
		e.manager.mu.Unlock()
		if seq > after {
			return seq
		}
		if time.Now().After(deadline) {
			e.t.Fatal("no free-space measurement was recorded")
		}
		time.Sleep(time.Millisecond)
	}
}

// reprobeDisk measures the free space again and waits for the result.
func (e *testEnv) reprobeDisk() {
	e.t.Helper()
	e.manager.mu.Lock()
	before := e.manager.disk.seq
	e.manager.mu.Unlock()
	e.manager.signalProbe()
	e.waitDiskProbe(before)
}

// waitFor polls the status until cond holds (15 s deadline).
func (e *testEnv) waitFor(what string, cond func(Status) bool) Status {
	e.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		status := e.manager.Status()
		if cond(status) {
			return status
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("timed out waiting for %s; last status %+v", what, status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (e *testEnv) waitIdle(state string) Status {
	e.t.Helper()
	return e.waitFor("state "+state, func(s Status) bool { return !s.OperationInProgress && s.State == state })
}

// placeEdition installs the fixture ZIM directly (file + state.json), as a
// completed install would leave it.
func placeEdition(t *testing.T, dir, language, name string) Edition {
	t.Helper()
	data := fixtureZIMBytes(t)
	parsed, ok := parseEditionName(name)
	if !ok {
		t.Fatalf("bad edition name %q", name)
	}
	sum := sha256.Sum256(data)
	edition := Edition{
		Language: language, Variant: parsed.Variant, Date: parsed.Date, Name: name, FileName: name + ".zim",
		Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), InstalledAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, edition.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeState(dir, &stateFile{Edition: &edition, LastUpdateCheck: edition.InstalledAt}); err != nil {
		t.Fatal(err)
	}
	return edition
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
