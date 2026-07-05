package memory

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory/kgreasoner"
)

func TestKnowledgeGraphSuggestInferredRelationsUsesActiveEdges(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	if err := kg.AddEdge("service-a", "service-b", "depends_on", nil); err != nil {
		t.Fatalf("AddEdge a->b: %v", err)
	}
	if err := kg.AddEdge("service-b", "database", "depends_on", nil); err != nil {
		t.Fatalf("AddEdge b->db: %v", err)
	}
	if err := kg.AddEdge("server", "guest-wifi", "uses", nil); err != nil {
		t.Fatalf("AddEdge server->wifi: %v", err)
	}
	if err := kg.RetractEdge("server", "guest-wifi", "uses", "wrong network"); err != nil {
		t.Fatalf("RetractEdge: %v", err)
	}

	inferences, err := kg.SuggestInferredRelations(10)
	if err != nil {
		t.Fatalf("SuggestInferredRelations: %v", err)
	}
	if !hasMemoryInference(inferences, "service-a", "depends_on", "database") {
		t.Fatalf("missing transitive inference: %+v", inferences)
	}
	if hasMemoryInference(inferences, "guest-wifi", "used_by", "server") {
		t.Fatalf("retracted edge should not produce inference: %+v", inferences)
	}
}

func hasMemoryInference(inferences []kgreasoner.InferredFact, source, relation, target string) bool {
	for _, inf := range inferences {
		if inf.Source == source && inf.Relation == relation && inf.Target == target {
			return true
		}
	}
	return false
}
