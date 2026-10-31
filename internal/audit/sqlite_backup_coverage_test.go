package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/config"
)

// sqliteBackupExclusions lists SQLite filenames that production code opens under
// a data directory but that are deliberately NOT part of config.SQLiteDatabasePaths
// (and therefore not in .ago backups). Every entry needs a reason. A new database
// must either be added to SQLiteDatabasePaths or to this list with a reason.
var sqliteBackupExclusions = map[string]string{
	"workspace_search.db":    "derived file-search index, rebuilt from the workspace",
	"documents.sqlite":       "document corpus; backed up through DocumentCorpusPath (coordinated memory snapshot)",
	"short_term.sqlite":      "memory snapshot file name inside snapshot directories, not a live store",
	"knowledge_graph.sqlite": "memory snapshot file name inside snapshot directories, not a live store",
	"memory.sqlite":          "memory-repair tool scratch file",
	// Fallback defaults of sqlite.*_path settings: the configured path is what
	// SQLiteDatabasePaths backs up (config always fills these in).
	"image_gallery.db":   "default of sqlite.image_gallery_path",
	"virtual_desktop.db": "default of sqlite.virtual_desktop_path",
	"media_registry.db":  "default of sqlite.media_registry_path",
}

var sqliteLiteralJoin = regexp.MustCompile(`filepath\.Join\([^()\n]*"([A-Za-z0-9_.-]+\.(?:db|sqlite))"\s*\)`)
var sqliteNamedConst = regexp.MustCompile(`(?m)^\s*[A-Za-z0-9_]*(?:DB|Db|db)[A-Za-z0-9_]*\s*=\s*"([A-Za-z0-9_.-]+\.(?:db|sqlite))"`)

// TestSQLiteOpensAreBackedUpOrExcluded fails when production code joins a literal
// *.db / *.sqlite filename onto a directory (or declares it as a *DB* constant)
// and the name is neither produced by config.SQLiteDatabasePaths nor listed in
// sqliteBackupExclusions.
func TestSQLiteOpensAreBackedUpOrExcluded(t *testing.T) {
	cfg := &config.Config{}
	dataDir := filepath.Join(t.TempDir(), "data")
	cfg.Directories.DataDir = dataDir
	cfg.Directories.VectorDBDir = filepath.Join(dataDir, "vectordb")
	cfg.SQLite.GameMakerPath = filepath.Join(dataDir, "game_maker.db")
	cfg.SQLite.DetectivePath = filepath.Join(dataDir, "detective.db")
	cfg.SQLite.ShortTermPath = filepath.Join(dataDir, "short_term.db")
	covered := map[string]bool{}
	for _, p := range config.SQLiteDatabasePaths(cfg) {
		covered[filepath.Base(p)] = true
	}
	for _, name := range []string{"detective.db", "newspaper.db", "mailer.db", "manus.db", "sip_calls.db", "huggingface.db"} {
		if !covered[name] {
			t.Fatalf("SQLiteDatabasePaths must include %s", name)
		}
	}

	repo := filepath.Join("..", "..")
	offenders := map[string][]string{}
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(repo, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, re := range []*regexp.Regexp{sqliteLiteralJoin, sqliteNamedConst} {
				for _, m := range re.FindAllStringSubmatch(string(data), -1) {
					name := m[1]
					if covered[name] {
						continue
					}
					if _, ok := sqliteBackupExclusions[name]; ok {
						continue
					}
					offenders[name] = append(offenders[name], filepath.ToSlash(path))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", top, err)
		}
	}
	for name, files := range offenders {
		t.Errorf("SQLite file %q (in %s) is neither in config.SQLiteDatabasePaths nor in sqliteBackupExclusions", name, strings.Join(files, ", "))
	}
}
