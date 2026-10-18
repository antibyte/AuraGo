package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
)

func TestDismissMemoryConflictKeepsFactsAndReviewedEvidence(t *testing.T) {
	s, err := NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"a", "b", "c"} {
		if err := s.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RegisterMemoryConflict("a", "b", "user|language", "german", "english", "first"); err != nil {
		t.Fatal(err)
	}
	if err := s.RegisterMemoryConflict("a", "c", "user|theme", "dark", "light", "other"); err != nil {
		t.Fatal(err)
	}
	items, _ := s.GetOpenMemoryConflicts(10)
	var id int64
	for _, item := range items {
		if item.ConflictKey == "user|language" {
			id = item.ID
		}
	}
	if err := s.DismissMemoryConflict(id, "both valid"); err != nil {
		t.Fatal(err)
	}
	for _, doc := range []string{"a", "b"} {
		meta, err := s.GetMemoryMeta(doc)
		if err != nil || IsMemoryArchived(meta) {
			t.Fatalf("lost fact %s: %+v %v", doc, meta, err)
		}
	}
	a, _ := s.GetMemoryMeta("a")
	if a.VerificationStatus != MemoryVerificationContradicted {
		t.Fatalf("other conflict was cleared: %+v", a)
	}
	if err := s.RegisterMemoryConflict("b", "a", "user|language", "english", "german", "scan again"); err != nil {
		t.Fatal(err)
	}
	reviewed, _ := s.GetMemoryConflictByID(id)
	if reviewed.Status != "dismissed" || reviewed.Reason != "both valid" {
		t.Fatalf("unchanged evidence reopened: %+v", reviewed)
	}
	if err := s.RegisterMemoryConflict("a", "b", "user|language", "french", "english", "changed evidence"); err != nil {
		t.Fatal(err)
	}
	reviewed, _ = s.GetMemoryConflictByID(id)
	if reviewed.Status != "open" || reviewed.ResolvedAt != "" {
		t.Fatalf("changed evidence did not reopen: %+v", reviewed)
	}
}

func TestKGContextExclusionsPrecedeLexicalLimit(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprint(fallback), func(t *testing.T) {
			kg := newTestKG(t)
			for i := 0; i < 12; i++ {
				if err := kg.AddNode(fmt.Sprintf("noise-%d", i), "docker", map[string]string{"type": "activity_entity"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := kg.AddNode("service", "docker", map[string]string{"type": "service"}); err != nil {
				t.Fatal(err)
			}
			query := "please find docker host"
			if fallback {
				if _, err := kg.db.Exec(`DROP TABLE kg_nodes_fts`); err != nil {
					t.Fatal(err)
				}
			}
			got := kg.SearchForContextStructured(query, 1, 800)
			if len(got.Nodes) != 1 || got.Nodes[0].ID != "service" {
				t.Fatalf("allowed hit crowded out: %+v", got.Nodes)
			}
			important, err := kg.GetImportantNodes(1, 0)
			if err != nil || len(important) != 1 || important[0].ID != "service" {
				t.Fatalf("important nodes: %+v %v", important, err)
			}
		})
	}
}

func TestKGSemanticExpandsPastExcludedCandidatesAndUsesFullQuery(t *testing.T) {
	kg := newTestKG(t)
	const query = "which host runs my home infrastructure"
	seenQuery := false
	embed := func(_ context.Context, text string) ([]float32, error) {
		if text == query {
			seenQuery = true
		}
		if strings.Contains(text, "useful") {
			return []float32{.99, .1}, nil
		}
		return []float32{1, 0}, nil
	}
	if err := kg.enableSemanticSearchWithCollection(chromem.NewDB(), embed, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := kg.AddNode(fmt.Sprintf("noise-%d", i), "irrelevant", map[string]string{"type": "activity_entity"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := kg.AddNode("useful", "useful", map[string]string{"type": "service"}); err != nil {
		t.Fatal(err)
	}
	got := kg.semanticSearchNodeScores(query, 1)
	if _, ok := got["useful"]; !ok || !seenQuery {
		t.Fatalf("hits=%v full query used=%v", got, seenQuery)
	}
}
