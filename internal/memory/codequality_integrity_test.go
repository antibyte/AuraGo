package memory

import (
	"fmt"
	"strings"
	"testing"
)

func TestGraphCleanupRollsBackEarlierDeletesOnSQLFailure(t *testing.T) {
	kg := newTestKG(t)
	if err := kg.AddEdge("one", "two", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("stale", "Stale", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET updated_at=datetime('now','-100 days'); UPDATE kg_edges SET updated_at=datetime('now','-100 days'); CREATE TRIGGER prevent_cleanup BEFORE DELETE ON kg_nodes BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := kg.CleanupStaleGraphWithOptions(KnowledgeGraphCleanupOptions{PendingCoMentionDays: 1, StaleNodeDays: 1}); err == nil {
		t.Fatal("expected delete failure")
	}
	var edges, nodes int
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_edges").Scan(&edges); err != nil {
		t.Fatal(err)
	}
	if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_nodes").Scan(&nodes); err != nil {
		t.Fatal(err)
	}
	if edges != 1 || nodes != 3 {
		t.Fatalf("partial cleanup committed: edges=%d nodes=%d", edges, nodes)
	}
}

func TestGraphMergeKeepsAcceptedSourceAndBothClaimHistories(t *testing.T) {
	for _, incoming := range []bool{false, true} {
		t.Run(fmt.Sprint(incoming), func(t *testing.T) {
			kg := newTestKG(t)
			for _, id := range []string{"source", "target", "peer"} {
				if err := kg.AddNode(id, id, nil); err != nil {
					t.Fatal(err)
				}
			}
			a, b, c, d := "source", "peer", "target", "peer"
			if incoming {
				a, b, c, d = "peer", "source", "peer", "target"
			}
			if _, err := kg.AddEdgeWithProvenance(c, d, "uses", nil, KGProvenanceInput{SourceKind: "manual"}); err != nil {
				t.Fatal(err)
			}
			if _, err := kg.db.Exec("UPDATE kg_edges SET status='retracted'; UPDATE kg_claims SET status='retracted'"); err != nil {
				t.Fatal(err)
			}
			claim, err := kg.AddEdgeWithProvenance(a, b, "uses", nil, KGProvenanceInput{SourceKind: "manual"})
			if err != nil {
				t.Fatal(err)
			}
			if err := kg.MergeNodes("target", "source"); err != nil {
				t.Fatal(err)
			}
			var status string
			if err := kg.db.QueryRow("SELECT status FROM kg_edges WHERE source=? AND target=?", c, d).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != "accepted" {
				t.Fatalf("accepted fact lost: %s", status)
			}
			var accepted, total int
			if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_claims WHERE id=? AND status='accepted'", claim.ID).Scan(&accepted); err != nil {
				t.Fatal(err)
			}
			if err := kg.db.QueryRow("SELECT COUNT(*) FROM kg_claims").Scan(&total); err != nil {
				t.Fatal(err)
			}
			if accepted != 1 || total != 2 {
				t.Fatalf("claim history lost: accepted=%d total=%d", accepted, total)
			}
		})
	}
}

func TestCanonicalRepairPreservesAnalysisEnvelope(t *testing.T) {
	stm := newTestNotesDB(t)
	if err := stm.UpsertMemoryMeta("original"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"original": "[fact] AuroraGo uses Go.\n\nsource:memory_analysis session:test-session"}}
	fake.storeOwned = func(concept, content string, mode VectorStoreMode) (VectorStoreResult, error) {
		fake.docs["repaired"] = concept + "\n\n" + content
		return VectorStoreResult{CreatedIDs: []string{"repaired"}}, nil
	}
	report, err := stm.RepairCanonicalMemoryNames(fake, CanonicalRepairOptions{})
	if err != nil || report.RepairedCount != 1 {
		t.Fatalf("repair: %+v %v", report, err)
	}
	for _, id := range report.Items[0].NewDocIDs {
		fact, source, ok := AnalysisDocumentParts(fake.docs[id])
		if !ok || !strings.Contains(fact, "AuraGo") || source != "source:memory_analysis session:test-session" {
			t.Fatalf("identity lost: %q", fake.docs[id])
		}
	}
}
