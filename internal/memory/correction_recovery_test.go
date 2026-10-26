package memory

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestMemoryCorrectionRestartAfterUnacknowledgedGraphCommit(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	open := func() *MemoryCorrectionService {
		t.Helper()
		stm, err := NewSQLiteMemory(filepath.Join(dir, "memory.db"), logger)
		if err != nil {
			t.Fatal(err)
		}
		kg, err := NewKnowledgeGraph(filepath.Join(dir, "graph.db"), "", logger)
		if err != nil {
			_ = stm.Close()
			t.Fatal(err)
		}
		return &MemoryCorrectionService{Memory: stm, Graph: kg, Access: CorrectionAccess{true, true, true}}
	}
	c := open()
	for _, id := range []string{"person", "coffee", "tea"} {
		if err := c.Graph.AddNode(id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Graph.AddEdge("person", "coffee", "prefers", nil); err != nil {
		t.Fatal(err)
	}
	ref := MemoryReference{Store: "kg_edge", ID: "person", Target: "coffee", Relation: "prefers"}
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: "tea"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	// Simulate termination after the graph's commit but before the coordinator's receipt.
	if _, err = c.Graph.applyCorrection(t.Context(), p.ID+":0", p, &p.Items[0]); err != nil {
		t.Fatal(err)
	}
	if err = c.Graph.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.Memory.Close(); err != nil {
		t.Fatal(err)
	}
	c = open()
	defer c.Memory.Close()
	defer c.Graph.Close()
	stored, err := c.Memory.MemoryCorrection(p.ID, "chat")
	if err != nil || stored.Items[0].State != "pending" {
		t.Fatal("expected unacknowledged coordinator state", err)
	}
	for range 2 {
		stored, err = c.Apply(t.Context(), p.ID, "chat")
		if err != nil || stored.Items[0].State != "done" || (stored.Status != "applied" && stored.Status != "applied_with_review") {
			t.Fatal("restart did not recover graph receipt", err)
		}
	}
	var claims int
	if err = c.Graph.db.QueryRow(`SELECT COUNT(*) FROM kg_claims WHERE subject_id='person' AND object_id='tea' AND status='accepted'`).Scan(&claims); err != nil || claims != 1 {
		t.Fatal("restart duplicated claims", claims, err)
	}
	if err = c.Graph.AddEdge("person", "coffee", "prefers", nil); err == nil {
		t.Fatal("restart lost withdrawn assertion guard")
	}
}

func TestMemoryCorrectionGraphMigrationBacksUpPopulatedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	g, err := NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	if err = g.AddNode("existing", "Existing fact", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = g.db.Exec(`DROP TABLE kg_correction_receipts; DROP TABLE kg_correction_guards`); err != nil {
		t.Fatal(err)
	}
	if _, err = g.db.Exec(`PRAGMA user_version=0`); err != nil {
		t.Fatal(err)
	}
	if err = g.Close(); err != nil {
		t.Fatal(err)
	}
	g, err = NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	backups, err := filepath.Glob(path + ".graph-schema-v2-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v %v", backups, err)
	}
	db, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var fact, integrity string
	if err = db.QueryRow(`SELECT label FROM kg_nodes WHERE id='existing'`).Scan(&fact); err != nil || fact != "Existing fact" {
		t.Fatal("backup lost graph", err)
	}
	if err = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal("inconsistent graph backup", err)
	}
}

func TestMemoryCorrectionToolEvidenceCannotAuthorizePersonalFact(t *testing.T) {
	for _, mode := range []string{"technical", "unclassified", "personal-value", "profile"} {
		t.Run(mode, func(t *testing.T) {
			c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: newTestKG(t), Access: CorrectionAccess{true, true, true}}
			properties := map[string]string{"type": "device", "os": "old OS"}
			if mode == "unclassified" {
				delete(properties, "type")
			}
			if err := c.Graph.AddNode("device", "device", properties); err != nil {
				t.Fatal(err)
			}
			change := MemoryCorrectionChange{Reference: MemoryReference{Store: "kg_node", ID: "device", Field: "os"}, Operation: "replace", Value: "new OS"}
			if mode == "personal-value" {
				change.Value = correctionNew
			}
			if mode == "profile" {
				if err := c.Memory.UpsertProfileEntry("prefs", "drink", "coffee", "analysis"); err != nil {
					t.Fatal(err)
				}
				change.Reference = MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}
				change.Value = "tea"
			}
			evidence := []CorrectionEvidence{{Reference: "tool:runtime", Kind: "tool", Quote: "new OS", ContentHash: "runtime-hash"}}
			p, err := c.Preview(t.Context(), "chat", "turn", "Checked device", []MemoryCorrectionChange{change}, evidence)
			if mode != "technical" {
				if err == nil {
					t.Fatal("tool evidence authorized personal or unclassified fact")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMemoryCorrectionAmbiguousCandidateRemainsUnchanged(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	other, err := StoreAutomaticMemoryDocument(c.Memory, c.Vector, "unrelated", "A visitor prefers black tea", MemoryMetaUpdate{VerificationStatus: "confirmed"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil || len(p.Review) != 1 || p.Review[0].ID != other.CreatedIDs[0] {
		t.Fatal("ambiguous candidate missing", err)
	}
	p, err = c.Apply(t.Context(), p.ID, "chat")
	if err != nil || p.Status != "applied_with_review" {
		t.Fatal("partial coverage not explicit", err)
	}
	meta, err := c.Memory.GetMemoryMeta(other.CreatedIDs[0])
	if err != nil || IsMemoryArchived(meta) {
		t.Fatal("similarity authorized unrelated mutation", err)
	}
}

func TestMemoryCorrectionRestoringProfileRespectsCapacity(t *testing.T) {
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Access: CorrectionAccess{true, true, true}}
	if err := c.Memory.UpsertProfileEntry("prefs", "drink", "coffee", "analysis"); err != nil {
		t.Fatal(err)
	}
	ref := MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}
	p, err := c.Preview(t.Context(), "chat", "turn", "Withdraw preference", []MemoryCorrectionChange{{Reference: ref, Operation: "archive"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	for i := range maxProfileEntriesPerCategory {
		if err = c.Memory.UpsertProfileEntry("prefs", fmt.Sprintf("item_%d", i), fmt.Sprintf("value %d", i), "analysis"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = c.Preview(t.Context(), "chat", "later", "New preference", []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: "tea"}}, correctionTestEvidence()); err == nil {
		t.Fatal("restoring correction bypassed profile capacity")
	}
}
