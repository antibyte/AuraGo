package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/fileutil"
	"aurago/internal/restorepending"

	"gopkg.in/yaml.v3"
)

func agoSQLiteURI(path, query string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: query}).String()
}

func restoreDestinationKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func agoCreateSQLiteSnapshot(ctx context.Context, src, tempDir string) (string, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("sqlite source is not a regular file")
	}
	hash := sha256.Sum256([]byte(src))
	dst := filepath.Join(tempDir, fmt.Sprintf("%x_%s", hash[:6], filepath.Base(src)))
	_ = os.Remove(dst)

	db, err := sql.Open("sqlite", agoSQLiteURI(src, "mode=ro&_pragma=busy_timeout%285000%29"))
	if err != nil {
		return "", fmt.Errorf("open sqlite for snapshot: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, `VACUUM main INTO ?`, dst); err != nil {
		return "", fmt.Errorf("create sqlite snapshot: %w", err)
	}
	if err := fileutil.RestrictToOwner(dst); err != nil {
		return "", fmt.Errorf("protect sqlite snapshot: %w", err)
	}
	if err := agoValidateSQLiteDB(dst); err != nil {
		return "", fmt.Errorf("validate sqlite snapshot: %w", err)
	}
	return dst, nil
}

// Preserve the active local path values used for the portable entries. The
// imported configuration may come from a machine with a different disk layout.
func agoKeepMappedSQLiteLocations(entries []agoImportEntry, dirs restorepending.Dirs) error {
	sqlitePaths := make(map[string]string)
	localDirs := make(map[string]string)
	for _, entry := range entries {
		archivePath := filepath.ToSlash(entry.clean)
		name, mapped := strings.CutPrefix(archivePath, config.SQLiteArchivePrefix)
		if !mapped {
			continue
		}
		switch {
		case strings.HasSuffix(name, "_path.db"):
			sqlitePaths[strings.TrimSuffix(name, ".db")] = entry.dest
		case name == "flows.db" || name == "newspaper.db" || name == "detective_legacy.db":
			sqlitePaths["game_maker_path"] = dirs.SQLite[config.SQLiteArchivePrefix+"game_maker_path.db"]
		case name == "documents.db":
			localDirs["vectordb_dir"] = dirs.VectorDB
		default:
			localDirs["data_dir"] = dirs.DataDir
		}
	}
	if len(sqlitePaths)+len(localDirs) == 0 {
		return nil
	}
	for _, entry := range entries {
		if entry.isDir || filepath.ToSlash(entry.clean) != "config.yaml" {
			continue
		}
		data, err := os.ReadFile(entry.stage)
		if err != nil {
			return err
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return err
		}
		if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return fmt.Errorf("configuration must be a YAML mapping")
		}
		for _, update := range []struct {
			section string
			values  map[string]string
		}{{"sqlite", sqlitePaths}, {"directories", localDirs}} {
			if len(update.values) == 0 {
				continue
			}
			section := agoConfigYAMLField(doc.Content[0], update.section)
			if section.Kind != yaml.MappingNode {
				return fmt.Errorf("configuration section %s must be a YAML mapping", update.section)
			}
			keys := make([]string, 0, len(update.values))
			for key := range update.values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				abs, err := filepath.Abs(update.values[key])
				if err != nil || update.values[key] == "" {
					return fmt.Errorf("missing local configuration path for %s.%s", update.section, key)
				}
				node := agoConfigYAMLField(section, key)
				*node = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: filepath.ToSlash(abs)}
			}
		}
		data, err = yaml.Marshal(&doc)
		if err != nil {
			return err
		}
		return config.WriteFileAtomic(entry.stage, data, 0o600)
	}
	return nil
}

func agoConfigYAMLField(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, node)
	return node
}

func (s *Server) initializedBackupDatabases(paths map[string]string) map[string]bool {
	required := make(map[string]bool)
	for name, initialized := range map[string]bool{
		"short_term_path":        s.ShortTermMem != nil,
		"knowledge_graph_path":   s.KG != nil,
		"inventory_path":         s.InventoryDB != nil,
		"invasion_path":          s.InvasionDB != nil,
		"cheatsheet_path":        s.CheatsheetDB != nil,
		"image_gallery_path":     s.ImageGalleryDB != nil,
		"media_registry_path":    s.MediaRegistryDB != nil,
		"homepage_registry_path": s.HomepageRegistryDB != nil,
		"contacts_path":          s.ContactsDB != nil,
		"planner_path":           s.PlannerDB != nil,
		"launchpad_path":         s.LaunchpadDB != nil,
		"sql_connections_path":   s.SQLConnectionsDB != nil,
		"skills_path":            s.SkillsDB != nil,
		"prepared_missions_path": s.PreparedMissionsDB != nil,
		"mission_history_path":   s.MissionHistoryDB != nil,
		"findit":                 s.findIt != nil && s.findIt.store != nil,
	} {
		if file := paths[config.SQLiteArchivePrefix+name+".db"]; initialized && file != "" {
			if abs, err := filepath.Abs(file); err == nil {
				required[restoreDestinationKey(abs)] = true
			}
		}
	}
	return required
}
