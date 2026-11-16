package main

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/restorepending"
	"aurago/internal/testutil"
	"github.com/gofrs/flock"
)

func stageStartupRestore(t *testing.T, root string, files map[string]string) {
	t.Helper()
	staging := t.TempDir()
	var entries []restorepending.Entry
	for rel, content := range files {
		dest := restorepending.StageFilePath(staging, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, restorepending.Entry{Clean: rel, Mode: 0600, SQLite: strings.HasSuffix(rel, ".db") || strings.HasSuffix(rel, ".sqlite")})
	}
	if _, err := restorepending.Stage(root, staging, entries); err != nil {
		t.Fatal(err)
	}
}

func TestStartupRestoreSharedDataLockPreservesEveryFile(t *testing.T) {
	first, second, shared := t.TempDir(), t.TempDir(), t.TempDir()
	cfgText := "directories:\n  vectordb_dir: " + filepath.ToSlash(shared) + "\nsqlite:\n  game_maker_path: " + filepath.ToSlash(filepath.Join(shared, "game.db")) + "\n"
	for _, root := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(cfgText), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadReadOnly(filepath.Join(first, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	running := make(dataDirectoryLocks)
	defer running.close()
	if err := running.acquire(configuredDataDirectories(cfg)...); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		p := filepath.Join(shared, "documents.sqlite"+suffix)
		files[p] = []byte("original" + suffix)
		if err := os.WriteFile(p, files[p], 0600); err != nil {
			t.Fatal(err)
		}
	}
	stageStartupRestore(t, second, map[string]string{"data/vectordb/documents.sqlite": "replacement"})
	pending := restorepending.StagedPath(second, "data/vectordb/documents.sqlite")
	files[pending] = []byte("replacement")
	before := restorepending.ReadState(second)
	locks := make(dataDirectoryLocks)
	defer locks.close()
	if _, err := prepareStartupRestore(filepath.Join(second, "config.yaml"), true, locks, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("occupied data directory accepted")
	}
	after := restorepending.ReadState(second)
	if !after.Pending || after.PendingID != before.PendingID || after.Last != nil {
		t.Fatalf("pending restore changed: %+v", after)
	}
	for file, want := range files {
		got, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("changed %s: %q, %v", file, got, err)
		}
	}
}

func TestStartupRestoreLocksNewConfigAndRetainsOldLocks(t *testing.T) {
	root, old, next := t.TempDir(), t.TempDir(), t.TempDir()
	cfgFile := filepath.Join(root, "config.yaml")
	cfgText := func(dir string) string {
		return "sqlite:\n  game_maker_path: " + filepath.ToSlash(filepath.Join(dir, "game.db")) + "\n"
	}
	if err := os.WriteFile(cfgFile, []byte(cfgText(old)), 0600); err != nil {
		t.Fatal(err)
	}
	stageStartupRestore(t, root, map[string]string{"config.yaml": cfgText(next)})
	locks := make(dataDirectoryLocks)
	defer locks.close()
	if _, err := prepareStartupRestore(cfgFile, true, locks, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{old, next, filepath.Join(root, "data")} {
		contender := flock.New(filepath.Join(dir, "aurago-data.lock"))
		ok, err := contender.TryLock()
		contender.Close()
		if err != nil || ok {
			t.Fatalf("directory lock not retained: %s, %v", dir, err)
		}
	}
	if restorepending.ReadState(root).Pending {
		t.Fatal("successful restore still pending")
	}
}

func TestDataDirectoryLocksDeduplicateAliases(t *testing.T) {
	root, target := t.TempDir(), t.TempDir()
	link := filepath.Join(root, "alias")
	testutil.DirectoryLink(t, link, target)
	locks := make(dataDirectoryLocks)
	defer locks.close()
	if err := locks.acquire(target, link, target); err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 {
		t.Fatalf("duplicate locks: %d", len(locks))
	}
}

func TestStartupRestoreInvalidConfigRollsBackUnderLocks(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "config.yaml")
	original := []byte("sqlite:\n  game_maker_path: ./data/game.db\n")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	stageStartupRestore(t, root, map[string]string{"config.yaml": "sqlite: ["})
	locks := make(dataDirectoryLocks)
	defer locks.close()
	if _, err := prepareStartupRestore(file, true, locks, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("configuration not rolled back: %q, %v", got, err)
	}
	state := restorepending.ReadState(root)
	if state.Pending || state.Last == nil || state.Last.Status != restorepending.StatusRolledBack {
		t.Fatalf("restore result: %+v", state)
	}
	backups, err := filepath.Glob(file + ".corrupted.*")
	if err != nil || len(backups) != 0 {
		t.Fatalf("validation wrote diagnostics: %v, %v", backups, err)
	}
}

func TestStartupRestoreRefusesNewConfigDataLockBeforeOpeningDatabases(t *testing.T) {
	root, shared := t.TempDir(), t.TempDir()
	file := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(file, []byte("sqlite:\n  game_maker_path: ./data/game.db\n"), 0600); err != nil {
		t.Fatal(err)
	}
	running := make(dataDirectoryLocks)
	defer running.close()
	if err := running.acquire(shared); err != nil {
		t.Fatal(err)
	}
	stageStartupRestore(t, root, map[string]string{"config.yaml": "sqlite:\n  game_maker_path: " + filepath.ToSlash(filepath.Join(shared, "game.db")) + "\n"})
	locks := make(dataDirectoryLocks)
	defer locks.close()
	if _, err := prepareStartupRestore(file, true, locks, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("new configuration's occupied directory accepted")
	}
	if _, err := os.Stat(filepath.Join(shared, "game.db")); !os.IsNotExist(err) {
		t.Fatalf("database opened before lock acquisition: %v", err)
	}
	state := restorepending.ReadState(root)
	if state.Last == nil || state.Last.Status != restorepending.StatusApplied {
		t.Fatalf("configuration restore not recorded: %+v", state)
	}
}
