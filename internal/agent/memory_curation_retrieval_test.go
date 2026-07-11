package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestRankMemoryCandidatesFiltersArchivedMemories(t *testing.T) {
	resetMemoryMetaCacheForTests()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-active", memory.MemoryMetaUpdate{VerificationStatus: "confirmed", ExtractionConfidence: 0.95, SourceReliability: 0.90}); err != nil {
		t.Fatalf("active meta: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-archived", memory.MemoryMetaUpdate{VerificationStatus: "unverified", ExtractionConfidence: 0.95, SourceReliability: 0.90}); err != nil {
		t.Fatalf("archived meta: %v", err)
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "doc-archived", Action: memory.MemoryCurationActionArchive, Reason: "test"}, "system", false); err != nil {
		t.Fatalf("archive meta: %v", err)
	}

	ranked := rankMemoryCandidates(
		[]string{"active memory", "archived memory"},
		[]string{"doc-active", "doc-archived"},
		stm,
		nil,
		time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	)
	if len(ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1; ranked=%+v", len(ranked), ranked)
	}
	if ranked[0].docID != "doc-active" {
		t.Fatalf("ranked doc = %q, want doc-active", ranked[0].docID)
	}
}

type archiveFilterVectorDB struct {
	byQuery map[string][]memory.SearchResult
}

func (v *archiveFilterVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return nil, nil
}
func (v *archiveFilterVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", nil
}
func (v *archiveFilterVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}
func (v *archiveFilterVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *archiveFilterVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	results, err := v.SearchMemoriesOnlyScored(query, topK)
	if err != nil {
		return nil, nil, err
	}
	memories := make([]string, 0, len(results))
	docIDs := make([]string, 0, len(results))
	for _, result := range results {
		memories = append(memories, result.Text)
		docIDs = append(docIDs, result.DocID)
	}
	return memories, docIDs, nil
}
func (v *archiveFilterVectorDB) SearchMemoriesOnlyScored(query string, topK int) ([]memory.SearchResult, error) {
	items := v.byQuery[query]
	if len(items) == 0 {
		return nil, nil
	}
	if topK > 0 && len(items) > topK {
		items = items[:topK]
	}
	return append([]memory.SearchResult(nil), items...), nil
}
func (v *archiveFilterVectorDB) GetByID(id string) (string, error) { return "", nil }
func (v *archiveFilterVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (v *archiveFilterVectorDB) DeleteDocument(id string) error { return nil }
func (v *archiveFilterVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return nil
}
func (v *archiveFilterVectorDB) Count() int       { return 0 }
func (v *archiveFilterVectorDB) IsDisabled() bool { return false }
func (v *archiveFilterVectorDB) IsReady() bool    { return true }
func (v *archiveFilterVectorDB) Close() error     { return nil }
func (v *archiveFilterVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (v *archiveFilterVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (v *archiveFilterVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (v *archiveFilterVectorDB) DeleteCheatsheet(id string) error         { return nil }
func (v *archiveFilterVectorDB) RegisterCollections(collections []string) {}

func TestSearchRankedMemoriesOnlyFiltersArchivedMemories(t *testing.T) {
	resetMemoryMetaCacheForTests()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-active", memory.MemoryMetaUpdate{VerificationStatus: "confirmed"}); err != nil {
		t.Fatalf("active meta: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-archived", memory.MemoryMetaUpdate{VerificationStatus: "unverified"}); err != nil {
		t.Fatalf("archived meta: %v", err)
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "doc-archived", Action: memory.MemoryCurationActionArchive, Reason: "test"}, "system", false); err != nil {
		t.Fatalf("archive meta: %v", err)
	}

	vdb := &archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{
		"nas backup": {
			{Text: "active memory", DocID: "doc-active", Similarity: 0.9},
			{Text: "archived memory", DocID: "doc-archived", Similarity: 0.95},
		},
	}}

	ranked, err := searchRankedMemoriesOnly(context.Background(), vdb, stm, "nas backup", 2, nil, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("searchRankedMemoriesOnly: %v", err)
	}
	if len(ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1", len(ranked))
	}
	if ranked[0].docID != "doc-active" {
		t.Fatalf("docID = %q, want doc-active", ranked[0].docID)
	}
}

func TestSearchRankedMemoriesOnlyBackfillsArchivedTopHit(t *testing.T) {
	resetMemoryMetaCacheForTests()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-archived", memory.MemoryMetaUpdate{VerificationStatus: "unverified"}); err != nil {
		t.Fatalf("archived meta: %v", err)
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "doc-archived", Action: memory.MemoryCurationActionArchive, Reason: "test"}, "system", false); err != nil {
		t.Fatalf("archive meta: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-active", memory.MemoryMetaUpdate{VerificationStatus: "confirmed"}); err != nil {
		t.Fatalf("active meta: %v", err)
	}

	vdb := &archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{
		"nas backup": {
			{Text: "archived memory", DocID: "doc-archived", Similarity: 0.99},
			{Text: "active memory", DocID: "doc-active", Similarity: 0.80},
		},
	}}

	ranked, err := searchRankedMemoriesOnly(context.Background(), vdb, stm, "nas backup", 1, nil, time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("searchRankedMemoriesOnly: %v", err)
	}
	if len(ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1", len(ranked))
	}
	if ranked[0].docID != "doc-active" {
		t.Fatalf("docID = %q, want doc-active", ranked[0].docID)
	}
}
