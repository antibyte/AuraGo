package localwiki

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func restartedManager(t *testing.T, env *testEnv) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := env.manager.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	env.manager = env.newManager()
	env.manager.Configure(env.settings())
	env.start()
}

// With an unreadable state.json nothing says which files belong to the
// installed edition: the directory must stay as it is, and the next start that
// can read the state must find the edition intact.
func TestManagerDoesNotReconcileWithoutAReadableState(t *testing.T) {
	env := newTestEnv(t)
	installed := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	served := env.kiwix.addEdition(installed.Name, fixtureZIMBytes(t))
	if err := writeDownload(env.dir, &downloadFile{Target: downloadTargetFor(served, "de"), URLs: env.kiwix.mirrorURLs(served)}); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(env.dir, stateFileName)
	good, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	edition := filepath.Join(env.dir, installed.FileName)

	env.start()
	status := env.manager.Status()
	if status.ErrorCode != CodeZIMUnreadable || status.Readable {
		t.Fatalf("status with an unreadable state.json = %+v", status)
	}
	if !fileExists(edition) || fileExists(edition+".part") || !fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("the directory was changed although state.json could not be read")
	}

	if err := os.WriteFile(statePath, good, 0o644); err != nil {
		t.Fatal(err)
	}
	restartedManager(t, env)
	status = env.manager.Status()
	if status.State != StateReady || !status.Readable || status.Edition == nil || status.Edition.Name != installed.Name || status.ErrorCode != "" {
		t.Fatalf("status after the state became readable = %+v", status)
	}
	if !fileExists(edition) || fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("the installed edition or the stale download.json is wrong")
	}
}

// A directory an earlier version damaged: the installed edition's file was set
// aside as a partial download. It is put back when it has the recorded size.
func TestManagerRestoresTheInstalledEditionFromAMisnamedPartialFile(t *testing.T) {
	setup := func(t *testing.T, partData []byte) (*testEnv, Edition) {
		env := newTestEnv(t)
		installed := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
		served := env.kiwix.addEdition(installed.Name, fixtureZIMBytes(t))
		if err := writeDownload(env.dir, &downloadFile{Target: downloadTargetFor(served, "de"), URLs: env.kiwix.mirrorURLs(served)}); err != nil {
			t.Fatal(err)
		}
		edition := filepath.Join(env.dir, installed.FileName)
		if err := os.Remove(edition); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(edition+".part", partData, 0o644); err != nil {
			t.Fatal(err)
		}
		return env, installed
	}

	t.Run("complete file", func(t *testing.T) {
		env, installed := setup(t, fixtureZIMBytes(t))
		env.start()
		status := env.manager.Status()
		if status.State != StateReady || !status.Readable || status.ErrorCode != "" {
			t.Fatalf("status = %+v", status)
		}
		edition := filepath.Join(env.dir, installed.FileName)
		if !fileExists(edition) || fileExists(edition+".part") || fileExists(filepath.Join(env.dir, downloadFileName)) {
			t.Fatal("the edition was not put back in place")
		}
	})
	t.Run("size differs", func(t *testing.T) {
		env, installed := setup(t, []byte("short"))
		env.start()
		status := env.manager.Status()
		if status.State != StateError || status.Readable || status.ErrorCode != CodeZIMUnreadable {
			t.Fatalf("status = %+v", status)
		}
		edition := filepath.Join(env.dir, installed.FileName)
		if fileExists(edition) || fileExists(edition+".part") {
			t.Fatal("a partial file of the wrong size must not become the edition")
		}
	})
}

// Only a regular file named like the finished download is set aside as the
// partial file: a directory or a link of that name is left alone.
func TestManagerReconcileIgnoresLinksAndDirectories(t *testing.T) {
	cases := map[string]func(t *testing.T, env *testEnv, path string, data []byte){
		"directory": func(t *testing.T, env *testEnv, path string, _ []byte) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"symbolic link": func(t *testing.T, env *testEnv, path string, data []byte) {
			real := filepath.Join(env.dir, "elsewhere.bin")
			if err := os.WriteFile(real, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(real, path); err != nil {
				t.Skipf("symbolic links are not available: %v", err)
			}
		},
	}
	for name, create := range cases {
		t.Run(name, func(t *testing.T) {
			env := newTestEnv(t)
			data := fixtureZIMBytes(t)
			served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
			target := downloadTargetFor(served, "de")
			mustMkdir(t, env.dir)
			if err := writeDownload(env.dir, &downloadFile{Target: target, URLs: env.kiwix.mirrorURLs(served)}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(env.dir, target.FileName)
			create(t, env, path, data)
			env.start()
			if _, err := os.Lstat(path); err != nil || fileExists(path+".part") {
				t.Fatalf("the entry was set aside: %v", err)
			}
			if status := env.manager.Status(); status.State != StateInterrupted {
				t.Fatalf("status = %+v", status)
			}
		})
	}
}

// The retired edition of the same name that a cancelled download unlisted from
// the pending deletes must not stay behind when another edition is installed.
func TestManagerStaleTargetCleanupRemovesTheUnlistedRetiredFile(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	german := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	english := env.kiwix.addEdition("wikipedia_en_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, e := range []*fakeEdition{german, english} {
		for _, mirror := range []string{"m1", "m2", "fallback"} {
			e.setMode(mirror, "slow")
		}
		defer e.unblock()
	}
	env.start()
	ctx := context.Background()

	retired := filepath.Join(env.dir, german.name+".zim")
	if err := os.WriteFile(retired, []byte("retired"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.manager.mu.Lock() // as if its deletion had failed earlier
	env.manager.state.PendingDelete = []string{german.name + ".zim"}
	env.manager.mu.Unlock()

	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatal(err)
	}
	env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	env.manager.mu.Lock()
	listed := slices.Contains(env.manager.state.PendingDelete, german.name+".zim")
	env.manager.mu.Unlock()
	if listed {
		t.Fatal("the target must be unlisted while its download runs")
	}
	if err := env.manager.Cancel(); err != nil {
		t.Fatal(err)
	}

	settings := env.settings()
	settings.Language = "en"
	env.manager.Configure(settings)
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatalf("Install(en): %v", err)
	}
	env.waitFor("first bytes of the English edition", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	if fileExists(retired) || fileExists(retired+".part") {
		t.Fatal("the retired file of the abandoned download was left behind")
	}
}

// The code of a failed load is not shown while an operation runs, and the
// status never looks readable when the installed edition could not be opened.
func TestManagerStatusReadableAndUnreadableEdition(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	for _, mirror := range []string{"m1", "m2", "fallback"} {
		served.setMode(mirror, "slow")
	}
	defer served.unblock()
	env.start()
	if status := env.manager.Status(); status.State != StateReady || !status.Readable {
		t.Fatalf("healthy edition: %+v", status)
	}
	restartedUnreadable := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := env.manager.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
			t.Fatal(err)
		}
		env.manager = env.newManager()
		env.manager.Configure(env.settings())
		env.start()
	}
	restartedUnreadable()
	if status := env.manager.Status(); status.State != StateError || status.Readable || status.ErrorCode != CodeZIMUnreadable {
		t.Fatalf("unreadable edition: %+v", status)
	}

	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatal(err)
	}
	status := env.waitFor("first bytes", func(s Status) bool { return s.OperationInProgress && s.BytesDone >= 1024 })
	if status.ErrorCode != "" || status.Readable {
		t.Fatalf("while downloading: %+v", status)
	}
	if err := env.manager.Cancel(); err != nil {
		t.Fatal(err)
	}
	status = env.manager.Status()
	if status.State != StateInterrupted || status.Edition == nil || status.Readable || status.ErrorCode != CodeZIMUnreadable ||
		!strings.Contains(status.Recommendation, "Delete the edition") {
		t.Fatalf("after Cancel the unreadable edition must be reported as such: %+v", status)
	}
	if _, _, ok := env.manager.Acquire(); ok {
		t.Fatal("Acquire succeeded for an unreadable edition")
	}
}

func TestManagerStatusReadableDuringUpdateAndAfterFailedUpdate(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	served := env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", fixtureZIMBytes(t))
	served.sha256 = strings.Repeat("0", 64)
	env.start()
	if err := env.manager.Install(context.Background(), InstallRequest{}); err != nil {
		t.Fatal(err)
	}
	status := env.waitFor("the failed update", func(s Status) bool { return !s.OperationInProgress && s.ErrorCode == CodeChecksumMismatch })
	if status.State != StateReady || !status.Readable {
		t.Fatalf("after a failed update: %+v", status)
	}
}

// discardPending must leave everything alone unless the manager is idle.
func TestManagerDiscardPendingWaitsForAnIdleManager(t *testing.T) {
	env := newTestEnv(t)
	installed := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
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

	m.mu.Lock()
	m.op = &operation{cancel: func() {}, done: make(chan struct{})} // a download is running
	m.mu.Unlock()
	m.discardPending(env.dir, pending)
	if !fileExists(part) || !fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("files of a running operation were removed")
	}
	m.mu.Lock()
	still := m.interrupted
	m.op = nil
	m.mu.Unlock()
	if !still {
		t.Fatal("the interrupted marker was cleared while an operation ran")
	}

	m.discardPending(env.dir, pending)
	if fileExists(part) || fileExists(filepath.Join(env.dir, downloadFileName)) || m.Status().State != StateReady {
		t.Fatal("an idle manager must discard the download")
	}
	if !fileExists(filepath.Join(env.dir, installed.FileName)) {
		t.Fatal("the installed edition was removed")
	}
}

func TestMergePendingDeletes(t *testing.T) {
	cases := []struct {
		name                     string
		current, snapshot, fails []string
		want                     []string
	}{
		{"deleted names leave the list", []string{"a", "b"}, []string{"a", "b"}, nil, nil},
		{"failed names stay", []string{"a", "b"}, []string{"a", "b"}, []string{"b"}, []string{"b"}},
		{"a name unlisted meanwhile is not listed again", []string{"a"}, []string{"a", "b"}, []string{"b"}, nil},
		{"names added meanwhile stay", []string{"a", "c"}, []string{"a"}, nil, []string{"c"}},
		{"order is kept", []string{"c", "a", "b"}, []string{"a", "b"}, []string{"a"}, []string{"c", "a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mergePendingDeletes(tc.current, tc.snapshot, tc.fails); !slices.Equal(got, tc.want) {
				t.Fatalf("mergePendingDeletes = %v, want %v", got, tc.want)
			}
		})
	}
}

// A file whose name a download took over is not deleted from a stale list.
func TestManagerDeleteListedChecksTheListing(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-10")
	env.start()
	name := "wikipedia_de_all_nopic_2026-09.zim"
	path := filepath.Join(env.dir, name)
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.manager.deleteListed(env.dir, name); err != nil || !fileExists(path) {
		t.Fatalf("an unlisted file was deleted: %v", err)
	}
	env.manager.mu.Lock()
	env.manager.state.PendingDelete = []string{name}
	env.manager.mu.Unlock()
	if err := env.manager.deleteListed(env.dir, name); err != nil || fileExists(path) {
		t.Fatalf("a listed file was not deleted: %v", err)
	}
	if !fileExists(filepath.Join(env.dir, edition.FileName)) {
		t.Fatal("the installed edition was removed")
	}
}
