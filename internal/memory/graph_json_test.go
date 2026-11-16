package memory

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validLegacyGraphJSON = `{"nodes":{"a":{"id":"a","label":"A","properties":{}},"b":{"id":"b","properties":{}}},"edges":[{"source":"a","target":"b","relation":"knows","properties":{}}]}`

func TestLegacyGraphJSONRollsBackAndPreservesSource(t *testing.T) {
	for _, tc := range []struct{ name, data, trigger string }{
		{"null", "null", ""},
		{"malformed", "{", ""},
		{"missing id", `{"nodes":{"a":{"label":"A"}}}`, ""},
		{"mismatched id", `{"nodes":{"a":{"id":"b"}}}`, ""},
		{"missing endpoint", strings.Replace(validLegacyGraphJSON, `"target":"b"`, `"target":"missing"`, 1), ""},
		{"empty relation", strings.Replace(validLegacyGraphJSON, `"relation":"knows"`, `"relation":" "`, 1), ""},
		{"node sql", validLegacyGraphJSON, `CREATE TRIGGER reject_node BEFORE INSERT ON kg_nodes BEGIN SELECT RAISE(ABORT,'fixture node failure'); END`},
		{"edge sql", validLegacyGraphJSON, `CREATE TRIGGER reject_edge BEFORE INSERT ON kg_edges BEGIN SELECT RAISE(ABORT,'fixture edge failure'); END`},
		{"commit", validLegacyGraphJSON, `CREATE TABLE commit_guard(id TEXT REFERENCES kg_nodes(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER reject_commit AFTER INSERT ON kg_edges BEGIN INSERT INTO commit_guard VALUES('absent'); END`},
		{"existing conflict", `{"nodes":{"old":{"id":"old","label":"changed"},"a":{"id":"a"}}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kg := newTestKG(t)
			if _, err := kg.db.Exec(`INSERT INTO kg_nodes(id,label) VALUES('old','original')`); err != nil {
				t.Fatal(err)
			}
			if tc.trigger != "" {
				if _, err := kg.db.Exec(tc.trigger); err != nil {
					t.Fatal(err)
				}
			}
			source := filepath.Join(t.TempDir(), "graph.json")
			if err := os.WriteFile(source, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			if err := kg.migrateFromJSON(source); err == nil {
				t.Fatal("invalid import succeeded")
			}
			var nodes, edges int
			if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&nodes); err != nil {
				t.Fatal(err)
			}
			if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&edges); err != nil {
				t.Fatal(err)
			}
			if nodes != 1 || edges != 0 {
				t.Fatalf("partial import: %d nodes, %d edges", nodes, edges)
			}
			var label string
			if err := kg.db.QueryRow("SELECT label FROM kg_nodes WHERE id='old'").Scan(&label); err != nil || label != "original" {
				t.Fatalf("old graph changed: %q, %v", label, err)
			}
			got, err := os.ReadFile(source)
			if err != nil || string(got) != tc.data {
				t.Fatalf("source changed: %v", err)
			}
			if _, err := os.Stat(source + ".migrated"); !os.IsNotExist(err) {
				t.Fatalf("failed import archived source: %v", err)
			}
		})
	}
}

func TestLegacyGraphJSONRetryPreservesArchivesAndConflicts(t *testing.T) {
	kg := newTestKG(t)
	source := filepath.Join(t.TempDir(), "graph.json")
	if err := os.WriteFile(source+".migrated", []byte("earlier archive"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := os.WriteFile(source, []byte(validLegacyGraphJSON), 0600); err != nil {
			t.Fatal(err)
		}
		if err := kg.migrateFromJSON(source); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(source); !os.IsNotExist(err) {
			t.Fatalf("source not archived: %v", err)
		}
	}
	archives, err := filepath.Glob(source + ".migrated*")
	if err != nil || len(archives) != 3 {
		t.Fatalf("archives=%v, %v", archives, err)
	}
	got, err := os.ReadFile(source + ".migrated")
	if err != nil || string(got) != "earlier archive" {
		t.Fatal("existing archive changed")
	}
	nodes, edges, err := kg.Stats()
	if err != nil || nodes != 2 || edges != 1 {
		t.Fatalf("retry duplicated records: %d %d %v", nodes, edges, err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_edges SET properties='{"changed":"yes"}'`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(validLegacyGraphJSON), 0600); err != nil {
		t.Fatal(err)
	}
	if err := kg.migrateFromJSON(source); err == nil {
		t.Fatal("conflicting edge accepted")
	}
}

func TestLegacyGraphJSONLimitAndConstructorWarning(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "graph.json")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(legacyImportReadLimit + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	dbPath := filepath.Join(dir, "graph.db")
	previous, err := NewKnowledgeGraph(dbPath, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := previous.db.Exec(`INSERT INTO kg_nodes(id,label) VALUES('original','Keep this record')`); err != nil {
		t.Fatal(err)
	}
	previous.Close()
	kg, err := NewKnowledgeGraph(dbPath, source, logger)
	if err != nil {
		t.Fatalf("import failure stopped startup: %v", err)
	}
	defer kg.Close()
	if !strings.Contains(logs.String(), "WARN") || !strings.Contains(logs.String(), "64 MiB") {
		t.Fatalf("missing import warning: %s", logs.String())
	}
	info, err := os.Stat(source)
	if err != nil || info.Size() != legacyImportReadLimit+1 {
		t.Fatalf("oversized source changed: %v", err)
	}
	var label string
	if err := kg.db.QueryRow("SELECT label FROM kg_nodes WHERE id='original'").Scan(&label); err != nil || label != "Keep this record" {
		t.Fatalf("existing graph changed: %q, %v", label, err)
	}
	if _, err := kg.db.Exec(`INSERT INTO kg_nodes(id,label) VALUES('usable','Existing graph remains writable')`); err != nil {
		t.Fatal(err)
	}
}
