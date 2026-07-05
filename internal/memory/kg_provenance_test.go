package memory

import (
	"database/sql"
	"encoding/json"
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

func TestSupersededEdgesAreHiddenFromDefaultReads(t *testing.T) {
	kg := newTestKG(t)

	if _, err := kg.AddEdgeWithProvenance("andi", "german", "primary_language", nil, KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}
	if err := kg.SupersedeEdge("andi", "german", "primary_language", "claim_new", "corrected language"); err != nil {
		t.Fatalf("SupersedeEdge: %v", err)
	}

	edges, err := kg.GetImportantEdges(10, []string{"andi"})
	if err != nil {
		t.Fatalf("GetImportantEdges: %v", err)
	}
	if containsTestEdge(edges, "andi", "german", "primary_language") {
		t.Fatalf("superseded edge should be hidden from important reads: %#v", edges)
	}

	claims, err := kg.GetClaimsForEdge("andi", "german", "primary_language", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge: %v", err)
	}
	if len(claims) != 1 || claims[0].Status != KGClaimSuperseded || claims[0].SupersededBy != "claim_new" {
		t.Fatalf("expected superseded claim history, got %#v", claims)
	}
}

func TestRetractedEdgesKeepClaimHistory(t *testing.T) {
	kg := newTestKG(t)

	if _, err := kg.AddEdgeWithProvenance("andi", "english", "primary_language", nil, KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}
	if err := kg.RetractEdge("andi", "english", "primary_language", "user correction"); err != nil {
		t.Fatalf("RetractEdge: %v", err)
	}

	searchEdges := decodeTestSearchEdges(t, kg.Search("english"))
	if containsTestEdge(searchEdges, "andi", "english", "primary_language") {
		t.Fatalf("retracted edge should be hidden from default search: %#v", searchEdges)
	}

	activeClaims, err := kg.GetClaimsForEdge("andi", "english", "primary_language", false, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge active: %v", err)
	}
	if len(activeClaims) != 0 {
		t.Fatalf("active claims should be hidden after retraction, got %#v", activeClaims)
	}

	historicalClaims, err := kg.GetClaimsForEdge("andi", "english", "primary_language", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge history: %v", err)
	}
	if len(historicalClaims) != 1 || historicalClaims[0].Status != KGClaimRetracted {
		t.Fatalf("expected retracted claim history, got %#v", historicalClaims)
	}
}

func containsTestEdge(edges []Edge, source, target, relation string) bool {
	for _, edge := range edges {
		if edge.Source == source && edge.Target == target && edge.Relation == relation {
			return true
		}
	}
	return false
}

func decodeTestSearchEdges(t *testing.T, raw string) []Edge {
	t.Helper()
	if raw == "[]" {
		return nil
	}
	var payload struct {
		Edges []Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal search payload %q: %v", raw, err)
	}
	return payload.Edges
}
