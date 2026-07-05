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
