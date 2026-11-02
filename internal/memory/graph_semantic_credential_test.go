package memory

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

// kgEmbedRecorder embeds every text with a fixed vector and remembers what the
// provider was asked to embed. Queries get a different vector than documents so
// that filler documents with the query vector always rank first.
type kgEmbedRecorder struct {
	mu    sync.Mutex
	texts []string
}

func (r *kgEmbedRecorder) embed(_ context.Context, text string) ([]float32, error) {
	r.mu.Lock()
	r.texts = append(r.texts, text)
	r.mu.Unlock()
	if strings.HasPrefix(text, "query:") {
		return []float32{1, 0}, nil
	}
	return []float32{1, 0.3}, nil
}

func (r *kgEmbedRecorder) reset() {
	r.mu.Lock()
	r.texts = nil
	r.mu.Unlock()
}

func (r *kgEmbedRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.texts)
}

func newSemanticRecorderKG(t *testing.T) (*KnowledgeGraph, *kgEmbedRecorder) {
	t.Helper()
	kg := newTestKG(t)
	rec := &kgEmbedRecorder{}
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), rec.embed, nil); err != nil {
		t.Fatalf("enableSemanticSearchWithCollection: %v", err)
	}
	waitForSemanticReindexIdle(t, kg)
	rec.reset()
	return kg, rec
}

func semanticCollectionHas(t *testing.T, col *chromem.Collection, id string) bool {
	t.Helper()
	_, err := col.GetByID(t.Context(), id)
	return err == nil
}

func semanticContentCached(kg *KnowledgeGraph, id string) bool {
	idx := kg.semanticIndex()
	idx.Mu.Lock()
	defer idx.Mu.Unlock()
	_, ok := idx.ContentCache[id]
	return ok
}

func assertNoCredentialEmbedded(t *testing.T, rec *kgEmbedRecorder) {
	t.Helper()
	for _, text := range rec.snapshot() {
		if strings.Contains(text, precheckCredential) {
			t.Fatalf("embedding provider received credential-bearing KG content: %q", text)
		}
	}
}

// KG corrections and agent writes can put a credential into node properties; the
// provider must never see it, and an earlier clean vector must not stay searchable.
func TestKGSemanticNodeUpsertWithholdsCredentialFromEmbedding(t *testing.T) {
	kg, rec := newSemanticRecorderKG(t)
	idx := kg.semanticIndex()
	ctx := t.Context()

	clean := Node{ID: "cellar-host", Label: "Cellar host", Properties: map[string]string{"type": "device", "notes": "rack server in the cellar"}}
	if err := kg.upsertSemanticNodeIndexContext(ctx, clean); err != nil {
		t.Fatalf("clean upsert: %v", err)
	}
	if !semanticCollectionHas(t, idx.Collection, clean.ID) || !semanticContentCached(kg, clean.ID) {
		t.Fatal("clean node was not indexed")
	}

	rec.reset()
	tainted := clean
	tainted.Properties = map[string]string{"type": "device", "notes": "login " + precheckCredential}
	if err := kg.upsertSemanticNodeIndexContext(ctx, tainted); err != nil {
		t.Fatalf("a policy rejection must not be an error for the caller: %v", err)
	}
	assertNoCredentialEmbedded(t, rec)
	doc, err := idx.Collection.GetByID(ctx, clean.ID)
	if err != nil || strings.Contains(doc.Content, "rack server in the cellar") || strings.Contains(doc.Content, "notes:") {
		t.Fatalf("stale content survived projection: %v", err)
	}
	fresh := Node{ID: "fresh-cred", Label: "Fresh", Properties: map[string]string{"type": "device", "notes": precheckCredential}}
	if !kg.upsertSemanticNodeIndex(fresh) || !semanticCollectionHas(t, idx.Collection, fresh.ID) {
		t.Fatal("safe remainder was not indexed")
	}
	assertNoCredentialEmbedded(t, rec)
	// A credential in the label cannot be projected safely. Remove prior vectors.
	tainted.Label = "password: hunter"
	if err := kg.upsertSemanticNodeIndexContext(ctx, tainted); err != nil {
		t.Fatal(err)
	}
	if semanticCollectionHas(t, idx.Collection, clean.ID) || semanticContentCached(kg, clean.ID) {
		t.Fatal("unsafe label retained")
	}

}

// A dirty credential node must be marked handled; otherwise every reindex pass
// would pick it up again and again.
func TestKGSemanticReindexSkipsCredentialNodeAndClearsDirtyFlag(t *testing.T) {
	kg, rec := newSemanticRecorderKG(t)
	idx := kg.semanticIndex()
	ctx := t.Context()

	props := func(notes string) map[string]string { return map[string]string{"type": "device", "notes": notes} }
	for id, p := range map[string]map[string]string{
		"clean-alpha": props("alpha rack server"),
		"clean-beta":  props("beta rack server"),
		"cred-gamma":  props("login " + precheckCredential),
	} {
		if err := kg.AddNode(id, "Node "+id, p); err != nil {
			t.Fatalf("AddNode %s: %v", id, err)
		}
	}
	// Start from an empty index with a stale vector for the credential node, and
	// every node dirty, so the batch path has to do all the work.
	if err := idx.Collection.Delete(ctx, nil, nil, "clean-alpha", "clean-beta", "cred-gamma"); err != nil {
		t.Fatal(err)
	}
	if err := idx.Collection.AddDocument(ctx, chromem.Document{ID: "cred-gamma", Content: "old clean version", Embedding: []float32{1, 0.3}}); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at = NULL`); err != nil {
		t.Fatal(err)
	}
	rec.reset()

	if err := kg.RunSemanticReindexContext(ctx); err != nil {
		t.Fatalf("RunSemanticReindexContext: %v", err)
	}
	assertNoCredentialEmbedded(t, rec)
	if got := len(rec.snapshot()); got != 3 {
		t.Fatalf("provider calls = %d, want 3 (including the projected node)", got)
	}
	for _, id := range []string{"clean-alpha", "clean-beta"} {
		if !semanticCollectionHas(t, idx.Collection, id) {
			t.Fatalf("clean node %s was not indexed", id)
		}
	}
	assertProjectedDocument(t, kg, "cred-gamma")
	dirtyNodes, dirtyEdges, err := kg.DirtySemanticCounts()
	if err != nil || dirtyNodes != 0 || dirtyEdges != 0 {
		t.Fatalf("dirty after reindex = nodes %d edges %d err %v, want none", dirtyNodes, dirtyEdges, err)
	}

	rec.reset()
	if err := kg.RunSemanticReindexContext(ctx); err != nil {
		t.Fatalf("second RunSemanticReindexContext: %v", err)
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Fatalf("second reindex made %d provider calls, want 0", got)
	}

	// The graph itself is untouched: the node is still stored and findable.
	node, err := kg.GetNode("cred-gamma")
	if err != nil || node == nil || !strings.Contains(node.Properties["notes"], precheckCredential) {
		t.Fatalf("credential node was changed in SQLite: %+v %v", node, err)
	}
}

func TestKGSemanticEdgeWithholdsCredentialFromEmbedding(t *testing.T) {
	kg, rec := newSemanticRecorderKG(t)
	idx := kg.semanticIndex()
	ctx := t.Context()

	edgeDocID := func(source, target, relation string) string {
		return "edge://" + source + "\x00" + target + "\x00" + relation
	}
	cleanID := edgeDocID("rack-1", "rack-2", "connects_to")
	taintedID := edgeDocID("rack-1", "rack-3", "uses")

	if err := kg.AddEdge("rack-1", "rack-2", "connects_to", map[string]string{"notes": "patch cable"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("rack-1", "rack-3", "uses", map[string]string{"notes": "login " + precheckCredential}); err != nil {
		t.Fatal(err)
	}
	// Dirty edges with an empty index and a stale vector for the tainted one.
	if err := idx.Collection.Delete(ctx, nil, nil, cleanID, taintedID); err != nil {
		t.Fatal(err)
	}
	stale := chromem.Document{ID: taintedID, Content: "old clean version", Embedding: []float32{1, 0.3}}
	if err := idx.Collection.AddDocument(ctx, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_edges SET semantic_indexed_at = NULL`); err != nil {
		t.Fatal(err)
	}
	rec.reset()

	if err := kg.RunSemanticReindexContext(ctx); err != nil {
		t.Fatalf("RunSemanticReindexContext: %v", err)
	}
	assertNoCredentialEmbedded(t, rec)
	if !semanticCollectionHas(t, idx.Collection, cleanID) {
		t.Fatal("clean edge was not indexed")
	}
	assertProjectedDocument(t, kg, taintedID)
	if _, dirtyEdges, err := kg.DirtySemanticCounts(); err != nil || dirtyEdges != 0 {
		t.Fatalf("dirty edges after reindex = %d err %v, want 0", dirtyEdges, err)
	}
	rec.reset()
	if err := kg.RunSemanticReindexContext(ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Fatalf("second reindex made %d provider calls, want 0", got)
	}

	// The single-edge write path applies the same policy and reports no error.
	idx.Mu.Lock()
	idx.RemoveContentCacheEntry(taintedID)
	idx.Mu.Unlock()
	if err := idx.Collection.AddDocument(ctx, stale); err != nil {
		t.Fatal(err)
	}
	edge := Edge{Source: "rack-1", Target: "rack-3", Relation: "uses", Properties: map[string]string{"notes": "login " + precheckCredential}}
	if err := kg.upsertSemanticEdgeIndexContext(ctx, edge); err != nil {
		t.Fatalf("a policy rejection must not be an error for the caller: %v", err)
	}
	assertNoCredentialEmbedded(t, rec)
	assertProjectedDocument(t, kg, taintedID)
}

// A delete between the count and the query must not make the KG searches fail
// silently; chromem rejects nResults larger than the shrunken collection.
func TestKGSemanticSearchSurvivesCollectionShrinkingBetweenCountAndQuery(t *testing.T) {
	const query = "query: rack servers"
	fillers := []string{"filler-1", "filler-2", "filler-3", "filler-4", "filler-5", "filler-6"}
	edgeID := "edge://clamp-a\x00clamp-b\x00connects_to"

	// The fillers carry the query vector, so they outrank the real documents. The
	// hook deletes them on the onCall-th search query of the test collection.
	setup := func(t *testing.T, onCall int) *KnowledgeGraph {
		t.Helper()
		kg, _ := newSemanticRecorderKG(t)
		idx := kg.semanticIndex()
		for _, id := range []string{"clamp-a", "clamp-b"} {
			if err := kg.AddNode(id, "Rack "+id, map[string]string{"type": "device", "notes": "clamp fixture"}); err != nil {
				t.Fatal(err)
			}
		}
		if err := kg.upsertSemanticEdgeIndexContext(t.Context(), Edge{Source: "clamp-a", Target: "clamp-b", Relation: "connects_to", Properties: map[string]string{"notes": "clamp fixture"}}); err != nil {
			t.Fatal(err)
		}
		for _, id := range fillers {
			if err := idx.Collection.AddDocument(t.Context(), chromem.Document{ID: id, Content: "filler", Embedding: []float32{1, 0}}); err != nil {
				t.Fatal(err)
			}
		}
		if got := idx.Collection.Count(); got != 9 {
			t.Fatalf("fixture count = %d, want 9", got)
		}
		calls := 0
		semanticSearchAfterCountHook = func(col *chromem.Collection) {
			if col != idx.Collection {
				return
			}
			calls++
			if calls == onCall {
				if err := col.Delete(context.Background(), nil, nil, fillers...); err != nil {
					t.Errorf("shrink collection: %v", err)
				}
			}
		}
		t.Cleanup(func() { semanticSearchAfterCountHook = nil })
		return kg
	}
	wantNodes := func(t *testing.T, got []string) {
		t.Helper()
		slices.Sort(got)
		if !slices.Equal(got, []string{"clamp-a", "clamp-b"}) {
			t.Fatalf("results = %v, want [clamp-a clamp-b]", got)
		}
	}

	t.Run("nodes with queryer", func(t *testing.T) {
		kg := setup(t, 1)
		var ids []string
		for _, n := range kg.semanticSearchNodesWithQueryer(kg.db, query, 0, 20) {
			ids = append(ids, n.ID)
		}
		wantNodes(t, ids)
	})
	t.Run("node ids", func(t *testing.T) {
		kg := setup(t, 1)
		wantNodes(t, kg.semanticSearchNodeIDs(query, 20))
	})
	t.Run("node scores expand after the shrink", func(t *testing.T) {
		// The first window (2 fillers) holds no node, so the loop expands; the
		// shrink happens right before the expanded query.
		kg := setup(t, 2)
		scores := kg.semanticSearchNodeScores(query, 2)
		var ids []string
		for id := range scores {
			ids = append(ids, id)
		}
		wantNodes(t, ids)
	})
	t.Run("node scores stop once the collection is exhausted", func(t *testing.T) {
		kg := setup(t, 2)
		scores := kg.semanticSearchNodeScores(query, 4)
		var ids []string
		for id := range scores {
			ids = append(ids, id)
		}
		wantNodes(t, ids)
	})
	t.Run("edge ids", func(t *testing.T) {
		kg := setup(t, 1)
		if got := kg.semanticSearchEdgeIDs(query, 20); !slices.Equal(got, []string{edgeID}) {
			t.Fatalf("edge results = %q, want [%q]", got, edgeID)
		}
	})
}

func assertProjectedDocument(t *testing.T, kg *KnowledgeGraph, id string) {
	t.Helper()
	doc, err := kg.semanticIndex().Collection.GetByID(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc.Content, precheckCredential) || strings.Contains(doc.Content, "old clean version") {
		t.Fatal("unsafe or obsolete content retained")
	}
}

func TestKGSemanticProjectionNeverEmbedsSensitiveFields(t *testing.T) {
	kg, rec := newSemanticRecorderKG(t)
	props := map[string]string{"type": "device", "location": "cellar", "password": "hunter", "secret": "geheim", "pin": "12 34"}
	if err := kg.AddNode("projected-a", "Safe node", props); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddEdge("projected-a", "projected-b", "uses", props); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.db.Exec(`UPDATE kg_nodes SET semantic_indexed_at=NULL; UPDATE kg_edges SET semantic_indexed_at=NULL`); err != nil {
		t.Fatal(err)
	}
	if err := kg.RunSemanticReindexContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, text := range rec.snapshot() {
		for _, value := range []string{"hunter", "geheim", "12 34"} {
			if strings.Contains(text, value) {
				t.Fatal("sensitive field reached embedding provider")
			}
		}
	}
	node, err := kg.GetNode("projected-a")
	if err != nil || node.Properties["password"] != "hunter" || props["pin"] != "12 34" {
		t.Fatal("canonical data changed")
	}
	doc, err := kg.semanticIndex().Collection.GetByID(t.Context(), "projected-a")
	if err != nil || !strings.Contains(doc.Content, "cellar") {
		t.Fatal("safe remainder not searchable")
	}
}
