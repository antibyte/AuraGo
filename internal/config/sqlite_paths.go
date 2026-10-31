package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Fixed SQLite database filenames stored under directories.data_dir but not
// listed in the sqlite config section.
const (
	SystemTasksDBFilename  = "system_tasks.db"
	GalaxaDBFilename       = "galaxa.db"
	DesktopStoreDBFilename = "desktop_store.db"
	TresorDBFilename       = "tresor.db"
	// FlowsDBFilename is the EasyDrag flow store. It has no sqlite setting of its own and
	// lives next to sqlite.game_maker_path (FlowsDBPath).
	FlowsDBFilename = "flows.db"
	// RetroGamesDirName/RetroGamesDBFilename locate the Retro Games index
	// (<data_dir>/retrogames/retrogames.db). Besides the rebuildable game index
	// it holds the user's favorites, plays and broken markers.
	RetroGamesDirName    = "retrogames"
	RetroGamesDBFilename = "retrogames.db"
	// HuggingFaceDBFilename is the Hugging Face job ledger (<data_dir>/huggingface.db).
	HuggingFaceDBFilename = "huggingface.db"
)

// FlowsDBPath returns where the EasyDrag flow store lives: next to the Game Maker
// database, or "" when sqlite.game_maker_path is not set.
func FlowsDBPath(cfg *Config) string {
	if cfg == nil || cfg.SQLite.GameMakerPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfg.SQLite.GameMakerPath), FlowsDBFilename)
}

// SQLiteDatabasePaths returns absolute SQLite database paths that should be
// included in backups. The canonical document corpus always participates;
// sqlite.long_term_path is legacy-only and included only while it exists.
func SQLiteDatabasePaths(cfg *Config) []string {
	if cfg == nil {
		return nil
	}

	paths := []string{
		DocumentCorpusPath(cfg),
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
		cfg.SQLite.VirtualComputersPath,
		cfg.SQLite.SiteMonitorPath,
		cfg.SQLite.SQLConnectionsPath,
		cfg.SQLite.SkillsPath,
		cfg.SQLite.KnowledgeGraphPath,
		cfg.SQLite.OptimizationPath,
		cfg.SQLite.PreparedMissionsPath,
		cfg.SQLite.MissionHistoryPath,
		cfg.SQLite.PushPath,
		cfg.SQLite.LaunchpadPath,
		cfg.SQLite.NetworkSharesPath,
		cfg.SQLite.CYDCreatorPath,
		FlowsDBPath(cfg),
	}

	if dataDir := cfg.Directories.DataDir; dataDir != "" {
		paths = append(paths,
			filepath.Join(dataDir, SystemTasksDBFilename),
			filepath.Join(dataDir, GalaxaDBFilename),
			filepath.Join(dataDir, DesktopStoreDBFilename),
			filepath.Join(dataDir, TresorDBFilename),
			filepath.Join(dataDir, RetroGamesDirName, RetroGamesDBFilename),
			// Stores whose location is fixed by their package (see the guard test
			// TestSQLiteOpensAreBackedUpOrExcluded for the full inclusion/exclusion list).
			filepath.Join(dataDir, "mailer", "mailer.db"),
			filepath.Join(dataDir, HuggingFaceDBFilename),
			filepath.Join(dataDir, "manus.db"),
			filepath.Join(dataDir, "sip_calls.db"),
			filepath.Join(dataDir, "docker_manager.db"),
			filepath.Join(dataDir, "meshcore.db"),
			filepath.Join(dataDir, "personal-radio", "radio.db"),
			filepath.Join(dataDir, "rtl-sdr", "receiver.db"),
			filepath.Join(dataDir, "go2rtc", "detections.db"),
			filepath.Join(dataDir, "bookshelf", "catalog.sqlite"),
		)
	}

	// Detective cases: the configured path, plus the pre-data-dir location next
	// to the Game Maker database that an upgraded install may still be using.
	paths = append(paths, cfg.SQLite.DetectivePath)
	if gm := cfg.SQLite.GameMakerPath; gm != "" {
		legacy := filepath.Join(filepath.Dir(gm), "detective.db")
		if _, err := os.Stat(legacy); err == nil {
			paths = append(paths, legacy)
		}
		// Newspaper editions live next to the Game Maker database, like flows.db.
		paths = append(paths, filepath.Join(filepath.Dir(gm), "newspaper.db"))
	}

	if legacy := cfg.SQLite.LongTermPath; legacy != "" {
		if _, err := os.Stat(legacy); err == nil {
			paths = append(paths, legacy)
		}
	}

	return dedupeNonEmptyPaths(paths)
}

// DocumentCorpusPath locates durable long-term content independently of its index.
func DocumentCorpusPath(cfg *Config) string {
	if cfg == nil || strings.TrimSpace(cfg.Directories.VectorDBDir) == "" {
		return ""
	}
	return filepath.Join(cfg.Directories.VectorDBDir, "documents.sqlite")
}

// IsBestEffortSQLiteBackup reports whether path (one of SQLiteDatabasePaths)
// is backed up only on a best-effort basis: a failed snapshot skips that file
// instead of failing the whole backup. Only rebuildable stores qualify:
//   - the Retro Games index: rebuildable and opened lazily, so a damaged file
//     can sit unnoticed until Retro Games is used again;
//   - the Hugging Face job ledger: transient job records that are re-created by
//     running the jobs again.
//
// Everything else holds user data and must fail the backup loudly.
// Paths are compared cleaned and absolute.
func IsBestEffortSQLiteBackup(cfg *Config, path string) bool {
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" || strings.TrimSpace(path) == "" {
		return false
	}
	got, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, rel := range [][]string{{RetroGamesDirName, RetroGamesDBFilename}, {HuggingFaceDBFilename}} {
		want, err := filepath.Abs(filepath.Join(append([]string{cfg.Directories.DataDir}, rel...)...))
		if err == nil && got == want {
			return true
		}
	}
	return false
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
