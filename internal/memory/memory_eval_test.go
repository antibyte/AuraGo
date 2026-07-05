package memory

import "testing"

func TestKnowledgeGraphMemoryQualityEvalH3H6(t *testing.T) {
	kg := newTestKG(t)

	// H3: contradiction flagging. A single-valued fact with two accepted objects
	// must become an explicit open conflict rather than silently overwriting history.
	germanClaim, err := kg.AddEdgeWithProvenance("user", "german", "primary_language", nil, KGProvenanceInput{SourceKind: "user"})
	if err != nil {
		t.Fatalf("add german claim: %v", err)
	}
	englishClaim, err := kg.AddEdgeWithProvenance("user", "english", "primary_language", nil, KGProvenanceInput{SourceKind: "user"})
	if err != nil {
		t.Fatalf("add english claim: %v", err)
	}
	conflicts, err := kg.GetOpenKGConflicts(10)
	if err != nil {
		t.Fatalf("GetOpenKGConflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("H3 conflict count = %d, want 1; german=%s english=%s conflicts=%#v", len(conflicts), germanClaim.ID, englishClaim.ID, conflicts)
	}

	if err := kg.ResolveKGConflict(conflicts[0].ID, englishClaim.ID, "H3 correction"); err != nil {
		t.Fatalf("ResolveKGConflict: %v", err)
	}
	germanHistory, err := kg.GetClaimsForEdge("user", "german", "primary_language", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge german history: %v", err)
	}
	if len(germanHistory) != 1 || germanHistory[0].Status != KGClaimSuperseded || germanHistory[0].SupersededBy != englishClaim.ID {
		t.Fatalf("H3 losing claim should be superseded by English: %#v", germanHistory)
	}
	englishActive, err := kg.GetClaimsForEdge("user", "english", "primary_language", false, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge english active: %v", err)
	}
	if len(englishActive) != 1 || englishActive[0].Status != KGClaimAccepted {
		t.Fatalf("H3 winning claim should remain accepted: %#v", englishActive)
	}

	// H6: forget/retract on command. Retraction hides the fact from active reads
	// while preserving why AuraGo used to believe it.
	if err := kg.RetractEdge("user", "english", "primary_language", "H6 forget command"); err != nil {
		t.Fatalf("RetractEdge: %v", err)
	}
	activeAfterRetract, err := kg.GetClaimsForEdge("user", "english", "primary_language", false, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge active after retract: %v", err)
	}
	if len(activeAfterRetract) != 0 {
		t.Fatalf("H6 retracted claim should not be active: %#v", activeAfterRetract)
	}
	historyAfterRetract, err := kg.GetClaimsForEdge("user", "english", "primary_language", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge history after retract: %v", err)
	}
	if len(historyAfterRetract) != 1 || historyAfterRetract[0].Status != KGClaimRetracted {
		t.Fatalf("H6 retracted claim should remain historical: %#v", historyAfterRetract)
	}

	deleteClaim, err := kg.AddEdgeWithProvenance("user", "cet", "timezone", nil, KGProvenanceInput{SourceKind: "user"})
	if err != nil {
		t.Fatalf("add timezone claim: %v", err)
	}
	if err := kg.DeleteEdge("user", "cet", "timezone"); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	edges, err := kg.GetAllEdges(100)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if containsTestEdge(edges, "user", "cet", "timezone") {
		t.Fatalf("deleted edge should not remain active: %#v", edges)
	}
	deleteHistory, err := kg.GetClaimsForEdge("user", "cet", "timezone", true, 10)
	if err != nil {
		t.Fatalf("GetClaimsForEdge delete history: %v", err)
	}
	if len(deleteHistory) != 1 || deleteHistory[0].ID != deleteClaim.ID {
		t.Fatalf("delete should preserve claim/evidence history, got %#v", deleteHistory)
	}
}
