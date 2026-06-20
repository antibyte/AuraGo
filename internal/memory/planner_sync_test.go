package memory

import (
	"testing"
)

func kgHasEdge(edges []Edge, source, target, relation string) bool {
	for _, edge := range edges {
		if edge.Source == source && edge.Target == target && edge.Relation == relation {
			return true
		}
	}
	return false
}

func TestPrunePlannerEdgesRemovesStaleTargets(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("appointment_a1", "Meeting", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("contact_keep", "Keep", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("contact_stale", "Stale", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("appointment_a1", "contact_keep", "involves", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("appointment_a1", "contact_stale", "involves", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}

	removed, err := kg.PrunePlannerEdges("appointment_a1", "involves", map[string]struct{}{"contact_keep": {}})
	if err != nil {
		t.Fatalf("PrunePlannerEdges: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if !kgHasEdge(edges, "appointment_a1", "contact_keep", "involves") {
		t.Fatal("expected keep edge to remain")
	}
	if kgHasEdge(edges, "appointment_a1", "contact_stale", "involves") {
		t.Fatal("expected stale edge removed")
	}
}

func TestPrunePlannerEdgesKeepsNonPlannerEdges(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddEdge("appointment_a1", "contact_manual", "involves", map[string]string{"source": "manual"}); err != nil {
		t.Fatal(err)
	}

	removed, err := kg.PrunePlannerEdges("appointment_a1", "involves", map[string]struct{}{})
	if err != nil {
		t.Fatalf("PrunePlannerEdges: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0 for non-planner edge", removed)
	}

	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if !kgHasEdge(edges, "appointment_a1", "contact_manual", "involves") {
		t.Fatal("expected non-planner edge preserved")
	}
}

func TestPrunePlannerNodesByPrefixRemovesStaleItemNodes(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("todo_t1", "Todo", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("todo_t1_item_keep", "Keep", map[string]string{"source": "planner", "type": "task_item"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("todo_t1_item_stale", "Stale", map[string]string{"source": "planner", "type": "task_item"}); err != nil {
		t.Fatal(err)
	}

	removed, err := kg.PrunePlannerNodesByPrefix("todo_t1_item_", map[string]struct{}{"todo_t1_item_keep": {}})
	if err != nil {
		t.Fatalf("PrunePlannerNodesByPrefix: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	if node, err := kg.GetNode("todo_t1_item_stale"); err != nil {
		t.Fatalf("GetNode stale: %v", err)
	} else if node != nil {
		t.Fatal("expected stale item node deleted")
	}
	if node, err := kg.GetNode("todo_t1_item_keep"); err != nil || node == nil {
		t.Fatalf("expected keep item node, node=%v err=%v", node, err)
	}
}

func TestDeleteStalePlannerSyncEdges(t *testing.T) {
	kg := newTestKG(t)

	if err := kg.AddNode("appointment_active", "Active", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("contact_c1", "Contact", nil); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("appointment_removed", "Removed", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}

	expected := map[string]struct{}{
		knowledgeGraphEdgeKey("appointment_active", "contact_c1", "involves"): {},
	}
	active := map[string]struct{}{
		"appointment_active": {},
	}

	if err := kg.AddEdge("appointment_active", "contact_c1", "involves", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}
	// Unexpected planner edge should be removed.
	if err := kg.AddEdge("appointment_active", "contact_c1", "related_to", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}
	// Edge referencing inactive planner node should be removed even if marked expected.
	if err := kg.AddEdge("appointment_removed", "contact_c1", "involves", map[string]string{"source": "planner"}); err != nil {
		t.Fatal(err)
	}

	removed, err := kg.DeleteStalePlannerSyncEdges(expected, active)
	if err != nil {
		t.Fatalf("DeleteStalePlannerSyncEdges: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}

	edges, err := kg.GetAllEdges(10)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}
	if !kgHasEdge(edges, "appointment_active", "contact_c1", "involves") {
		t.Fatal("expected valid involves edge kept")
	}
	if kgHasEdge(edges, "appointment_active", "contact_c1", "related_to") {
		t.Fatal("expected unexpected planner edge removed")
	}
	if kgHasEdge(edges, "appointment_removed", "contact_c1", "involves") {
		t.Fatal("expected edge with inactive planner endpoint removed")
	}
}

func TestPruneStalePlannerRootNodes(t *testing.T) {
	kg := newTestKG(t)

	for _, spec := range []struct {
		id   string
		keep bool
	}{
		{"appointment_active", true},
		{"appointment_removed", false},
		{"todo_active", true},
		{"todo_removed", false},
		{"todo_active_item_keep", true},
		{"todo_active_item_stale", false},
	} {
		if err := kg.AddNode(spec.id, spec.id, map[string]string{"source": "planner"}); err != nil {
			t.Fatalf("AddNode %s: %v", spec.id, err)
		}
	}

	active := map[string]struct{}{
		"appointment_active":    {},
		"todo_active":           {},
		"todo_active_item_keep": {},
	}

	removed, err := kg.PruneStalePlannerRootNodes(active)
	if err != nil {
		t.Fatalf("PruneStalePlannerRootNodes: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2 stale root nodes", removed)
	}

	for _, id := range []string{"appointment_active", "todo_active", "todo_active_item_keep", "todo_active_item_stale"} {
		node, err := kg.GetNode(id)
		if err != nil {
			t.Fatalf("GetNode %s: %v", id, err)
		}
		if node == nil {
			t.Fatalf("expected node %s to remain", id)
		}
	}
	for _, id := range []string{"appointment_removed", "todo_removed"} {
		node, err := kg.GetNode(id)
		if err != nil {
			t.Fatalf("GetNode %s: %v", id, err)
		}
		if node != nil {
			t.Fatalf("expected stale root node %s deleted", id)
		}
	}
}