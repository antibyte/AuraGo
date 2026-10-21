package memory

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestMemoryCorrectionPublicationMigrationRepairsLegacyState(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	var activeID string
	for _, item := range p.Items {
		if item.Reference.Store == "ltm" {
			activeID = item.ReplacementID
		}
	}
	pending, err := c.Preview(t.Context(), "chat", "later", "New preference", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "ltm", ID: activeID}, Operation: "replace", Value: "User prefers sparkling water"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if err = c.stageCorrectionVector(t.Context(), pending, &pending.Items[0]); err != nil {
		t.Fatal(err)
	}
	// Recreate the first version's stranded profile lock and prematurely confirmed vector.
	if _, err = c.Memory.db.Exec(`DROP TRIGGER correction_vector_insert; DROP TRIGGER correction_vector_update;
		DROP TRIGGER correction_vector_delete; DROP TABLE memory_correction_vectors;
		DELETE FROM user_profile WHERE category='prefs' AND key='drink'`); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Memory.db.Exec(`PRAGMA user_version=11`); err != nil {
		t.Fatal(err)
	}
	replacement := pending.Items[0].ReplacementID
	if err = c.Memory.ApplyMemoryCurationAction(MemoryCurationAction{DocID: replacement, Action: "confirm"}, "admin", false); err != nil {
		t.Fatal(err)
	}
	var path string
	if err = c.Memory.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		t.Fatal(err)
	}
	if err = c.Memory.Close(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c.Memory, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Memory.Close()
	backup := correctionUpgradeBackup(t, path, "memory-correction-publication-v1")
	defer backup.Close()
	var status string
	if err = backup.QueryRow(`SELECT verification_status FROM memory_meta WHERE doc_id=?`, replacement).Scan(&status); err != nil || status != "confirmed" {
		t.Fatal("backup did not preserve the pre-migration state", status, err)
	}
	meta, err := c.Memory.GetMemoryMeta(replacement)
	if err != nil || !IsMemoryArchived(meta) {
		t.Fatal("migration exposed pending replacement", meta, err)
	}
	if err = c.Memory.ApplyMemoryCurationAction(MemoryCurationAction{DocID: replacement, Action: "unverify"}, "admin", false); err == nil {
		t.Fatal("restart lost publication guard")
	}
	if _, err = c.Apply(t.Context(), pending.ID, "chat"); err != nil {
		t.Fatal("pending operation could not resume after migration", err)
	}
	restored, err := c.Preview(t.Context(), "chat", "fresh", "Fresh user preference", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}, Operation: "replace", Value: "User prefers sparkling water"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal("legacy profile lock was not repaired", err)
	}
	if _, err = c.Apply(t.Context(), restored.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	if err = c.Memory.initCorrectionPublication(); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(path + ".memory-correction-publication-v1-*.bak")
	if len(backups) != 1 {
		t.Fatal("migration repeated", backups)
	}
}

func correctionUpgradeBackup(t *testing.T, path, label string) *sql.DB {
	t.Helper()
	backups, err := filepath.Glob(path + "." + label + "-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup files=%v, error=%v", backups, err)
	}
	db, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	var integrity string
	if err = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		db.Close()
		t.Fatal("inconsistent migration backup", integrity, err)
	}
	return db
}

func TestMemoryCorrectionGraphRevisionMigrationAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	g, err := NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: g, Access: CorrectionAccess{true, true, true}}
	if err = g.AddNode("host", "host", map[string]string{"type": "server", "version": "1", "os": "old"}); err != nil {
		t.Fatal(err)
	}
	preview := func(field, value string) *MemoryCorrection {
		t.Helper()
		p, err := c.Preview(t.Context(), "chat", "turn-"+value, "New observation", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "kg_node", ID: "host", Field: field}, Operation: "replace", Value: value}}, correctionTestEvidence())
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	legacy := preview("version", "9")
	var snapshot correctionSnapshot
	if err = json.Unmarshal(legacy.Items[0].Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Revision = "2026-01-01 00:00:00"
	legacy.Items[0].Snapshot, _ = json.Marshal(snapshot)
	if err = saveMemoryCorrection(t.Context(), c.Memory.db, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err = g.db.Exec(`DROP TRIGGER kg_correction_revision_insert; DROP TRIGGER kg_correction_revision_delete;
		DROP TRIGGER kg_correction_revision_update; DROP TABLE kg_correction_field_revisions`); err != nil {
		t.Fatal(err)
	}
	if _, err = g.db.Exec(`PRAGMA user_version=0`); err != nil {
		t.Fatal(err)
	}
	if err = g.Close(); err != nil {
		t.Fatal(err)
	}
	c.Graph, err = NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	backup := correctionUpgradeBackup(t, path, "memory-correction-revisions-v1")
	defer backup.Close()
	var count int
	if err = backup.QueryRow(`SELECT COUNT(*) FROM kg_nodes WHERE id='host'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("backup lost graph node", err)
	}
	if _, err = c.Apply(t.Context(), legacy.ID, "chat"); err == nil {
		t.Fatal("legacy timestamp preview inherited new revision authority")
	}
	version, os := preview("version", "2"), preview("os", "new")
	for _, p := range []*MemoryCorrection{os, version} {
		if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
			t.Fatal("independent property invalidated preview", err)
		}
	}
	stale := preview("version", "9")
	// Direct writers also advance revisions, even when updated_at does not change.
	if _, err = c.Graph.db.Exec(`UPDATE kg_nodes SET properties=json_set(properties,'$.version','3') WHERE id='host';
		UPDATE kg_nodes SET properties=json_set(properties,'$.version','2') WHERE id='host'`); err != nil {
		t.Fatal(err)
	}
	if err = c.Graph.Close(); err != nil {
		t.Fatal(err)
	}
	c.Graph, err = NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Graph.Close()
	if _, err = c.Apply(t.Context(), stale.ID, "chat"); err == nil {
		t.Fatal("restart lost revisions for reverted property")
	}
}
