package memory

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func TestKnowledgeGraphExportJSONLDIncludesClaimsAndContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	claim, err := kg.AddEdgeWithProvenance("server", "portainer", "uses", map[string]string{"source": "test"}, KGProvenanceInput{
		SourceKind:   "user",
		SessionID:    "session-test",
		RawText:      "Server uses Portainer",
		EvidenceType: "remember",
	})
	if err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}

	doc, err := kg.ExportJSONLD(false, 20)
	if err != nil {
		t.Fatalf("ExportJSONLD: %v", err)
	}
	if doc.Context["kg"] == "" {
		t.Fatalf("missing kg context: %+v", doc.Context)
	}
	if doc.Metadata["include_inactive"] != false {
		t.Fatalf("include_inactive metadata = %#v, want false", doc.Metadata["include_inactive"])
	}

	relationships := collectJSONLDRelationships(t, doc)
	if len(relationships) != 1 {
		t.Fatalf("relationship count = %d, want 1: %+v", len(relationships), relationships)
	}
	if relationships[0].Relation != "uses" || relationships[0].Status != KGClaimAccepted {
		t.Fatalf("unexpected relationship: %+v", relationships[0])
	}
	if len(relationships[0].Claims) != 1 || relationships[0].Claims[0].ID != claim.ID {
		t.Fatalf("missing claim in relationship: %+v", relationships[0].Claims)
	}
	if relationships[0].Claims[0].Evidence == nil || relationships[0].Claims[0].Evidence.RawText != "Server uses Portainer" {
		t.Fatalf("missing evidence in claim: %+v", relationships[0].Claims[0])
	}
}

func TestKnowledgeGraphExportJSONLDInactiveToggle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	if _, err := kg.AddEdgeWithProvenance("server", "guest-wifi", "uses_network", nil, KGProvenanceInput{SourceKind: "user"}); err != nil {
		t.Fatalf("AddEdgeWithProvenance: %v", err)
	}
	if err := kg.RetractEdge("server", "guest-wifi", "uses_network", "wrong network"); err != nil {
		t.Fatalf("RetractEdge: %v", err)
	}

	activeOnly, err := kg.ExportJSONLD(false, 20)
	if err != nil {
		t.Fatalf("ExportJSONLD active: %v", err)
	}
	if relationships := collectJSONLDRelationships(t, activeOnly); len(relationships) != 0 {
		t.Fatalf("active export should hide retracted relationships: %+v", relationships)
	}

	withInactive, err := kg.ExportJSONLD(true, 20)
	if err != nil {
		t.Fatalf("ExportJSONLD inactive: %v", err)
	}
	relationships := collectJSONLDRelationships(t, withInactive)
	if len(relationships) != 1 || relationships[0].Status != KGClaimRetracted {
		t.Fatalf("inactive export should include retracted relationship: %+v", relationships)
	}
	if len(relationships[0].Claims) != 1 || relationships[0].Claims[0].Status != KGClaimRetracted {
		t.Fatalf("inactive export should include retracted claim history: %+v", relationships[0].Claims)
	}
}

func collectJSONLDRelationships(t *testing.T, doc *KGJSONLDDocument) []KGJSONLDRelationship {
	t.Helper()
	var relationships []KGJSONLDRelationship
	for _, entry := range doc.Graph {
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal graph entry: %v", err)
		}
		var probe struct {
			Type string `json:"@type"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Fatalf("unmarshal graph probe: %v", err)
		}
		if probe.Type != "kg:Relationship" {
			continue
		}
		var rel KGJSONLDRelationship
		if err := json.Unmarshal(raw, &rel); err != nil {
			t.Fatalf("unmarshal relationship: %v", err)
		}
		relationships = append(relationships, rel)
	}
	return relationships
}
