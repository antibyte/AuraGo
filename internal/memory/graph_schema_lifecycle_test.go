package memory

import (
	"database/sql"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestLegacyGraphMigrationPreservesEdgesClaimsAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.db")
	kg, err := NewKnowledgeGraph(path, "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO kg_nodes(id,label,properties,protected) VALUES ('a','A','{"type":"person"}',1),('b','B','{}',0)`,
		`INSERT INTO kg_edges(id,source,target,relation,properties,status,status_reason,access_count) VALUES (17,'a','b','knows','{"source":"user"}','retracted','kept',9)`,
		`INSERT INTO kg_evidence(id,raw_text) VALUES ('e','original evidence')`,
		`INSERT INTO kg_claims(id,subject_id,predicate,object_id,evidence_id) VALUES ('c','a','knows','b','e')`,
		`DROP INDEX idx_kg_nodes_type`, `DROP INDEX idx_kg_nodes_source`,
		`ALTER TABLE kg_nodes DROP COLUMN node_type`, `ALTER TABLE kg_nodes DROP COLUMN source_type`,
	} {
		if _, err := kg.db.Exec(stmt); err != nil {
			t.Fatal(stmt, err)
		}
	}
	kg.Close()
	kg, err = NewKnowledgeGraph(path, "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()
	var props, status, reason string
	var count int
	if err := kg.db.QueryRow(`SELECT properties,status,status_reason,access_count FROM kg_edges WHERE id=17`).Scan(&props, &status, &reason, &count); err != nil {
		t.Fatal(err)
	}
	if props != `{"source":"user"}` || status != "retracted" || reason != "kept" || count != 9 {
		t.Fatalf("edge changed: %s %s %s %d", props, status, reason, count)
	}
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_claims c JOIN kg_evidence e ON c.evidence_id=e.id WHERE c.id='c' AND e.raw_text='original evidence'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("claim/evidence changed: %d %v", count, err)
	}
	backups, _ := filepath.Glob(path + ".graph-schema-v2-*.bak")
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if err := backup.QueryRow(`SELECT count(*) FROM kg_edges WHERE id=17`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("backup lost edge: %d %v", count, err)
	}
	if _, err := kg.db.Exec(`DELETE FROM kg_nodes WHERE id='a'`); err != nil {
		t.Fatal(err)
	}
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_edges`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cascade broken: %d %v", count, err)
	}
}

func TestGraphMigrationForeignKeyFailureRollsBack(t *testing.T) {
	kg := newTestKG(t)
	err := kg.execKGMigrationTransaction([]string{`PRAGMA defer_foreign_keys=ON`, `INSERT INTO kg_nodes(id) VALUES ('kept-on-rollback')`, `INSERT INTO kg_edges(source,target,relation) VALUES ('kept-on-rollback','missing','x')`})
	if err == nil {
		t.Fatal("expected foreign key failure")
	}
	var count int
	if err := kg.db.QueryRow(`SELECT count(*) FROM kg_nodes WHERE id='kept-on-rollback'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("migration did not roll back: %d %v", count, err)
	}
}
