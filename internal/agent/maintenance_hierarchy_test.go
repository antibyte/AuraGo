package agent

import (
	"io"
	"log/slog"
	"testing"

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
func (v *hierarchyVectorDB) StoreCheatsheet(id, name, content string) error { return nil }
func (v *hierarchyVectorDB) DeleteCheatsheet(id string) error               { return nil }

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
