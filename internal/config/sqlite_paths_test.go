package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteDatabasePathsIncludesConfiguredAndDataDirDatabases(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cfg := &Config{}
	cfg.Directories.DataDir = dataDir
	cfg.SQLite.ShortTermPath = filepath.Join(dataDir, "short_term.db")
	cfg.SQLite.LaunchpadPath = filepath.Join(dataDir, "launchpad.db")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(dataDir, "virtual_desktop.db")
	cfg.SQLite.NetworkSharesPath = filepath.Join(dataDir, "network_shares.db")
	cfg.SQLite.LongTermPath = filepath.Join(dataDir, "long_term.db")

	got := SQLiteDatabasePaths(cfg)
	want := map[string]bool{
		cfg.SQLite.ShortTermPath:                       true,
		cfg.SQLite.LaunchpadPath:                       true,
		cfg.SQLite.VirtualDesktopPath:                  true,
		cfg.SQLite.NetworkSharesPath:                   true,
		filepath.Join(dataDir, SystemTasksDBFilename):  true,
		filepath.Join(dataDir, GalaxaDBFilename):       true,
		filepath.Join(dataDir, DesktopStoreDBFilename): true,
	}

	for _, path := range got {
		if !want[path] && path != cfg.SQLite.LongTermPath {
			t.Fatalf("unexpected path %q in %v", path, got)
		}
		delete(want, path)
	}
	if len(want) != 0 {
		t.Fatalf("missing paths %v from %v", want, got)
	}
	if containsPath(got, cfg.SQLite.LongTermPath) {
		t.Fatalf("legacy long_term_path should be omitted when file does not exist: %v", got)
	}
}

func TestSQLiteDatabasePathsIncludesLegacyLongTermWhenFileExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	legacyPath := filepath.Join(dataDir, "long_term.db")
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &Config{}
	cfg.Directories.DataDir = dataDir
	cfg.SQLite.LongTermPath = legacyPath

	got := SQLiteDatabasePaths(cfg)
	if !containsPath(got, legacyPath) {
		t.Fatalf("expected legacy long_term.db in paths, got %v", got)
	}
}

func TestSQLiteProtectedPathsAddsWalAndShmSidecars(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "short_term.db")
	cfg := &Config{}
	cfg.SQLite.ShortTermPath = dbPath

	got := SQLiteProtectedPaths(cfg)
	want := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	if len(got) != len(want) {
		t.Fatalf("protected paths = %v, want %v", got, want)
	}
	for i, path := range want {
		if got[i] != path {
			t.Fatalf("protected[%d] = %q, want %q (full=%v)", i, got[i], path, got)
		}
	}
}

func containsPath(paths []string, target string) bool {
	target = filepath.Clean(target)
	for _, p := range paths {
		if filepath.Clean(p) == target {
			return true
		}
	}
	return false
}
