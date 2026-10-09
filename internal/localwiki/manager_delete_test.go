package localwiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestManagerUpdateSwapsLibraryWhileReadersFinish(t *testing.T) {
	env := newTestEnv(t)
	data := fixtureZIMBytes(t)
	old := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.start()
	ctx := context.Background()

	const readers = 4
	stop := make(chan struct{})
	var wg sync.WaitGroup
	releases := make(chan func(), readers)
	for i := 0; i < readers; i++ {
		lib, release, ok := env.manager.Acquire()
		if !ok {
			t.Fatal("Acquire failed before the update")
		}
		releases <- release
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if lib.Edition().Name != old.Name || !lib.Fulltext() {
						t.Error("an in-flight reader saw the new edition")
						return
					}
				}
			}
		}()
	}

	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
	if err := env.manager.CheckUpdate(ctx); err != nil || env.manager.Status().UpdateAvailable == nil {
		t.Fatalf("CheckUpdate: %v", err)
	}
	if err := env.manager.Install(ctx, InstallRequest{}); err != nil {
		t.Fatalf("update Install: %v", err)
	}
	status := env.waitFor("new edition", func(s Status) bool {
		return !s.OperationInProgress && s.Edition != nil && s.Edition.Name == "wikipedia_de_all_nopic_2026-10"
	})
	if status.UpdateAvailable != nil || status.State != StateReady {
		t.Fatalf("status after update = %+v", status)
	}
	oldPath := filepath.Join(env.dir, old.FileName)
	if !fileExists(oldPath) {
		t.Fatal("the old edition was deleted while readers still used it")
	}
	lib, release, ok := env.manager.Acquire()
	if !ok || lib.Edition().Name != "wikipedia_de_all_nopic_2026-10" {
		t.Fatal("new requests do not get the new edition")
	}
	release()

	close(stop)
	wg.Wait()
	close(releases)
	for release := range releases {
		release()
	}
	if fileExists(oldPath) {
		t.Fatal("the old edition was not deleted after its last reader released it")
	}
	st, err := readState(env.dir)
	if err != nil || len(st.PendingDelete) != 0 || st.Edition.Name != "wikipedia_de_all_nopic_2026-10" {
		t.Fatalf("state.json after update = %+v, %v", st, err)
	}
}

func TestManagerDeleteOldFirstWhenOnlyOneEditionFits(t *testing.T) {
	env := newTestEnv(t)
	data := fixtureZIMBytes(t)
	old := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
	env.start()
	ctx := context.Background()
	env.free.Store(1<<30 + 10) // fits only after the old edition is deleted

	var space *InsufficientSpaceError
	if err := env.manager.Install(ctx, InstallRequest{}); !errors.As(err, &space) || !space.CanDeleteOld {
		t.Fatalf("Install(keep_old) = %v (%+v)", err, space)
	}
	if _, release, ok := env.manager.Acquire(); !ok {
		t.Fatal("a refused install must leave the installed edition online")
	} else {
		release()
	}
	if err := env.manager.Install(ctx, InstallRequest{ReplaceMode: ReplaceDeleteOldFirst}); err != nil {
		t.Fatalf("Install(delete_old_first) = %v", err)
	}
	status := env.waitFor("new edition", func(s Status) bool {
		return !s.OperationInProgress && s.Edition != nil && s.Edition.Name == "wikipedia_de_all_nopic_2026-10"
	})
	if status.State != StateReady || fileExists(filepath.Join(env.dir, old.FileName)) {
		t.Fatalf("old edition still present or not ready: %+v", status)
	}
}

func TestManagerDeleteOldFirstNeedsSpaceWithoutTheOldEdition(t *testing.T) {
	env := newTestEnv(t)
	data := fixtureZIMBytes(t)
	old := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	env.kiwix.addEdition("wikipedia_de_all_nopic_2026-10", data)
	env.start()
	env.free.Store(1 << 20) // too little even once the small old edition is gone

	var space *InsufficientSpaceError
	err := env.manager.Install(context.Background(), InstallRequest{ReplaceMode: ReplaceDeleteOldFirst})
	if !errors.As(err, &space) || space.CanDeleteOld {
		t.Fatalf("Install(delete_old_first) = %v (%+v)", err, space)
	}
	if !fileExists(filepath.Join(env.dir, old.FileName)) {
		t.Fatal("a refused install must not delete the installed edition")
	}
	if _, release, ok := env.manager.Acquire(); !ok {
		t.Fatal("a refused install must leave the installed edition online")
	} else {
		release()
	}
}

func TestManagerDeleteRemovesEditionPartAndState(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	target := Edition{
		Language: "de", Variant: VariantNoPic, Name: "wikipedia_de_all_nopic_2026-10", FileName: "wikipedia_de_all_nopic_2026-10.zim",
		Size: 10, SHA256: strings.Repeat("ab", 32),
	}
	part := filepath.Join(env.dir, target.FileName+".part")
	if err := os.WriteFile(part, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(env.dir, "wikipedia_en_all_maxi_2026-01.zim.part")
	foreign := filepath.Join(env.dir, "notes.part")
	for _, path := range []string{stray, foreign} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeDownload(env.dir, &downloadFile{Target: target}); err != nil {
		t.Fatal(err)
	}
	env.start()
	lib, release, ok := env.manager.Acquire()
	if !ok {
		t.Fatal("Acquire failed")
	}
	if err := env.manager.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	status := env.manager.Status()
	if status.State != StateNotInstalled || status.Edition != nil || status.ErrorCode != "" {
		t.Fatalf("status after delete = %+v", status)
	}
	if _, _, ok := env.manager.Acquire(); ok {
		t.Fatal("Acquire succeeded after Delete")
	}
	if fileExists(part) || fileExists(stray) || fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("partial download not removed")
	}
	if !fileExists(foreign) {
		t.Fatal("Delete removed a file that is not a partial edition")
	}
	_ = lib.Edition()
	release()
	if fileExists(filepath.Join(env.dir, edition.FileName)) || fileExists(filepath.Join(env.dir, stateFileName)) {
		t.Fatal("edition or state.json left behind after the last reader released it")
	}
	if err := env.manager.Delete(); err != nil {
		t.Fatalf("Delete without an edition = %v", err)
	}
}

func TestManagerDeleteClearsAnUnreadableEdition(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
	if err := os.WriteFile(filepath.Join(env.dir, edition.FileName), []byte("not a zim"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.start()
	if status := env.manager.Status(); status.State != StateError {
		t.Fatalf("status = %+v", status)
	}
	if err := env.manager.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	status := env.manager.Status()
	if status.State != StateNotInstalled || status.ErrorCode != "" || status.Edition != nil {
		t.Fatalf("status after delete = %+v", status)
	}
	if fileExists(filepath.Join(env.dir, edition.FileName)) || fileExists(filepath.Join(env.dir, stateFileName)) {
		t.Fatal("the unreadable edition was not removed")
	}
}

func TestManagerDeleteIsRefusedWhileADownloadRuns(t *testing.T) {
	env := newTestEnv(t)
	placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-09")
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
	if err := env.manager.Delete(); !errors.Is(err, ErrBusy) {
		t.Fatalf("Delete during a download = %v, want ErrBusy", err)
	}
	if _, release, ok := env.manager.Acquire(); !ok {
		t.Fatal("the edition went offline during the update")
	} else {
		release()
	}
	if err := env.manager.Cancel(); err != nil {
		t.Fatal(err)
	}
	// The old edition stays online; Delete then removes it together with the
	// partial download.
	if status := env.manager.Status(); status.State != StateInterrupted || status.Edition == nil {
		t.Fatalf("status after cancel = %+v", status)
	}
	if err := env.manager.Delete(); err != nil {
		t.Fatalf("Delete after cancel: %v", err)
	}
	if fileExists(filepath.Join(env.dir, served.name+".zim.part")) || fileExists(filepath.Join(env.dir, downloadFileName)) {
		t.Fatal("partial download not removed")
	}
	if status := env.manager.Status(); status.State != StateNotInstalled || status.Edition != nil {
		t.Fatalf("status after delete = %+v", status)
	}
}

// An edition file that cannot be deleted yet (on Windows: still held open)
// stays listed in state.json and is deleted by a later retry.
func TestManagerRetriesPendingDeletes(t *testing.T) {
	env := newTestEnv(t)
	edition := placeEdition(t, env.dir, "de", "wikipedia_de_all_nopic_2026-10")
	stuck := filepath.Join(env.dir, "wikipedia_de_all_nopic_2026-09.zim")
	if err := os.MkdirAll(stuck, 0o755); err != nil { // a non-empty directory cannot be removed as a file
		t.Fatal(err)
	}
	inner := filepath.Join(stuck, "keep")
	if err := os.WriteFile(inner, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeState(env.dir, &stateFile{Edition: &edition, PendingDelete: []string{"wikipedia_de_all_nopic_2026-09.zim"}}); err != nil {
		t.Fatal(err)
	}
	env.start()
	if st, err := readState(env.dir); err != nil || len(st.PendingDelete) != 1 {
		t.Fatalf("the undeletable file must stay listed: %+v, %v", st, err)
	}
	if err := os.Remove(inner); err != nil {
		t.Fatal(err)
	}
	env.manager.retryPendingDeletes()
	if fileExists(stuck) {
		t.Fatal("the retry did not delete the file")
	}
	if st, err := readState(env.dir); err != nil || len(st.PendingDelete) != 0 || st.Edition == nil {
		t.Fatalf("state.json after the retry = %+v, %v", st, err)
	}
}
