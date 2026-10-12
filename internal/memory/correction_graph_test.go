package memory

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestMemoryCorrectionGraphClaimsPreserveOtherRelationships(t *testing.T) {
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: newTestKG(t), Access: CorrectionAccess{true, true, true}}
	for _, id := range []string{"person", "coffee", "tea", "other"} {
		if err := c.Graph.AddNode(id, id, map[string]string{"type": "concept"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Graph.AddEdge("person", "coffee", "prefers", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Graph.AddEdge("person", "other", "owns", nil); err != nil {
		t.Fatal(err)
	}
	ref := MemoryReference{Store: "kg_edge", ID: "person", Target: "coffee", Relation: "prefers"}
	p, err := c.Preview(t.Context(), "chat", "turn", "Changed preference", []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: "tea"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	p, err = c.Apply(t.Context(), p.ID, "chat")
	if err != nil {
		t.Fatal(err)
	}
	var accepted, oldClaims int
	if err = c.Graph.db.QueryRow(`SELECT COUNT(*) FROM kg_edges WHERE source='person' AND status='accepted'`).Scan(&accepted); err != nil || accepted != 2 {
		t.Fatal("foreign edge lost", accepted, err)
	}
	if err = c.Graph.db.QueryRow(`SELECT COUNT(*) FROM kg_claims WHERE subject_id='person' AND object_id='coffee' AND status='superseded'`).Scan(&oldClaims); err != nil || oldClaims == 0 {
		t.Fatal("old claim lost", oldClaims, err)
	}
	if err = c.Graph.AddEdge("person", "coffee", "prefers", nil); err == nil {
		t.Fatal("withdrawn edge resurrected")
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	ref.Target = "tea"
	next, err := c.Preview(t.Context(), "chat", "later", "A new user correction", []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: "coffee"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), next.ID, "chat"); err != nil {
		t.Fatal("new evidenced change could not restore prior target", err)
	}
}

func TestMemoryCorrectionGraphIndexFailureIsResumable(t *testing.T) {
	c, ref, _ := correctionFixture(t)
	var fail atomic.Bool
	if err := c.Graph.enableSemanticSearchWithCollection(chromem.NewDB(), func(context.Context, string) ([]float32, error) {
		if fail.Load() {
			return nil, errors.New("index unavailable")
		}
		return []float32{1, 0}, nil
	}, nil); err != nil {
		t.Fatal(err)
	}
	waitForSemanticReindexIdle(t, c.Graph)
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: correctionNew}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	p, err = c.Apply(t.Context(), p.ID, "chat")
	if err == nil || p.Status != "incomplete" {
		t.Fatal("index failure was successful", err)
	}
	found := false
	for _, item := range p.Items {
		found = found || item.State == "index_pending"
	}
	if !found {
		t.Fatal("no index receipt")
	}
	node, err := c.Graph.GetNode("core_fact_" + ref.ID)
	if err != nil || node.Properties["content"] != correctionNew {
		t.Fatal("graph commit disappeared", err)
	}
	fail.Store(false)
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	doc, err := c.Graph.semanticIndex().Collection.GetByID(t.Context(), node.ID)
	if err != nil || !strings.Contains(doc.Content, correctionNew) {
		t.Fatal("index not repaired", err)
	}
}

func TestMemoryCorrectionMultiplePropertiesOnOneEntity(t *testing.T) {
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: newTestKG(t), Access: CorrectionAccess{true, true, true}}
	if err := c.Graph.AddNode("device", "device", map[string]string{"cpu": "old CPU", "ram": "old RAM"}); err != nil {
		t.Fatal(err)
	}
	changes := []MemoryCorrectionChange{{Reference: MemoryReference{Store: "kg_node", ID: "device", Field: "cpu"}, Operation: "replace", Value: "new CPU"}, {Reference: MemoryReference{Store: "kg_node", ID: "device", Field: "ram"}, Operation: "replace", Value: "new RAM"}}
	p, err := c.Preview(t.Context(), "chat", "turn", "Verified hardware", changes, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal("repeat was not idempotent", err)
	}
}

func TestMemoryCorrectionRetractedEdgeNeedsFreshEvidence(t *testing.T) {
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: newTestKG(t), Access: CorrectionAccess{true, true, true}}
	for _, id := range []string{"person", "tea"} {
		if err := c.Graph.AddNode(id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Graph.AddEdge("person", "tea", "prefers", nil); err != nil {
		t.Fatal(err)
	}
	ref := MemoryReference{Store: "kg_edge", ID: "person", Target: "tea", Relation: "prefers"}
	p, err := c.Preview(t.Context(), "chat", "turn", "Withdraw preference", []MemoryCorrectionChange{{Reference: ref, Operation: "archive"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	p, err = c.Preview(t.Context(), "chat", "new-turn", "Renew preference", []MemoryCorrectionChange{{Reference: ref, Operation: "replace", Value: "tea"}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	var old int
	if err = c.Graph.db.QueryRow(`SELECT COUNT(*) FROM kg_claims WHERE status='retracted'`).Scan(&old); err != nil || old == 0 {
		t.Fatal("retraction history lost", err)
	}
}
