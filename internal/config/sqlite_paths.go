package config

import (
	"os"
	"path/filepath"
)

// Fixed SQLite database filenames stored under directories.data_dir but not
// listed in the sqlite config section.
const (
	SystemTasksDBFilename  = "system_tasks.db"
	GalaxaDBFilename       = "galaxa.db"
	DesktopStoreDBFilename = "desktop_store.db"
)

// SQLiteDatabasePaths returns absolute SQLite database paths that should be
// included in backups. Long-term memory lives in directories.vectordb_dir
// (chromem); sqlite.long_term_path is legacy-only and included only when the
// file still exists on disk.
func SQLiteDatabasePaths(cfg *Config) []string {
	if cfg == nil {
		return nil
	}

	paths := []string{
		cfg.SQLite.ShortTermPath,
		cfg.SQLite.InventoryPath,
		cfg.SQLite.InvasionPath,
		cfg.SQLite.CheatsheetPath,
		cfg.SQLite.ImageGalleryPath,
		cfg.SQLite.RemoteControlPath,
		cfg.SQLite.MediaRegistryPath,
		cfg.SQLite.HomepageRegistryPath,
		cfg.SQLite.ContactsPath,
		cfg.SQLite.PlannerPath,
		cfg.SQLite.VirtualDesktopPath,
		cfg.SQLite.SiteMonitorPath,
		cfg.SQLite.SQLConnectionsPath,
		cfg.SQLite.SkillsPath,
		cfg.SQLite.KnowledgeGraphPath,
		cfg.SQLite.OptimizationPath,
		cfg.SQLite.PreparedMissionsPath,
		cfg.SQLite.MissionHistoryPath,
		cfg.SQLite.PushPath,
		cfg.SQLite.LaunchpadPath,
	}

	if dataDir := cfg.Directories.DataDir; dataDir != "" {
		paths = append(paths,
			filepath.Join(dataDir, SystemTasksDBFilename),
			filepath.Join(dataDir, GalaxaDBFilename),
			filepath.Join(dataDir, DesktopStoreDBFilename),
		)
	}

	if legacy := cfg.SQLite.LongTermPath; legacy != "" {
		if _, err := os.Stat(legacy); err == nil {
			paths = append(paths, legacy)
		}
	}

	return dedupeNonEmptyPaths(paths)
}

// SQLiteProtectedPaths returns SQLite database paths plus WAL/SHM sidecars that
// the agent must never read or write via filesystem tools.
func SQLiteProtectedPaths(cfg *Config) []string {
	paths := SQLiteDatabasePaths(cfg)
	if len(paths) == 0 {
		return nil
	}

	protected := make([]string, 0, len(paths)*3)
	for _, dbPath := range paths {
		protected = append(protected, dbPath, dbPath+"-wal", dbPath+"-shm")
	}
	return dedupeNonEmptyPaths(protected)
}

func dedupeNonEmptyPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}