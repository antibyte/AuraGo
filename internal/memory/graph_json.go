package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"strconv"
	"strings"
)

const legacyImportReadLimit = 64 << 20

func (kg *KnowledgeGraph) migrateFromJSON(jsonPath string) error {
	info, err := os.Lstat(jsonPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy graph: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("legacy graph is not a regular file")
	}
	if info.Size() > legacyImportReadLimit {
		return fmt.Errorf("legacy graph exceeds 64 MiB read limit")
	}
	f, err := os.Open(jsonPath)
	if err != nil {
		return fmt.Errorf("open legacy graph: %w", err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("legacy graph changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, legacyImportReadLimit+1))
	if err != nil {
		return fmt.Errorf("read legacy graph: %w", err)
	}
	if len(data) > legacyImportReadLimit {
		return fmt.Errorf("legacy graph exceeds 64 MiB read limit")
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close legacy graph: %w", err)
	}
	var state *struct {
		Nodes map[string]Node `json:"nodes"`
		Edges []Edge          `json:"edges"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parse legacy graph: %w", err)
	}
	if state == nil {
		return fmt.Errorf("legacy graph must be an object")
	}
	tx, err := kg.db.Begin()
	if err != nil {
		return fmt.Errorf("begin legacy graph import: %w", err)
	}
	defer tx.Rollback()
	for key, n := range state.Nodes {
		if strings.TrimSpace(n.ID) == "" || n.ID != strings.TrimSpace(n.ID) || key != n.ID {
			return fmt.Errorf("legacy graph contains an invalid node identity")
		}
		props, err := json.Marshal(n.Properties)
		if err != nil {
			return fmt.Errorf("encode legacy node properties: %w", err)
		}
		count := n.AccessCount
		if raw, ok := n.Properties["access_count"]; ok {
			count, err = strconv.Atoi(raw)
			if err != nil {
				return fmt.Errorf("invalid legacy node access count")
			}
		}
		if count < 0 {
			return fmt.Errorf("invalid legacy node access count")
		}
		protected := n.Protected || n.Properties["protected"] == "true"
		var label, existingProps string
		var existingCount int
		var existingProtected bool
		err = tx.QueryRow("SELECT label,properties,access_count,protected FROM kg_nodes WHERE id=?", n.ID).Scan(&label, &existingProps, &existingCount, &existingProtected)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec("INSERT INTO kg_nodes(id,label,properties,access_count,protected) VALUES(?,?,?,?,?)", n.ID, n.Label, string(props), count, protected); err != nil {
				return fmt.Errorf("insert legacy node: %w", err)
			}
		case err != nil:
			return fmt.Errorf("read existing legacy node: %w", err)
		default:
			if label != n.Label || existingCount != count || existingProtected != protected || !legacyGraphPropertiesEqual(existingProps, n.Properties) {
				return fmt.Errorf("legacy node conflicts with existing SQLite data")
			}
		}
	}
	for _, e := range state.Edges {
		for _, identity := range []string{e.Source, e.Target, e.Relation} {
			if strings.TrimSpace(identity) == "" || identity != strings.TrimSpace(identity) {
				return fmt.Errorf("legacy graph contains an invalid edge identity")
			}
		}
		for _, id := range []string{e.Source, e.Target} {
			var exists bool
			if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM kg_nodes WHERE id=?)", id).Scan(&exists); err != nil {
				return fmt.Errorf("check legacy edge endpoint: %w", err)
			}
			if !exists {
				return fmt.Errorf("legacy graph edge refers to a missing node")
			}
		}
		props, err := json.Marshal(e.Properties)
		if err != nil {
			return fmt.Errorf("encode legacy edge properties: %w", err)
		}
		var existingProps string
		err = tx.QueryRow("SELECT properties FROM kg_edges WHERE source=? AND target=? AND relation=?", e.Source, e.Target, e.Relation).Scan(&existingProps)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec("INSERT INTO kg_edges(source,target,relation,properties) VALUES(?,?,?,?)", e.Source, e.Target, e.Relation, string(props)); err != nil {
				return fmt.Errorf("insert legacy edge: %w", err)
			}
		case err != nil:
			return fmt.Errorf("read existing legacy edge: %w", err)
		default:
			if !legacyGraphPropertiesEqual(existingProps, e.Properties) {
				return fmt.Errorf("legacy edge conflicts with existing SQLite data")
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy graph import: %w", err)
	}
	// A create-only hard link preserves earlier archives, including concurrent ones.
	archive := jsonPath + ".migrated"
	for {
		if err := os.Link(jsonPath, archive); os.IsExist(err) {
			archive = jsonPath + ".migrated-" + rand.Text()
			continue
		} else if err != nil {
			return fmt.Errorf("archive imported graph: %w", err)
		}
		break
	}
	if err := os.Remove(jsonPath); err != nil {
		return fmt.Errorf("remove archived graph source: %w", err)
	}
	if kg.logger != nil {
		kg.logger.Info("[KG] Migrated graph.json to SQLite", "nodes", len(state.Nodes), "edges", len(state.Edges))
	}
	return nil
}

func legacyGraphPropertiesEqual(encoded string, properties map[string]string) bool {
	var existing map[string]string
	return json.Unmarshal([]byte(encoded), &existing) == nil && maps.Equal(existing, properties)
}
