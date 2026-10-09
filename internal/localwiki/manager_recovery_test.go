package localwiki

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

// downloadTargetFor describes a fake Kiwix edition the way download.json does.
func downloadTargetFor(served *fakeEdition, language string) Edition {
	parsed, _ := parseEditionName(served.name)
	return Edition{
		Language: language, Variant: parsed.Variant, Date: parsed.Date, Name: served.name, FileName: served.name + ".zim",
		Size: int64(len(served.data)), SHA256: served.sha256,
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func zimRequests(env *testEnv) int {
	count := 0
	for _, request := range env.kiwix.requestLog() {
		if strings.HasSuffix(request.Path, ".zim") {
			count++
		}
	}
	return count
}

// A crash between the rename and the state.json write leaves the finished
// <edition>.zim unnamed by state.json. Loading turns it back into the partial
// file, so Resume only re-hashes it and needs no space for a second copy.
func TestManagerRecoversAFinishedDownloadThatWasNotPublished(t *testing.T) {
	env := newTestEnv(t)
	data := fixtureZIMBytes(t)
	old := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
	target := downloadTargetFor(served, "de")
	if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served)}); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(env.dir, target.FileName)
	if err := os.WriteFile(orphan, data, 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()

	status := env.manager.Status()
	if status.State != StateInterrupted || status.Edition == nil || status.Edition.Name != old.Name {
		t.Fatalf("status after the restart = %+v", status)
	}
	if fileExists(orphan) || !fileExists(orphan+".part") {
		t.Fatal("the finished download was not turned back into the partial file")
	}
	if _, release, ok := env.manager.Acquire(); !ok {
		t.Fatal("the installed edition must stay online")
	} else {
		release()
	}

	env.free.Store(1<<30 + 10) // the margin only: nothing is left to download
	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	status = env.waitFor("the recovered edition", func(s Status) bool {
		return !s.OperationInProgress && s.Edition != nil && s.Edition.Name == served.name
	})
	if status.State != StateReady || zimRequests(env) != 0 {
		t.Fatalf("status = %+v, edition downloads = %d", status, zimRequests(env))
	}
	if !fileExists(orphan) || fileExists(orphan+".part") || fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("unexpected files after the recovered publication")
	}
}

// A crash after state.json was written leaves the download.json of the edition
// that is installed now.
func TestManagerDropsTheDownloadOfTheInstalledEdition(t *testing.T) {
	env := newTestEnv(t)
	installed := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-10")
	served := env.kiwix.addEdition(installed.Name, fixtureZIMBytes(t))
	if err := writeDownload(env.dir, &downloadFile{Target: downloadTargetFor(served, "de"), URLs: env.kiwix.mirrorURLs(served)}); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(env.dir, installed.FileName+".part")
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()
	status := env.manager.Status()
	if status.State != StateReady || status.Edition == nil || status.Edition.Name != installed.Name {
		t.Fatalf("status = %+v", status)
	}
	if fileExists(filepath.Join(env.dir, downloadFileName)) || fileExists(stale) || !fileExists(filepath.Join(env.dir, installed.FileName)) {
		t.Fatal("the stale download.json or partial file was kept, or the edition was touched")
	}
}

func TestManagerDeleteRemovesAnUnpublishedDownload(t *testing.T) {
	t.Run("after the recovery at start", func(t *testing.T) {
		env := newTestEnv(t)
		data := fixtureZIMBytes(t)
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
		mustMkdir(t, env.dir)
		target := downloadTargetFor(served, "de")
		if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served)}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.dir, target.FileName), data, 0o644); err != nil {
			t.Fatal(err)
		}
		env.start()
		if err := env.manager.Delete(); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		entries, err := os.ReadDir(env.dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("files left behind: %v, %v", entries, err)
		}
		if status := env.manager.Status(); status.State != StateNotInstalled {
			t.Fatalf("status = %+v", status)
		}
	})
	t.Run("finished file next to a partial one", func(t *testing.T) {
		// Both files exist (an earlier version of AuraGo resumed from scratch),
		// so loading leaves them alone; Delete must still remove the finished one.
		env := newTestEnv(t)
		old := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
		target := downloadTargetFor(served, "de")
		if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served)}); err != nil {
			t.Fatal(err)
		}
		finished := filepath.Join(env.dir, target.FileName)
		for _, path := range []string{finished, finished + ".part"} {
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		env.start()
		if err := env.manager.Delete(); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		entries, err := os.ReadDir(env.dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("files left behind (old edition %s): %v, %v", old.FileName, entries, err)
		}
	})
}

// download.json goes before the partial files, so a Delete that stops half way
// leaves nothing that looks like a resumable download. Only a Windows file
// that is still open fails to delete portably.
func TestManagerDeleteRemovesDownloadJSONBeforeThePartialFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("a deletion failure that works on every platform needs an open file, which only Windows refuses to delete")
	}
	env := newTestEnv(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	mustMkdir(t, env.dir)
	target := downloadTargetFor(served, "de")
	if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served)}); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(env.dir, target.FileName+".part")
	if err := os.WriteFile(part, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()
	held, err := os.Open(part)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := env.manager.Delete(); err == nil {
		t.Fatal("Delete succeeded although the partial file is open")
	}
	if fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("download.json must be removed before the partial files")
	}
	_ = held.Close()
	if err := env.manager.Delete(); err != nil || fileExists(part) {
		t.Fatalf("second Delete = %v", err)
	}
}

// Switching the selection back to the installed edition and pressing Install
// must not leave the abandoned download of the other language behind.
func TestManagerInstallOfTheInstalledEditionDropsAnAbandonedDownload(t *testing.T) {
	env := newTestEnv(t)
	installed := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.kiwix.addEdition(installed.Name, fixtureZIMBytes(t))
	english := env.kiwix.addEdition("wikipedia_en_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		english.setMode(mirror, "slow")
	}
	defer english.unblock()
	env.start()
	ctx := context.Background()

	settings := env.settings()
	settings.Language = "en"
	env.manager.Configure(settings)
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatalf("Install(en): %v", err)
	}
	env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	if err := env.manager.Cancel(); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(env.dir, english.name+".zim.part")
	if status := env.manager.Status(); status.State != StateInterrupted || !fileExists(part) {
		t.Fatalf("status after cancel = %+v", status)
	}

	env.manager.Configure(env.settings())
	if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("Install(de) = %v, want ErrAlreadyInstalled", err)
	}
	status := env.manager.Status()
	if status.State != StateReady || status.ErrorCode != "" || status.Edition == nil || status.Edition.Name != installed.Name {
		t.Fatalf("status = %+v", status)
	}
	if fileExists(part) || fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("the abandoned download was kept")
	}
	if !fileExists(filepath.Join(env.dir, installed.FileName)) {
		t.Fatal("the installed edition was touched")
	}
}

func TestManagerFailedUpdateKeepsServingAndReportsReady(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		prepare func(*fakeEdition) []byte
		code    string
	}{
		{"checksum mismatch", func(e *fakeEdition) []byte { e.sha256 = strings.Repeat("0", 64); return e.data }, CodeChecksumMismatch},
		{"not a ZIM", nil, CodeZIMUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			old := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
			data := fixtureZIMBytes(t)
			if tc.prepare == nil {
				data = testPayload(4096)
			}
			served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
			if tc.prepare != nil {
				tc.prepare(served)
			}
			env.start()
			if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
				t.Fatal(err)
			}
			status := env.waitFor("the failed update", func(s Status) bool { return !s.OperationInProgress && s.ErrorCode == tc.code })
			if status.State != StateReady || status.Edition == nil || status.Edition.Name != old.Name || !status.Fulltext {
				t.Fatalf("a failed update must keep the installed edition ready: %+v", status)
			}
			if status.Recommendation == "" || strings.Contains(status.Recommendation, "Delete the edition") {
				t.Fatalf("recommendation for a failed update = %q", status.Recommendation)
			}
			if _, release, ok := env.manager.Acquire(); !ok {
				t.Fatal("the installed edition went offline")
			} else {
				release()
			}
		})
	}

	t.Run("startup error stays an error", func(t *testing.T) {
		env := newTestEnv(t)
		edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
			t.Fatal(err)
		}
		env.start()
		status := env.manager.Status()
		if status.State != StateError || status.ErrorCode != CodeZIMUnreadable || !strings.Contains(status.Recommendation, "Delete the edition") {
			t.Fatalf("status = %+v", status)
		}
	})

	t.Run("a new edition replaces an unreadable one", func(t *testing.T) {
		env := newTestEnv(t)
		edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
			t.Fatal(err)
		}
		env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
			t.Fatal(err)
		}
		status := env.waitFor("the replacement", func(s Status) bool {
			return !s.OperationInProgress && s.Edition != nil && s.Edition.Name == "wikipedia_de_all_nopic_2026-10"
		})
		if status.State != StateReady || status.ErrorCode != "" {
			t.Fatalf("status after the replacement = %+v", status)
		}
	})
}

func TestManagerKeepsTheUpdateNoticeAcrossRestarts(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(5000))
	env.start()
	if err := env.manager.CheckUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := UpdateInfo{Date: "2026-10", Size: 5000}
	if got := env.manager.Status().UpdateAvailable; got == nil || *got != want {
		t.Fatalf("update = %+v", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := env.manager.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	requests := len(env.kiwix.requestLog())
	env.manager = env.newManager()
	env.manager.Configure(env.settings())
	env.start()
	if got := env.manager.Status().UpdateAvailable; got == nil || *got != want {
		t.Fatalf("update after the restart = %+v", got)
	}
	if len(env.kiwix.requestLog()) != requests {
		t.Fatal("Start contacted the network")
	}
}

func TestReadStateDropsAnUpdateThatNoLongerApplies(t *testing.T) {
	edition := Edition{
		Language: "de", Variant: VariantNoPic, Date: "2026-09", Name: "wikipedia_de_all_nopic_2026-09", FileName: "wikipedia_de_all_nopic_2026-09.zim",
		Size: 10, SHA256: strings.Repeat("ab", 32),
	}
	cases := []struct {
		name    string
		edition *Edition
		update  *stateUpdate
		keep    bool
	}{
		{"newer edition", &edition, &stateUpdate{Name: "wikipedia_de_all_nopic_2026-10", Date: "bogus", Size: 5}, true},
		{"same edition", &edition, &stateUpdate{Name: "wikipedia_de_all_nopic_2026-09", Size: 5}, false},
		{"older edition", &edition, &stateUpdate{Name: "wikipedia_de_all_nopic_2026-08", Size: 5}, false},
		{"other language", &edition, &stateUpdate{Name: "wikipedia_en_all_nopic_2026-10", Size: 5}, false},
		{"other variant", &edition, &stateUpdate{Name: "wikipedia_de_all_maxi_2026-10", Size: 5}, false},
		{"bad name", &edition, &stateUpdate{Name: "../wikipedia_de_all_nopic_2026-10", Size: 5}, false},
		{"implausible size", &edition, &stateUpdate{Name: "wikipedia_de_all_nopic_2026-10", Size: maxEditionBytes + 1}, false},
		{"nothing installed", nil, &stateUpdate{Name: "wikipedia_de_all_nopic_2026-10", Size: 5}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeState(dir, &stateFile{Edition: tc.edition, Update: tc.update, PendingDelete: []string{"wikipedia_de_all_nopic_2026-01.zim"}}); err != nil {
				t.Fatal(err)
			}
			st, err := readState(dir)
			if err != nil {
				t.Fatal(err)
			}
			if tc.keep != (st.Update != nil) {
				t.Fatalf("update = %+v, keep = %v", st.Update, tc.keep)
			}
			if tc.keep && st.Update.Date != "2026-10" {
				t.Fatalf("the date must come from the edition name: %+v", st.Update)
			}
		})
	}
}

// A retired edition of the same name still waiting for its deletion must not
// be deleted after the download has been published under that name.
func TestManagerInstallUnlistsTheTargetFromPendingDeletes(t *testing.T) {
	env := newTestEnv(t)
	installed := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		served.setMode(mirror, "slow")
	}
	defer served.unblock()
	stuck := filepath.Join(env.dir, served.name+".zim") // a non-empty directory cannot be deleted as a file
	if err := os.MkdirAll(stuck, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stuck, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeState(env.dir, &stateFile{Edition: &installed, PendingDelete: []string{served.name + ".zim"}}); err != nil {
		t.Fatal(err)
	}
	env.start()
	if st, err := readState(env.dir); err != nil || len(st.PendingDelete) != 1 {
		t.Fatalf("the undeletable file must stay listed: %+v, %v", st, err)
	}
	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatal(err)
	}
	env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	if st, err := readState(env.dir); err != nil || len(st.PendingDelete) != 0 || st.Edition == nil {
		t.Fatalf("the target is still scheduled for deletion: %+v, %v", st, err)
	}
	env.manager.retryPendingDeletes()
	if !fileExists(stuck) {
		t.Fatal("a retry deleted the file the download is about to publish")
	}
}

// The conditions of a reload are checked after taking the load lock: a load
// that finished while the call waited must not be repeated.
func TestManagerReloadChecksItsConditionsUnderTheLoadLock(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	other := filepath.Join(t.TempDir(), "elsewhere")
	placeEdition(t, other, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	m := env.manager

	m.loadMu.Lock() // another load is in progress
	settings := env.settings()
	settings.DataDir = other
	m.Configure(settings) // wakes the background loop, which now waits for the lock
	done := make(chan struct{})
	go func() {
		m.loadIfStale()
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	m.mu.Lock()
	m.activeDir = other // the load that held the lock has finished
	current := m.lib
	m.mu.Unlock()
	m.loadMu.Unlock()
	<-done
	time.Sleep(100 * time.Millisecond)
	m.mu.Lock()
	same := m.lib == current
	m.mu.Unlock()
	if !same {
		t.Fatal("a directory that was loaded meanwhile was loaded again")
	}
}

func TestRateMeterThrottlesItsSamples(t *testing.T) {
	var meter rateMeter
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	meter.observe(start, 0)
	if rate := meter.observe(start.Add(100*time.Millisecond), 1_000_000); rate != 0 || len(meter.samples) != 1 {
		t.Fatalf("a sample 100 ms after the last one was recorded: rate %d, %d samples", rate, len(meter.samples))
	}
	if rate := meter.observe(start.Add(10*time.Second), 100_000_000); rate != 10_000_000 || len(meter.samples) != 2 {
		t.Fatalf("rate = %d with %d samples", rate, len(meter.samples))
	}
	if rate := meter.observe(start.Add(10*time.Second+200*time.Millisecond), 101_000_000); rate != 10_000_000 || len(meter.samples) != 2 {
		t.Fatalf("a throttled observation must return the last rate: %d, %d samples", rate, len(meter.samples))
	}
	if rate := meter.observe(start.Add(10*time.Second+300*time.Millisecond), 0); rate != 0 {
		t.Fatalf("a restart from byte 0 must reset the meter, got %d", rate)
	}
}
