package memory

import (
	"database/sql"
	"testing"
)

func TestKnowledgeGraphInitializesProvenanceTables(t *testing.T) {
	kg := newTestKG(t)

	for _, table := range []string{"kg_claims", "kg_evidence", "kg_conflicts"} {
		t.Run(table, func(t *testing.T) {
			var name string
			err := kg.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
			if err != nil {
				t.Fatalf("table %s missing: %v", table, err)
			}
		})
	}
}

func TestKnowledgeGraphClaimCanOmitEvidence(t *testing.T) {
	kg := newTestKG(t)

	if _, err := kg.db.Exec(`
		INSERT INTO kg_claims (id, subject_id, predicate, object_id)
		VALUES (?, ?, ?, ?)
	`, "claim_without_evidence", "andi", "likes", "go"); err != nil {
		t.Fatalf("insert claim without evidence_id: %v", err)
	}

	var evidenceID sql.NullString
	if err := kg.db.QueryRow(`SELECT evidence_id FROM kg_claims WHERE id = ?`, "claim_without_evidence").Scan(&evidenceID); err != nil {
		t.Fatalf("query evidence_id: %v", err)
	}
	if evidenceID.Valid {
		t.Fatalf("evidence_id should be NULL when omitted, got %q", evidenceID.String)
	}
}

func TestAddEdgeWithProvenanceRecordsClaimAndEvidence(t *testing.T) {
	kg := newTestKG(t)

	claim, err := kg.AddEdgeWithProvenance("andi", "german", "prefers_language", nil, KGProvenanceInput{
		SourceKind: "user",
		SessionID:  "s1",
		Channel:    "web",
		RawText:    "Andi prefers German",
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}
	if claim == nil || claim.ID == "" {
		t.Fatalf("expected claim with ID, got %#v", claim)
	}

	claims, err := kg.GetClaimsForEdge("andi", "german", "prefers_language", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("claims len = %d, want 1: %#v", len(claims), claims)
	}
	got := claims[0]
	if got.Status != KGClaimAccepted || got.SourceKind != "user" || got.SessionID != "s1" || got.Confidence != 0.95 {
		t.Fatalf("claim fields = %#v", got)
	}
	if got.EvidenceID == "" || got.Evidence == nil {
		t.Fatalf("expected evidence to be linked, got %#v", got)
	}
	if got.Evidence.RawText != "Andi prefers German" || got.Evidence.Channel != "web" {
		t.Fatalf("evidence fields = %#v", got.Evidence)
	}

	var evidenceRows int
	if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_evidence`).Scan(&evidenceRows); err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	if evidenceRows != 1 {
		t.Fatalf("evidence rows = %d, want 1", evidenceRows)
	}
}

func TestAddEdgeWithProvenanceAllowsClaimWithoutEvidence(t *testing.T) {
	kg := newTestKG(t)

	claim, err := kg.AddEdgeWithProvenance("andi", "go", "likes", nil, KGProvenanceInput{
		SourceKind: "manual",
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}
	if claim == nil || claim.EvidenceID != "" || claim.Evidence != nil {
		t.Fatalf("claim should not have evidence when provenance is empty: %#v", claim)
	}

	claims, err := kg.GetClaimsForEdge("andi", "go", "likes", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("claims len = %d, want 1: %#v", len(claims), claims)
	}
	if claims[0].EvidenceID != "" || claims[0].Evidence != nil {
		t.Fatalf("stored claim should not have evidence: %#v", claims[0])
	}
}
