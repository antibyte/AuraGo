package localwiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/zim"
)

func (e *testEnv) disabledSettings() Settings {
	settings := e.settings()
	settings.Enabled = false
	return settings
}

// libraryOpen reports whether the manager holds an open edition.
func (e *testEnv) libraryOpen() bool {
	e.manager.mu.Lock()
	defer e.manager.mu.Unlock()
	return e.manager.lib != nil
}

func (e *testEnv) diskSeq() uint64 {
	e.manager.mu.Lock()
	defer e.manager.mu.Unlock()
	return e.manager.disk.seq
}

// A disabled integration loads the storage directory's state (the config page
// shows the edition and offers Delete) but never opens the edition file and
// never measures the free space. Switching it on opens the edition and
// measures again.
func TestManagerDisabledLoadsTheStateButNotTheEdition(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.manager.Configure(env.disabledSettings())
	env.manager.diskProbeEvery = 5 * time.Millisecond
	env.start()

	status := env.manager.Status()
	if status.Loading || status.State != StateReady || status.Readable || status.Fulltext || status.Edition == nil ||
		status.Edition.Name != edition.Name || status.ErrorCode != CodeDisabled || status.Recommendation != Recommendation(CodeDisabled) {
		t.Fatalf("status while disabled = %+v", status)
	}
	if env.libraryOpen() {
		t.Fatal("the edition was opened while the integration is disabled")
	}
	if _, release, ok := env.manager.Acquire(); ok {
		release()
		t.Fatal("Acquire succeeded while the integration is disabled")
	}
	env.manager.signalProbe()
	time.Sleep(50 * time.Millisecond) // ten probe ticks
	if seq := env.diskSeq(); seq != 0 {
		t.Fatalf("the disk was measured %d times while the integration is disabled", seq)
	}

	env.manager.Configure(env.settings())
	status = env.waitFor("the edition opened after switching on", func(s Status) bool { return s.Readable && !s.Loading })
	if status.State != StateReady || !status.Fulltext || status.ErrorCode != "" {
		t.Fatalf("status after switching on = %+v", status)
	}
	env.waitDiskProbe(0)
	lib, release, ok := env.manager.Acquire()
	if !ok || lib.Edition().Name != edition.Name {
		t.Fatalf("Acquire after switching on = %v, %v", lib, ok)
	}
	release()
	if len(env.kiwix.requestLog()) != 0 {
		t.Fatalf("switching on contacted the network: %v", env.kiwix.requestLog())
	}
}

// Switching the integration off takes the edition out of service at once:
// Acquire fails, a reader that holds it finishes its work, and the file is
// closed after its release (so it could be deleted by hand, also on Windows).
// Switching it on again opens it again.
func TestManagerSwitchedOffClosesTheEditionAfterItsReaders(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	held, release, ok := env.manager.Acquire()
	if !ok {
		t.Fatal("Acquire before switching off failed")
	}
	main, err := held.Main()
	if err != nil {
		t.Fatalf("Main: %v", err)
	}

	env.manager.Configure(env.disabledSettings())
	if _, extra, ok := env.manager.Acquire(); ok {
		extra()
		t.Fatal("Acquire succeeded after switching off")
	}
	if status := env.manager.Status(); status.Readable || status.State != StateReady || status.ErrorCode != CodeDisabled || status.Edition == nil {
		t.Fatalf("status after switching off = %+v", status)
	}
	item, err := held.Content(main.Path)
	if err != nil {
		t.Fatalf("a reader holding the edition could not finish: %v", err)
	}
	if _, err := item.Reader.Read(make([]byte, 16)); err != nil {
		t.Fatalf("reading the held article: %v", err)
	}
	release()
	if _, err := held.Content(main.Path); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("Content after the last release = %v, want zim.ErrClosed (the edition must be closed)", err)
	}

	env.manager.Configure(env.settings())
	env.waitFor("the edition reopened", func(s Status) bool { return s.Readable && !s.Loading && s.ErrorCode == "" })
	lib, again, ok := env.manager.Acquire()
	if !ok || lib.Edition().Name != edition.Name {
		t.Fatalf("Acquire after switching on again = %v, %v", lib, ok)
	}
	if _, err := lib.Content(main.Path); err != nil {
		t.Fatalf("Content after switching on again: %v", err)
	}
	again()
}

// The edition file of a disabled integration is not held open: on Windows it
// could not be deleted otherwise. Delete works while disabled too.
func TestManagerDisabledLeavesTheEditionFileFree(t *testing.T) {
	t.Run("by hand", func(t *testing.T) {
		env := newTestEnv(t)
		edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		env.manager.Configure(env.disabledSettings())
		env.start()
		if err := os.Remove(filepath.Join(env.dir, edition.FileName)); err != nil {
			t.Fatalf("the edition file of a disabled integration cannot be deleted: %v", err)
		}
	})
	t.Run("Delete", func(t *testing.T) {
		env := newTestEnv(t)
		edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		env.manager.Configure(env.disabledSettings())
		env.start()
		if err := env.manager.Delete(); err != nil {
			t.Fatalf("Delete while disabled = %v", err)
		}
		if fileExists(filepath.Join(env.dir, edition.FileName)) || fileExists(filepath.Join(env.dir, stateFileName)) {
			t.Fatal("Delete while disabled left the edition behind")
		}
		if status := env.manager.Status(); status.State != StateNotInstalled || status.Edition != nil || status.ErrorCode != CodeDisabled {
			t.Fatalf("status after Delete = %+v", status)
		}
		// Nothing is left to open once the integration is switched on.
		env.manager.Configure(env.settings())
		env.waitFor("idle after switching on", func(s Status) bool { return !s.Loading && s.State == StateNotInstalled && s.ErrorCode == "" })
	})
}

// A download that finishes after the integration was switched off records the
// edition but does not serve it until the integration is switched on again.
func TestManagerPublishAfterSwitchingOffKeepsTheEditionClosed(t *testing.T) {
	env := newTestEnv(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		served.setMode(mirror, "slow")
	}
	defer served.unblock()
	env.start()
	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	env.manager.Configure(env.disabledSettings())
	served.unblock()
	status := env.waitFor("the download finished", func(s Status) bool { return !s.OperationInProgress && s.Edition != nil })
	if status.State != StateReady || status.Readable || status.ErrorCode != CodeDisabled || status.Edition.Name != served.name {
		t.Fatalf("status after publishing while disabled = %+v", status)
	}
	if env.libraryOpen() {
		t.Fatal("an edition published while disabled is open")
	}
	if _, release, ok := env.manager.Acquire(); ok {
		release()
		t.Fatal("Acquire succeeded while disabled")
	}
	env.manager.Configure(env.settings())
	env.waitFor("the published edition opened", func(s Status) bool { return s.Readable && !s.Loading && s.ErrorCode == "" })
	lib, release, ok := env.manager.Acquire()
	if !ok || lib.Edition().Name != served.name {
		t.Fatalf("Acquire after switching on = %v, %v", lib, ok)
	}
	release()
}

// An edition that cannot be opened when the integration is switched on is
// reported once as zim_unreadable; the loop does not retry it endlessly.
func TestManagerSwitchingOnReportsAnUnreadableEdition(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.manager.Configure(env.disabledSettings())
	env.start()
	if status := env.manager.Status(); status.State != StateReady || status.ErrorCode != CodeDisabled {
		t.Fatalf("status while disabled = %+v", status)
	}
	env.manager.Configure(env.settings())
	status := env.waitFor("the open attempt", func(s Status) bool { return !s.Loading })
	if status.State != StateError || status.ErrorCode != CodeZIMUnreadable || status.Readable || status.Edition == nil {
		t.Fatalf("status after switching on = %+v", status)
	}
	env.manager.mu.Lock()
	pending := env.manager.staleLocked()
	env.manager.mu.Unlock()
	if pending {
		t.Fatal("the unreadable edition is still pending to be opened")
	}
}

// Switching off and on again retries an edition that could not be opened, so
// a file replaced by hand while the integration was off is served; this holds
// for a failed open after switching on and for one at load time.
func TestManagerSwitchingOffAndOnRetriesAnUnreadableEdition(t *testing.T) {
	for _, startEnabled := range []bool{false, true} {
		env := newTestEnv(t)
		edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		file := filepath.Join(env.dir, edition.FileName)
		good, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("not a zim"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !startEnabled {
			env.manager.Configure(env.disabledSettings())
		}
		env.start()
		if !startEnabled {
			env.manager.Configure(env.settings())
		}
		status := env.waitFor("the failed open", func(s Status) bool { return !s.Loading && s.ErrorCode == CodeZIMUnreadable })
		if status.Readable {
			t.Fatalf("start enabled %v: an unreadable edition is readable: %+v", startEnabled, status)
		}

		env.manager.Configure(env.disabledSettings())
		if err := os.WriteFile(file, good, 0o644); err != nil {
			t.Fatal(err)
		}
		env.manager.Configure(env.settings())
		status = env.waitFor("the repaired edition opened", func(s Status) bool { return s.Readable && !s.Loading })
		if status.State != StateReady || status.ErrorCode != "" || status.Edition == nil || status.Edition.Name != edition.Name {
			t.Fatalf("start enabled %v: status after switching on again = %+v", startEnabled, status)
		}
		lib, release, ok := env.manager.Acquire()
		if !ok || lib.Edition().Name != edition.Name {
			t.Fatalf("start enabled %v: Acquire = %v, %v", startEnabled, lib, ok)
		}
		release()
	}
}

// A setting change that keeps the integration on does not retry an
// unreadable edition (the loop would otherwise reopen it on every save).
func TestManagerKeepsAnUnreadableEditionWhileStillOn(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()
	env.waitFor("the failed open", func(s Status) bool { return !s.Loading && s.ErrorCode == CodeZIMUnreadable })
	settings := env.settings()
	settings.UpdateCheck = !settings.UpdateCheck
	env.manager.Configure(settings)
	env.manager.mu.Lock()
	code, pending := env.manager.loadCode, env.manager.staleLocked()
	env.manager.mu.Unlock()
	if code != CodeZIMUnreadable || pending {
		t.Fatalf("a save while on reset the unreadable edition: code %q, pending %v", code, pending)
	}
}
