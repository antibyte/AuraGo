package agent

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/memory"
	"aurago/internal/memory/kgsemantic"

	chromem "github.com/philippgille/chromem-go"
)

func TestCoreMemorySyncUpdatesProtectedLabelContentAndSemanticIndex(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := memory.NewKnowledgeGraph(filepath.Join(t.TempDir(), "graph.db"), "", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	vector := chromem.NewDB()
	if err := kg.EnableSemanticSearchShared(vector, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil }); err != nil {
		t.Fatal(err)
	}
	id, err := stm.AddCoreMemoryFact("User prefers German")
	if err != nil {
		t.Fatal(err)
	}
	nodeID := fmt.Sprintf("core_fact_%d", id)
	if err := SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger); err != nil {
		t.Fatal(err)
	}
	if _, err := kg.SetNodeProtected(nodeID, true); err != nil {
		t.Fatal(err)
	}
	for _, updated := range []string{"User prefers English", strings.Repeat("日本語ä", 20)} {
		if err := stm.UpdateCoreMemoryFact(id, updated); err != nil {
			t.Fatal(err)
		}
		if err := SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger); err != nil {
			t.Fatal(err)
		}
		node, err := kg.GetNode(nodeID)
		if err != nil || node == nil || node.Properties["content"] != updated || !node.Protected || !utf8.ValidString(node.Label) || utf8.RuneCountInString(node.Label) > 50 {
			t.Fatalf("stale or damaged node: %#v %v", node, err)
		}
		if len([]rune(updated)) <= 50 && node.Label != updated {
			t.Fatalf("stale label: %q want %q", node.Label, updated)
		}
		doc, err := vector.GetCollection(kgsemantic.CollectionName, nil).GetByID(t.Context(), nodeID)
		if err != nil || !strings.Contains(doc.Content, updated) || strings.Contains(doc.Content, "User prefers German") {
			t.Fatalf("stale semantic index: %+v %v", doc, err)
		}
	}
	if err := stm.DeleteCoreMemoryFact(id); err != nil {
		t.Fatal(err)
	}
	if err := SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger); err != nil {
		t.Fatal(err)
	}
	if node, err := kg.GetNode(nodeID); err != nil || node == nil || !node.Protected {
		t.Fatalf("protected node removed: %v %v", node, err)
	}
}

func TestCoreMemorySyncFailurePreventsStaleCleanup(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "graph.db")
	kg, err := memory.NewKnowledgeGraph(path, "", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	id, err := stm.AddCoreMemoryFact("User prefers German")
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("core_fact_999", "Stale core fact", map[string]string{"source": "core_memory", "type": "concept"}); err != nil {
		t.Fatal(err)
	}
	if err := kg.AddNode("core_fact_external", "Foreign node", map[string]string{"source": "file", "type": "concept"}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER reject_core_update BEFORE UPDATE ON kg_nodes BEGIN SELECT RAISE(ABORT, 'synthetic sync failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := stm.UpdateCoreMemoryFact(id, "User prefers English"); err != nil {
		t.Fatal(err)
	}
	if err := SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger); err == nil {
		t.Fatal("sync failure was ignored")
	}
	if node, err := kg.GetNode("core_fact_999"); err != nil || node == nil {
		t.Fatalf("cleanup ran after incomplete sync: %v %v", node, err)
	}
	if _, err := db.Exec("DROP TRIGGER reject_core_update"); err != nil {
		t.Fatal(err)
	}
	if err := SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger); err != nil {
		t.Fatal(err)
	}
	if node, err := kg.GetNode("core_fact_external"); err != nil || node == nil {
		t.Fatalf("foreign node removed: %v %v", node, err)
	}
}
