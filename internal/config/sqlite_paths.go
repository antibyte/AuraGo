package config

import (
	"os"
	"path/filepath"
	"slices"
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
	KaggleDBFilename      = "kaggle.db"
)

// FlowsDBPath returns where the EasyDrag flow store lives: next to the Game Maker
// database, or "" when sqlite.game_maker_path is not set.
func FlowsDBPath(cfg *Config) string {
	if cfg == nil || cfg.SQLite.GameMakerPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfg.SQLite.GameMakerPath), FlowsDBFilename)
}

// SQLiteArchivePrefix contains logical database names in portable backups.
const SQLiteArchivePrefix = "data/.sqlite/"

// SQLiteArchivePaths maps stable archive names to files from the local configuration.
// Empty locations are omitted. A known location can be absent on a fresh install.
func SQLiteArchivePaths(cfg *Config) map[string]string {
	if cfg == nil {
		return nil
	}
	paths := map[string]string{
		"documents":              DocumentCorpusPath(cfg),
		"flows":                  FlowsDBPath(cfg),
		"short_term_path":        cfg.SQLite.ShortTermPath,
		"inventory_path":         cfg.SQLite.InventoryPath,
		"invasion_path":          cfg.SQLite.InvasionPath,
		"cheatsheet_path":        cfg.SQLite.CheatsheetPath,
		"image_gallery_path":     cfg.SQLite.ImageGalleryPath,
		"remote_control_path":    cfg.SQLite.RemoteControlPath,
		"media_registry_path":    cfg.SQLite.MediaRegistryPath,
		"homepage_registry_path": cfg.SQLite.HomepageRegistryPath,
		"contacts_path":          cfg.SQLite.ContactsPath,
		"planner_path":           cfg.SQLite.PlannerPath,
		"virtual_desktop_path":   cfg.SQLite.VirtualDesktopPath,
		"virtual_computers_path": cfg.SQLite.VirtualComputersPath,
		"site_monitor_path":      cfg.SQLite.SiteMonitorPath,
		"sql_connections_path":   cfg.SQLite.SQLConnectionsPath,
		"skills_path":            cfg.SQLite.SkillsPath,
		"knowledge_graph_path":   cfg.SQLite.KnowledgeGraphPath,
		"optimization_path":      cfg.SQLite.OptimizationPath,
		"prepared_missions_path": cfg.SQLite.PreparedMissionsPath,
		"mission_history_path":   cfg.SQLite.MissionHistoryPath,
		"push_path":              cfg.SQLite.PushPath,
		"launchpad_path":         cfg.SQLite.LaunchpadPath,
		"network_shares_path":    cfg.SQLite.NetworkSharesPath,
		"cyd_creator_path":       cfg.SQLite.CYDCreatorPath,
		"game_maker_path":        cfg.SQLite.GameMakerPath,
		"detective_path":         cfg.SQLite.DetectivePath,
		"long_term_path":         cfg.SQLite.LongTermPath,
	}
	if dataDir := cfg.Directories.DataDir; dataDir != "" {
		paths["system_tasks"] = filepath.Join(dataDir, SystemTasksDBFilename)
		paths["galaxa"] = filepath.Join(dataDir, GalaxaDBFilename)
		paths["desktop_store"] = filepath.Join(dataDir, DesktopStoreDBFilename)
		paths["tresor"] = filepath.Join(dataDir, TresorDBFilename)
		paths["retro_games"] = filepath.Join(dataDir, RetroGamesDirName, RetroGamesDBFilename)
		paths["mailer"] = filepath.Join(dataDir, "mailer", "mailer.db")
		paths["huggingface"] = filepath.Join(dataDir, HuggingFaceDBFilename)
		paths["kaggle"] = filepath.Join(dataDir, KaggleDBFilename)
		paths["manus"] = filepath.Join(dataDir, "manus.db")
		paths["sip_calls"] = filepath.Join(dataDir, "sip_calls.db")
		paths["docker_manager"] = filepath.Join(dataDir, "docker_manager.db")
		paths["meshcore"] = filepath.Join(dataDir, "meshcore.db")
		paths["personal_radio"] = filepath.Join(dataDir, "personal-radio", "radio.db")
		paths["rtl_sdr"] = filepath.Join(dataDir, "rtl-sdr", "receiver.db")
		paths["person_detection"] = filepath.Join(dataDir, "go2rtc", "detections.db")
		paths["bookshelf"] = filepath.Join(dataDir, "bookshelf", "catalog.sqlite")
		paths["findit"] = filepath.Join(dataDir, "findit.db")
	}
	if gm := cfg.SQLite.GameMakerPath; gm != "" {
		paths["detective_legacy"] = filepath.Join(filepath.Dir(gm), "detective.db")
		paths["newspaper"] = filepath.Join(filepath.Dir(gm), "newspaper.db")
	}
	archivePaths := make(map[string]string, len(paths))
	for key, path := range paths {
		if path != "" {
			archivePaths[SQLiteArchivePrefix+key+".db"] = path
		}
	}
	return archivePaths
}

// SQLiteDatabasePaths lists configured and existing legacy SQLite stores.
func SQLiteDatabasePaths(cfg *Config) []string {
	var paths []string
	for name, file := range SQLiteArchivePaths(cfg) {
		if name == SQLiteArchivePrefix+"long_term_path.db" || name == SQLiteArchivePrefix+"detective_legacy.db" {
			if _, err := os.Stat(file); err != nil {
				continue
			}
		}
		paths = append(paths, file)
	}
	slices.Sort(paths)
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
// is backed up only on a best-effort basis. The Hugging Face ledger holds
// transient job records that are recreated by running the jobs again.
// Paths are compared cleaned and absolute.
func IsBestEffortSQLiteBackup(cfg *Config, path string) bool {
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" || strings.TrimSpace(path) == "" {
		return false
	}
	got, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	want, err := filepath.Abs(filepath.Join(cfg.Directories.DataDir, HuggingFaceDBFilename))
	return err == nil && got == want
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
