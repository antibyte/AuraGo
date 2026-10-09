package localwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRateMeter(t *testing.T) {
	var meter rateMeter
	start := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if rate := meter.observe(start, 0); rate != 0 {
		t.Fatalf("first sample rate = %d", rate)
	}
	if rate := meter.observe(start.Add(10*time.Second), 100_000_000); rate != 10_000_000 {
		t.Fatalf("rate = %d, want 10 MB/s", rate)
	}
	// Sparse samples (one per MiB on a slow line) keep an anchor older than the window.
	if rate := meter.observe(start.Add(40*time.Second), 400_000_000); rate != 10_000_000 {
		t.Fatalf("rate with sparse samples = %d, want 10 MB/s", rate)
	}
	if rate := meter.observe(start.Add(41*time.Second), 0); rate != 0 {
		t.Fatalf("a restart from byte 0 must reset the meter, got %d", rate)
	}
}

func TestManagerStatusBeforeAnyInstall(t *testing.T) {
	env := newTestEnv(t)
	env.start()
	status := env.manager.Status()
	if status.State != StateNotInstalled || status.Edition != nil || status.OperationInProgress {
		t.Fatalf("status = %+v", status)
	}
	if status.Selection != (Selection{Language: "de", Variant: VariantNoPic}) || status.DataDir != env.dir || status.SystemLanguage != "de" {
		t.Fatalf("selection/dir = %+v", status)
	}
	if status.FreeBytes != 1<<40 || len(status.Languages) != 16 || status.ErrorCode != "" {
		t.Fatalf("free/languages/error = %d %d %q", status.FreeBytes, len(status.Languages), status.ErrorCode)
	}
	env.freeErr.Store(true)
	if env.manager.Status().FreeBytes != -1 {
		t.Fatal("unknown free space must be reported as -1")
	}
	if _, release, ok := env.manager.Acquire(); ok {
		release()
		t.Fatal("Acquire succeeded without an edition")
	}
}

func TestManagerStartLoadsInstalledEditionWithoutNetwork(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	status := env.manager.Status()
	if status.State != StateReady || status.Edition == nil || status.Edition.Name != edition.Name ||
		!status.SelectionMatchesInstalled || !status.Fulltext || status.ErrorCode != "" {
		t.Fatalf("status = %+v", status)
	}
	lib, release, ok := env.manager.Acquire()
	if !ok || lib.Edition().Name != edition.Name {
		t.Fatalf("Acquire = %v, %v", lib, ok)
	}
	release()
	release() // extra calls are ignored
	if len(env.kiwix.requestLog()) != 0 {
		t.Fatalf("Start contacted the network: %v", env.kiwix.requestLog())
	}
}

func TestManagerReportsUnreadableInstalledEdition(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()
	status := env.manager.Status()
	if status.State != StateError || status.ErrorCode != CodeZIMUnreadable || status.Recommendation == "" || status.Edition == nil {
		t.Fatalf("status = %+v", status)
	}
	if _, _, ok := env.manager.Acquire(); ok {
		t.Fatal("Acquire succeeded for an unreadable edition")
	}
}

func TestManagerReportsInterruptedDownloadWithoutResuming(t *testing.T) {
	env := newTestEnv(t)
	if err := os.MkdirAll(env.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := Edition{
		Language: "de", Variant: VariantNoPic, Name: "wikipedia_de_all_nopic_2026-10", FileName: "wikipedia_de_all_nopic_2026-10.zim",
		Size: 10, SHA256: strings.Repeat("ab", 32),
	}
	if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: []string{env.kiwix.server.URL + "/m1/" + target.FileName}}); err != nil {
		t.Fatal(err)
	}
	env.start()
	time.Sleep(100 * time.Millisecond)
	if status := env.manager.Status(); status.State != StateInterrupted || status.OperationInProgress {
		t.Fatalf("status = %+v", status)
	}
	if len(env.kiwix.requestLog()) != 0 {
		t.Fatal("an interrupted download was resumed automatically")
	}
}

func TestManagerSelectionAndEnabledGates(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	settings := env.settings()
	settings.Language = "en"
	env.manager.Configure(settings)
	status := env.manager.Status()
	if status.SelectionMatchesInstalled || status.Edition.Language != "de" || status.Selection.Language != "en" {
		t.Fatalf("selection mismatch not reported: %+v", status)
	}
	if _, release, ok := env.manager.Acquire(); !ok {
		t.Fatal("a selection change must not take the installed edition offline")
	} else {
		release()
	}
	settings.Enabled = false
	env.manager.Configure(settings)
	if _, _, ok := env.manager.Acquire(); ok {
		t.Fatal("Acquire succeeded while the integration is disabled")
	}
	if env.manager.Settings().Enabled {
		t.Fatal("Settings() does not reflect Configure")
	}
}

func TestManagerReloadsChangedStorageDirectory(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	other := filepath.Join(t.TempDir(), "elsewhere")
	settings := env.settings()
	settings.DataDir = other
	env.manager.Configure(settings)
	env.waitFor("reload of the new directory", func(s Status) bool { return s.DataDir == other && s.State == StateNotInstalled })
	if !fileExists(filepath.Join(env.dir, "wikipedia_de_all_nopic_2026-09.zim")) {
		t.Fatal("changing the directory must never delete the old edition")
	}
	env.manager.Configure(env.settings())
	env.waitIdle(StateReady)
}

func TestManagerFinishesPendingDeletesAtStart(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-10")
	old := filepath.Join(env.dir, "wikipedia_de_all_nopic_2026-09.zim")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(env.dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeState(env.dir, &stateFile{Edition: &edition, PendingDelete: []string{"wikipedia_de_all_nopic_2026-09.zim", "notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	env.start()
	if fileExists(old) || !fileExists(foreign) {
		t.Fatal("pending delete must remove only listed edition files")
	}
	st, err := readState(env.dir)
	if err != nil || len(st.PendingDelete) != 0 {
		t.Fatalf("pending list not cleared: %+v, %v", st, err)
	}
}
