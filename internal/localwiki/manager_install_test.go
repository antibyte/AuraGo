package localwiki

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManagerInstallDownloadsVerifiesAndPublishes(t *testing.T) {
	env := newTestEnv(t)
	data := fixtureZIMBytes(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
	env.start()
	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	status := env.waitIdle(StateReady)
	edition := status.Edition
	if edition == nil || edition.Name != served.name || edition.FileName != served.name+".zim" || edition.Size != int64(len(data)) ||
		edition.SHA256 != served.sha256 || len(edition.UUID) != 36 || edition.ArticleCount <= 0 || edition.InstalledAt.IsZero() {
		t.Fatalf("edition = %+v", edition)
	}
	if !status.Fulltext || !status.SelectionMatchesInstalled || status.ErrorCode != "" {
		t.Fatalf("status = %+v", status)
	}
	if !fileExists(filepath.Join(env.dir, edition.FileName)) || fileExists(filepath.Join(env.dir, edition.FileName+".part")) ||
		fileExists(filepath.Join(env.dir, downloadFileName)) || !fileExists(filepath.Join(env.dir, stateFileName)) {
		t.Fatal("unexpected files after publication")
	}
	lib, release, ok := env.manager.Acquire()
	if !ok || lib.Edition().Name != served.name {
		t.Fatalf("Acquire = %v", ok)
	}
	release()
	if err := env.manager.Install(context.Background(), InstallRequest{}); !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("second Install = %v, want ErrAlreadyInstalled", err)
	}
}

func TestManagerInstallPreflightErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("disabled", func(t *testing.T) {
		env := newTestEnv(t)
		settings := env.settings()
		settings.Enabled = false
		env.manager.Configure(settings)
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrDisabled) {
			t.Fatalf("Install = %v", err)
		}
	})
	t.Run("storage directory", func(t *testing.T) {
		env := newTestEnv(t)
		settings := env.settings()
		settings.DataDir = "relative/wikipedia"
		env.manager.Configure(settings)
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrDataDirInvalid) {
			t.Fatalf("relative directory: %v", err)
		}
		if env.manager.Status().ErrorCode != CodeDataDirInvalid {
			t.Fatal("status does not flag the invalid directory")
		}
		env.manager.Configure(env.settings())
		env.manager.sensitive = func(string) bool { return true }
		if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrDataDirInvalid) {
			t.Fatalf("sensitive directory: %v", err)
		}
	})
	t.Run("catalog unreachable", func(t *testing.T) {
		env := newTestEnv(t)
		env.kiwix.setCatalogStatus(http.StatusServiceUnavailable)
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrCatalogUnreachable) {
			t.Fatalf("Install = %v", err)
		}
		if env.manager.Status().State != StateNotInstalled {
			t.Fatal("a refused preflight must not change the state")
		}
	})
	t.Run("free space", func(t *testing.T) {
		env := newTestEnv(t)
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
		env.start()
		env.free.Store(10)
		var space *InsufficientSpaceError
		err := env.manager.Install(ctx, InstallRequest{})
		if !errors.As(err, &space) || space.Required != int64(len(served.data))+1<<30 || space.Available != 10 || space.CanDeleteOld {
			t.Fatalf("Install = %v (%+v)", err, space)
		}
		env.freeErr.Store(true)
		if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrUnknownFreeSpace) {
			t.Fatalf("unknown free space = %v", err)
		}
		if err := env.manager.Install(ctx, InstallRequest{ConfirmUnknownSpace: true}); err != nil {
			t.Fatalf("confirmed install = %v", err)
		}
		env.waitIdle(StateReady)
	})
}

func TestManagerCancelResumeAndRestart(t *testing.T) {
	env := newTestEnv(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		served.setMode(mirror, "slow")
	}
	defer served.unblock()
	env.start()
	ctx := context.Background()
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	if err := env.manager.Install(ctx, InstallRequest{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("Install during a download = %v, want ErrBusy", err)
	}
	if err := env.manager.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	status := env.manager.Status()
	if status.State != StateInterrupted || status.OperationInProgress {
		t.Fatalf("status after cancel = %+v", status)
	}
	part := filepath.Join(env.dir, served.name+".zim.part")
	if info, err := os.Stat(part); err != nil || info.Size() < 1024 {
		t.Fatalf("part file after cancel: %v", err)
	}
	if err := env.manager.Cancel(); !errors.Is(err, ErrNoOperation) {
		t.Fatalf("second Cancel = %v", err)
	}

	// A restart reports the interrupted download and does not resume it.
	requests := len(env.kiwix.requestLog())
	ctxShutdown, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := env.manager.Shutdown(ctxShutdown); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	env.manager = env.newManager()
	env.manager.Configure(env.settings())
	env.start()
	time.Sleep(100 * time.Millisecond)
	if status := env.manager.Status(); status.State != StateInterrupted || len(env.kiwix.requestLog()) != requests {
		t.Fatalf("restart resumed or lost the download: %+v", status)
	}

	// Resume continues with a Range request after re-hashing the part file.
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		served.setMode(mirror, "")
	}
	partSize, _ := os.Stat(part)
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatalf("resume Install: %v", err)
	}
	env.waitIdle(StateReady)
	want := "bytes=" + strconv.FormatInt(partSize.Size(), 10) + "-"
	resumed := false
	for _, request := range env.kiwix.requestLog()[requests:] {
		resumed = resumed || request.Range == want
	}
	if !resumed {
		t.Fatalf("resume did not request the missing range: %v", env.kiwix.requestLog()[requests:])
	}
}

func TestManagerFailuresAreReported(t *testing.T) {
	ctx := context.Background()
	t.Run("checksum mismatch", func(t *testing.T) {
		env := newTestEnv(t)
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
		served.sha256 = strings.Repeat("0", 64)
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
			t.Fatal(err)
		}
		status := env.waitIdle(StateError)
		if status.ErrorCode != CodeChecksumMismatch || fileExists(filepath.Join(env.dir, served.name+".zim.part")) ||
			fileExists(filepath.Join(env.dir, downloadFileName)) {
			t.Fatalf("status = %+v", status)
		}
	})
	t.Run("not a ZIM", func(t *testing.T) {
		env := newTestEnv(t)
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(4096))
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
			t.Fatal(err)
		}
		status := env.waitIdle(StateError)
		if status.ErrorCode != CodeZIMUnreadable || fileExists(filepath.Join(env.dir, served.name+".zim")) ||
			fileExists(filepath.Join(env.dir, served.name+".zim.part")) {
			t.Fatalf("status = %+v", status)
		}
	})
	t.Run("all mirrors failed", func(t *testing.T) {
		env := newTestEnv(t)
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
		for _, mirror := range []string{"m1", "m2", "fallback"} {
			served.setMode(mirror, "fail")
		}
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
			t.Fatal(err)
		}
		status := env.waitIdle(StateInterrupted)
		if status.ErrorCode != CodeDownloadFailed || !fileExists(filepath.Join(env.dir, downloadFileName)) {
			t.Fatalf("status = %+v", status)
		}
	})
	t.Run("disk fills up", func(t *testing.T) {
		env := newTestEnv(t)
		served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(256<<10))
		served.setMode("m1", "slow")
		served.slowChunk = 8 << 10
		env.manager.diskInterval = 4 << 10
		env.start()
		if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
			t.Fatal(err)
		}
		env.free.Store(10)
		served.unblock()
		status := env.waitIdle(StateInterrupted)
		if status.ErrorCode != CodeInsufficientDiskSpace || status.RequiredBytes <= 1<<30 || status.FreeBytes != 10 {
			t.Fatalf("status = %+v", status)
		}
		if !fileExists(filepath.Join(env.dir, served.name+".zim.part")) {
			t.Fatal("the part file must stay after a pause")
		}
	})
}

// A state.json that cannot be replaced fails the publication after the file
// was moved into place; the verified download must survive for a retry.
func TestManagerPublicationKeepsTheVerifiedDownloadWhenStateCannotBeWritten(t *testing.T) {
	env := newTestEnv(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	env.start()
	blocker := filepath.Join(env.dir, stateFileName)
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocker, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatal(err)
	}
	status := env.waitIdle(StateInterrupted)
	part := filepath.Join(env.dir, served.name+".zim.part")
	if status.ErrorCode != CodeDownloadFailed || !fileExists(part) || fileExists(filepath.Join(env.dir, served.name+".zim")) {
		t.Fatalf("status = %+v", status)
	}
	if _, _, ok := env.manager.Acquire(); ok {
		t.Fatal("a failed publication must not put the edition online")
	}

	if err := os.RemoveAll(blocker); err != nil {
		t.Fatal(err)
	}
	downloads := 0
	for _, request := range env.kiwix.requestLog() {
		if strings.HasSuffix(request.Path, ".zim") {
			downloads++
		}
	}
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatalf("retry Install: %v", err)
	}
	env.waitIdle(StateReady)
	after := 0
	for _, request := range env.kiwix.requestLog() {
		if strings.HasSuffix(request.Path, ".zim") {
			after++
		}
	}
	if after != downloads {
		t.Fatalf("the retry downloaded the edition again (%d -> %d requests)", downloads, after)
	}
}

// A storage-directory change made while a download runs is held back; it must
// be applied when the download ends.
func TestManagerLoadsAPendingDirectoryChangeWhenTheOperationEnds(t *testing.T) {
	env := newTestEnv(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		served.setMode(mirror, "slow")
	}
	defer served.unblock()
	env.start()
	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatal(err)
	}
	env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	other := filepath.Join(t.TempDir(), "elsewhere")
	settings := env.settings()
	settings.DataDir = other
	env.manager.Configure(settings)
	time.Sleep(50 * time.Millisecond)
	if status := env.manager.Status(); !status.OperationInProgress || status.State == StateNotInstalled {
		t.Fatalf("the running download lost its directory: %+v", status)
	}
	if err := env.manager.Cancel(); err != nil {
		t.Fatal(err)
	}
	env.waitFor("the new directory", func(s Status) bool { return s.DataDir == other && s.State == StateNotInstalled })
}

func TestMirrorListKeepsTheFallbackWithinTheLimit(t *testing.T) {
	fallback := "https://download.example/zim/wikipedia/x.zim"
	var mirrors []string
	for i := 0; i < maxMirrors; i++ {
		mirrors = append(mirrors, "https://m"+strconv.Itoa(i)+".example/x.zim")
	}
	got := mirrorList(mirrors, fallback)
	if len(got) != maxMirrors || got[len(got)-1] != fallback || got[0] != mirrors[0] {
		t.Fatalf("mirrorList = %v", got)
	}
	got = mirrorList([]string{fallback, mirrors[0]}, fallback)
	if len(got) != 2 || got[0] != mirrors[0] || got[1] != fallback {
		t.Fatalf("a mirror equal to the fallback must not repeat: %v", got)
	}
}

// A restart file left by a killed process is removed when the directory is
// loaded and again before Install checks the free space, so it never causes a
// false "insufficient space".
func TestManagerRemovesStaleRestartFilesBeforeTheSpaceCheck(t *testing.T) {
	const kept = 4096
	env := newTestEnv(t)
	data := fixtureZIMBytes(t)
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
	target := Edition{
		Language: "de", Variant: VariantNoPic, Date: "2026-10", Name: served.name, FileName: served.name + ".zim",
		Size: int64(len(data)), SHA256: served.sha256,
	}
	if err := os.MkdirAll(env.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served), StartedAt: env.clock.Now()}); err != nil {
		t.Fatal(err)
	}
	partPath := filepath.Join(env.dir, target.FileName+".part")
	if err := os.WriteFile(partPath, data[:kept], 0o644); err != nil {
		t.Fatal(err)
	}
	restartPath := partPath + restartSuffix
	plantRestart := func() {
		t.Helper()
		if err := os.WriteFile(restartPath, make([]byte, 64<<10), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The disk has room for exactly the resumed download; a restart file
	// still lying there takes its size away.
	required := requiredBytes(target.Size, kept)
	m := NewManager(Deps{
		HTTPClient: env.kiwix.server.Client(),
		FreeDiskBytes: func(string) (int64, error) {
			if info, err := os.Stat(restartPath); err == nil {
				return required - info.Size(), nil
			}
			return required, nil
		},
		Now:            env.clock.Now,
		CatalogBaseURL: env.kiwix.server.URL,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.Shutdown(ctx)
	})
	env.manager = m
	m.Configure(env.settings())

	plantRestart()
	env.start()
	if fileExists(restartPath) {
		t.Fatal("loading the directory left the stale restart file")
	}
	plantRestart()
	if err := m.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatalf("Install = %v, want the resume to fit once the stale restart file is gone", err)
	}
	env.waitIdle(StateReady)
	if fileExists(restartPath) || fileExists(partPath) {
		t.Fatal("restart or part file left after the install")
	}
}
