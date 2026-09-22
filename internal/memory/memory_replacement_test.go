package memory

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestReplaceMemoryDocumentPreservesCompleteMetadata(t *testing.T) {
	stm := newTestNotesDB(t)
	if err := stm.UpsertMemoryMetaWithDetails("old-doc", MemoryMetaUpdate{
		ExtractionConfidence: 0.94,
		VerificationStatus:   MemoryVerificationConfirmed,
		SourceType:           "user",
		SourceReliability:    0.97,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}
	if _, err := stm.db.Exec(`
		UPDATE memory_meta
		SET access_count = 7,
		    useful_count = 4,
		    useless_count = 1,
		    last_effectiveness_at = '2025-01-02 03:04:05',
		    last_reviewed_at = '2025-01-03 04:05:06',
		    review_note = 'keep provenance'
		WHERE doc_id = 'old-doc'
	`); err != nil {
		t.Fatalf("seed complete metadata: %v", err)
	}
	expected, err := stm.GetMemoryMeta("old-doc")
	if err != nil {
		t.Fatalf("GetMemoryMeta: %v", err)
	}
	const oldContent = "Project AuroraGo deployment note"
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": oldContent}}
	newIDs, err := stm.ReplaceMemoryDocument(fake, "old-doc", "replacement", oldContent, "Project AuraGo deployment note", expected, "test replacement", "test")
	if err != nil {
		t.Fatalf("ReplaceMemoryDocument: %v", err)
	}
	if len(newIDs) != 1 || len(fake.deleted) != 1 || fake.deleted[0] != "old-doc" {
		t.Fatalf("newIDs=%v deleted=%v, want one replacement and old deletion", newIDs, fake.deleted)
	}
	newMeta, err := stm.GetMemoryMeta(newIDs[0])
	if err != nil {
		t.Fatalf("GetMemoryMeta replacement: %v", err)
	}
	if newMeta.AccessCount != expected.AccessCount || newMeta.UsefulCount != expected.UsefulCount || newMeta.UselessCount != expected.UselessCount {
		t.Fatalf("replacement counters = %+v, want access=%d useful=%d useless=%d", newMeta, expected.AccessCount, expected.UsefulCount, expected.UselessCount)
	}
	if newMeta.VerificationStatus != expected.VerificationStatus || newMeta.SourceType != expected.SourceType || newMeta.SourceReliability != expected.SourceReliability || newMeta.ReviewNote != expected.ReviewNote {
		t.Fatalf("replacement provenance = %+v, want status/source/reliability/review preserved", newMeta)
	}
	if newMeta.ArchivedAt != "" || IsMemoryArchived(newMeta) {
		t.Fatalf("replacement metadata archived = %+v, want active", newMeta)
	}
	oldMeta, err := stm.GetMemoryMeta("old-doc")
	if err != nil {
		t.Fatalf("GetMemoryMeta archived source: %v", err)
	}
	if !IsMemoryArchived(oldMeta) {
		t.Fatalf("old metadata = %+v, want archived", oldMeta)
	}
}

func TestReplaceMemoryDocumentAbortsOnMetadataDrift(t *testing.T) {
	stm := newTestNotesDB(t)
	if err := stm.UpsertMemoryMeta("old-doc"); err != nil {
		t.Fatalf("UpsertMemoryMeta: %v", err)
	}
	expected, err := stm.GetMemoryMeta("old-doc")
	if err != nil {
		t.Fatalf("GetMemoryMeta: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": "Project AuroraGo note"}}
	fake.afterStore = func() {
		_ = stm.UpsertMemoryMetaWithDetails("old-doc", MemoryMetaUpdate{
			ExtractionConfidence: expected.ExtractionConfidence,
			VerificationStatus:   expected.VerificationStatus,
			SourceType:           expected.SourceType,
			SourceReliability:    0.42,
		})
	}
	newIDs, err := stm.ReplaceMemoryDocument(fake, "old-doc", "replacement", "Project AuroraGo note", "Project AuraGo note", expected, "test replacement", "test")
	if err == nil || !strings.Contains(err.Error(), "metadata changed") {
		t.Fatalf("error = %v, want metadata drift conflict", err)
	}
	if len(newIDs) != 0 || len(fake.deleted) != 1 || fake.deleted[0] != "new-doc-1" {
		t.Fatalf("newIDs=%v deleted=%v, want only created replacement rollback", newIDs, fake.deleted)
	}
	meta, err := stm.GetMemoryMeta("old-doc")
	if err != nil {
		t.Fatalf("GetMemoryMeta after drift: %v", err)
	}
	if IsMemoryArchived(meta) {
		t.Fatalf("old metadata = %+v, want active after drift abort", meta)
	}
}

func TestReplaceMemoryDocumentRejectsProtectedSource(t *testing.T) {
	stm := newTestNotesDB(t)
	if err := stm.UpsertMemoryMeta("old-doc"); err != nil {
		t.Fatalf("UpsertMemoryMeta: %v", err)
	}
	if err := stm.SetMemoryMetaProtection("old-doc", true, false); err != nil {
		t.Fatalf("SetMemoryMetaProtection: %v", err)
	}
	expected, err := stm.GetMemoryMeta("old-doc")
	if err != nil {
		t.Fatalf("GetMemoryMeta: %v", err)
	}
	fake := &fakeRepairVectorDB{docs: map[string]string{"old-doc": "Project AuroraGo note"}}
	_, err = stm.ReplaceMemoryDocument(fake, "old-doc", "replacement", "Project AuroraGo note", "Project AuraGo note", expected, "test replacement", "test")
	if err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("error = %v, want protected rejection", err)
	}
	if len(fake.stored) != 0 || len(fake.deleted) != 0 {
		t.Fatalf("stored=%v deleted=%v, want no vector mutation", fake.stored, fake.deleted)
	}
}

func TestReplaceMemoryDocumentRejectsUnknownOwnership(t *testing.T) {
	stm := newTestNotesDB(t)
	const oldContent = "Project AuroraGo note"
	fake := &fakeRepairVectorDB{docs: map[string]string{"old": oldContent}}
	if err := stm.UpsertMemoryMeta("old"); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	expected, err := stm.GetMemoryMeta("old")
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	fake.storeOwned = func(string, string, VectorStoreMode) (VectorStoreResult, error) {
		return VectorStoreResult{CreatedIDs: []string{"new"}, UnknownIDs: []string{"unknown"}}, nil
	}
	if _, err := stm.ReplaceMemoryDocument(fake, "old", "replacement", oldContent, "Project AuraGo note", expected, "test replacement", "test"); err == nil {
		t.Fatal("expected unknown ownership to abort replacement")
	}
	if got, err := fake.GetByID("old"); err != nil || got != oldContent {
		t.Fatalf("source vector changed after unknown ownership: %q, %v", got, err)
	}
}

func TestStoreDocumentWithOwnershipLegacyMarksUnknown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	fake := &legacyOwnershipVectorDB{}
	result, err := StoreDocumentWithOwnership(fake, "concept", "content")
	if err != nil {
		t.Fatalf("StoreDocumentWithOwnership: %v", err)
	}
	if len(result.UnknownIDs) != 1 || result.UnknownIDs[0] != "legacy-1" || len(result.CreatedIDs) != 0 || len(result.ReusedIDs) != 0 {
		t.Fatalf("result = %+v, want unknown legacy ID", result)
	}
}

type legacyOwnershipVectorDB struct{}

func (legacyOwnershipVectorDB) StoreDocument(string, string) ([]string, error) {
	return []string{"legacy-1"}, nil
}
func (legacyOwnershipVectorDB) StoreDocumentWithEmbedding(string, string, []float32) (string, error) {
	return "", nil
}
func (legacyOwnershipVectorDB) StoreDocumentInCollection(string, string, string) ([]string, error) {
	return nil, nil
}
func (legacyOwnershipVectorDB) StoreDocumentWithEmbeddingInCollection(string, string, []float32, string) (string, error) {
	return "", nil
}
func (legacyOwnershipVectorDB) StoreBatch([]ArchiveItem) ([]string, error) { return nil, nil }
func (legacyOwnershipVectorDB) SearchSimilar(string, int, ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (legacyOwnershipVectorDB) SearchMemoriesOnly(string, int) ([]string, []string, error) {
	return nil, nil, nil
}
func (legacyOwnershipVectorDB) GetByIDFromCollection(string, string) (string, error) { return "", nil }
func (legacyOwnershipVectorDB) GetByID(string) (string, error)                       { return "", fmt.Errorf("missing") }
func (legacyOwnershipVectorDB) DeleteDocument(string) error                          { return nil }
func (legacyOwnershipVectorDB) DeleteDocumentFromCollection(string, string) error    { return nil }
func (legacyOwnershipVectorDB) Count() int                                           { return 0 }
func (legacyOwnershipVectorDB) IsDisabled() bool                                     { return false }
func (legacyOwnershipVectorDB) IsReady() bool                                        { return true }
func (legacyOwnershipVectorDB) Close() error                                         { return nil }
func (legacyOwnershipVectorDB) StoreCheatsheet(string, string, string, ...string) error {
	return nil
}
func (legacyOwnershipVectorDB) DeleteCheatsheet(string) error { return nil }
func (legacyOwnershipVectorDB) RegisterCollections([]string)  {}
