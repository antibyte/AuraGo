package agent

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type hierarchyVectorDB struct {
	stored map[string]string
	count  int
}

func (v *hierarchyVectorDB) StoreDocument(concept, content string) ([]string, error) {
	if v.stored == nil {
		v.stored = map[string]string{}
	}
	if strings.HasPrefix(concept, "fail:") {
		return nil, fmt.Errorf("store failed for %s", concept)
	}
	id := concept
	v.stored[id] = content
	v.count++
	return []string{id}, nil
}
func (v *hierarchyVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return concept, nil
}
func (v *hierarchyVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) { return nil, nil }
func (v *hierarchyVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *hierarchyVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *hierarchyVectorDB) GetByID(id string) (string, error) { return v.stored[id], nil }
func (v *hierarchyVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return v.stored[id], nil
}
func (v *hierarchyVectorDB) DeleteDocument(id string) error { delete(v.stored, id); return nil }
func (v *hierarchyVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	delete(v.stored, id)
	return nil
}
func (v *hierarchyVectorDB) Count() int       { return v.count }
func (v *hierarchyVectorDB) IsDisabled() bool { return false }
func (v *hierarchyVectorDB) Close() error     { return nil }
func (v *hierarchyVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (v *hierarchyVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (v *hierarchyVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (v *hierarchyVectorDB) DeleteCheatsheet(id string) error         { return nil }
func (v *hierarchyVectorDB) RegisterCollections(collections []string) {}

func TestConsolidateSTMtoLTMDoesNotClaimWhenNoLLMAvailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for i, role := range []string{"user", "assistant", "user"} {
		if _, err := stm.InsertMessage("s1", role, fmt.Sprintf("remember the NAS backup target %d", i), false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if err := stm.DeleteOldMessages("s1", 1); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	cfg := &config.Config{}
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.MaxBatchMessages = 10

	consolidateSTMtoLTM(cfg, logger, nil, stm, &hierarchyVectorDB{}, nil)

	msgs, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil {
		t.Fatalf("GetConsolidationCandidates: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("len(candidates) = %d, want pending candidates", len(msgs))
	}
	for _, msg := range msgs {
		if msg.ConsolidationStatus != "pending" {
			t.Fatalf("status = %q, want pending", msg.ConsolidationStatus)
		}
	}
}

func TestStoreConsolidationFactsReportsStoreFailures(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	stored, err := storeConsolidationFacts(logger, stm, &hierarchyVectorDB{}, []helperConsolidationFact{
		{Concept: "ok:backup", Content: "The backup target is the NAS."},
		{Concept: "fail:backup", Content: "This store should fail."},
	})
	if err == nil {
		t.Fatal("expected store failure to be reported")
	}
	if stored != 1 {
		t.Fatalf("stored = %d, want 1", stored)
	}
}

func TestSyncCoreMemoryToKnowledgeGraphRemovesDeletedFacts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	id, err := stm.AddCoreMemoryFact("User prefers quiet infrastructure dashboards.")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}
	SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger)

	nodeID := fmt.Sprintf("core_fact_%d", id)
	if node, err := kg.GetNode(nodeID); err != nil || node == nil {
		t.Fatalf("expected synced core fact node, node=%v err=%v", node, err)
	}
	if err := stm.DeleteCoreMemoryFact(id); err != nil {
		t.Fatalf("DeleteCoreMemoryFact: %v", err)
	}
	SyncCoreMemoryToKnowledgeGraph(t.Context(), stm, kg, logger)

	node, err := kg.GetNode(nodeID)
	if err != nil {
		t.Fatalf("GetNode after delete: %v", err)
	}
	if node != nil {
		t.Fatalf("expected stale core fact node removed, got %#v", node)
	}
}

func TestConsolidateEpisodicHierarchyPromotesLevelOneEpisodes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	_ = stm.InsertEpisodicMemoryWithDetails("2026-03-31", "Deploy", "Homepage rollout finished", map[string]string{"scope": "homepage"}, 3, "consolidation", memory.EpisodicMemoryDetails{SessionID: "session-a", HierarchyLevel: 1, Participants: []string{"user", "agent"}})
	_ = stm.InsertEpisodicMemoryWithDetails("2026-03-31", "Fix", "Resolved reverse proxy config issue", map[string]string{"scope": "proxy"}, 3, "consolidation", memory.EpisodicMemoryDetails{SessionID: "session-a", HierarchyLevel: 1, Participants: []string{"user", "agent"}})

	vdb := &hierarchyVectorDB{}
	consolidateEpisodicHierarchy(logger, stm, vdb, nil)

	levelOne, err := stm.GetEpisodicMemoriesByHierarchyLevel(1, 10)
	if err != nil {
		t.Fatalf("GetEpisodicMemoriesByHierarchyLevel level1: %v", err)
	}
	if len(levelOne) != 0 {
		t.Fatalf("expected source episodes to be promoted out of level 1, got %d", len(levelOne))
	}
	levelTwo, err := stm.GetEpisodicMemoriesByHierarchyLevel(2, 10)
	if err != nil {
		t.Fatalf("GetEpisodicMemoriesByHierarchyLevel level2: %v", err)
	}
	if len(levelTwo) == 0 {
		t.Fatal("expected synthesized level-2 episodic memory")
	}
	if vdb.Count() == 0 {
		t.Fatal("expected hierarchical consolidation to store a synthesis document")
	}
}
