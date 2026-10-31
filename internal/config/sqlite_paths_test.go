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
	cfg.Directories.VectorDBDir = filepath.Join(root, "custom-vectors")

	got := SQLiteDatabasePaths(cfg)
	want := map[string]bool{
		filepath.Join(root, "custom-vectors", "documents.sqlite"): true,
		cfg.SQLite.ShortTermPath:                                  true,
		cfg.SQLite.LaunchpadPath:                                  true,
		cfg.SQLite.VirtualDesktopPath:                             true,
		cfg.SQLite.NetworkSharesPath:                              true,
		filepath.Join(dataDir, SystemTasksDBFilename):             true,
		filepath.Join(dataDir, GalaxaDBFilename):                  true,
		filepath.Join(dataDir, DesktopStoreDBFilename):            true,
		filepath.Join(dataDir, TresorDBFilename):                  true,
		// Retro Games keeps favorites, plays and broken markers in its index.
		filepath.Join(dataDir, "retrogames", "retrogames.db"): true,
		// Stores whose location is fixed by their package (audit B1).
		filepath.Join(dataDir, "mailer", "mailer.db"):         true,
		filepath.Join(dataDir, "huggingface.db"):              true,
		filepath.Join(dataDir, "manus.db"):                    true,
		filepath.Join(dataDir, "sip_calls.db"):                true,
		filepath.Join(dataDir, "docker_manager.db"):           true,
		filepath.Join(dataDir, "meshcore.db"):                 true,
		filepath.Join(dataDir, "personal-radio", "radio.db"):  true,
		filepath.Join(dataDir, "rtl-sdr", "receiver.db"):      true,
		filepath.Join(dataDir, "go2rtc", "detections.db"):     true,
		filepath.Join(dataDir, "bookshelf", "catalog.sqlite"): true,
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

func TestIsBestEffortSQLiteBackupOnlyForRebuildableStores(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cfg := &Config{}
	cfg.Directories.DataDir = dataDir
	cfg.SQLite.ShortTermPath = filepath.Join(dataDir, "short_term.db")

	index := filepath.Join(dataDir, "retrogames", "retrogames.db")
	if !IsBestEffortSQLiteBackup(cfg, index) {
		t.Fatalf("%s must be best-effort", index)
	}
	if !IsBestEffortSQLiteBackup(cfg, filepath.Join(dataDir, "retrogames", ".", "retrogames.db")) {
		t.Fatal("an uncleaned spelling of the index path must match")
	}
	hf := filepath.Join(dataDir, "huggingface.db")
	if !IsBestEffortSQLiteBackup(cfg, hf) {
		t.Fatalf("%s (job ledger) must be best-effort", hf)
	}
	for _, path := range append(SQLiteDatabasePaths(cfg), filepath.Join(dataDir, "retrogames.db"), filepath.Join(dataDir, "retrogames", "other.db"), "") {
		if path == index || path == hf {
			continue
		}
		if IsBestEffortSQLiteBackup(cfg, path) {
			t.Fatalf("%q must keep the fail-the-backup policy", path)
		}
	}
	if !containsPath(SQLiteDatabasePaths(cfg), index) {
		t.Fatalf("the best-effort index must still be backed up: %v", SQLiteDatabasePaths(cfg))
	}

	// A relative data_dir matches the absolute path the backup works with.
	rel := &Config{}
	rel.Directories.DataDir = filepath.Join("data", "dir")
	abs, err := filepath.Abs(filepath.Join("data", "dir", "retrogames", "retrogames.db"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsBestEffortSQLiteBackup(rel, abs) {
		t.Fatalf("relative data_dir: %s must be best-effort", abs)
	}

	if IsBestEffortSQLiteBackup(nil, index) {
		t.Fatal("nil config must match nothing")
	}
	if IsBestEffortSQLiteBackup(&Config{}, filepath.Join("retrogames", "retrogames.db")) {
		t.Fatal("without a data_dir nothing is best-effort")
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
