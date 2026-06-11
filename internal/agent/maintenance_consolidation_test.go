package agent

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

type dedupConsolidationVectorDB struct {
	seen map[string]bool
}

func (v *dedupConsolidationVectorDB) StoreDocument(concept, content string) ([]string, error) {
	if v.seen == nil {
		v.seen = map[string]bool{}
	}
	if v.seen[concept] {
		return nil, nil
	}
	v.seen[concept] = true
	return []string{concept}, nil
}
func (v *dedupConsolidationVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return concept, nil
}
func (v *dedupConsolidationVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) { return nil, nil }
func (v *dedupConsolidationVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *dedupConsolidationVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *dedupConsolidationVectorDB) GetByID(id string) (string, error) { return "", nil }
func (v *dedupConsolidationVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (v *dedupConsolidationVectorDB) DeleteDocument(id string) error { return nil }
func (v *dedupConsolidationVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return nil
}
func (v *dedupConsolidationVectorDB) Count() int       { return 0 }
func (v *dedupConsolidationVectorDB) IsDisabled() bool { return false }
func (v *dedupConsolidationVectorDB) IsReady() bool    { return true }
func (v *dedupConsolidationVectorDB) Close() error     { return nil }
func (v *dedupConsolidationVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (v *dedupConsolidationVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (v *dedupConsolidationVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (v *dedupConsolidationVectorDB) DeleteCheatsheet(id string) error         { return nil }
func (v *dedupConsolidationVectorDB) RegisterCollections(collections []string) {}

type partialFailConsolidationVectorDB struct {
	attempts  int
	stored    []string
	rolledBack []string
}

func (v *partialFailConsolidationVectorDB) StoreDocument(concept, content string) ([]string, error) {
	v.attempts++
	if v.attempts >= 2 {
		return nil, fmt.Errorf("simulated store failure for %s", concept)
	}
	v.stored = append(v.stored, concept)
	return []string{concept}, nil
}
func (v *partialFailConsolidationVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return concept, nil
}
func (v *partialFailConsolidationVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) { return nil, nil }
func (v *partialFailConsolidationVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *partialFailConsolidationVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *partialFailConsolidationVectorDB) GetByID(id string) (string, error) { return "", nil }
func (v *partialFailConsolidationVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (v *partialFailConsolidationVectorDB) DeleteDocument(id string) error {
	v.rolledBack = append(v.rolledBack, id)
	return nil
}
func (v *partialFailConsolidationVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return nil
}
func (v *partialFailConsolidationVectorDB) Count() int       { return 0 }
func (v *partialFailConsolidationVectorDB) IsDisabled() bool { return false }
func (v *partialFailConsolidationVectorDB) IsReady() bool    { return true }
func (v *partialFailConsolidationVectorDB) Close() error     { return nil }
func (v *partialFailConsolidationVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (v *partialFailConsolidationVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (v *partialFailConsolidationVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (v *partialFailConsolidationVectorDB) DeleteCheatsheet(id string) error         { return nil }
func (v *partialFailConsolidationVectorDB) RegisterCollections(collections []string) {}

func TestShouldMarkConsolidationSuccess(t *testing.T) {
	tests := []struct {
		name       string
		stored     int
		skipped    int
		factCount  int
		validFacts int
		wantOK     bool
		wantReason string
	}{
		{name: "empty facts", factCount: 0, wantOK: false, wantReason: "no_facts_extracted"},
		{name: "stored facts", stored: 2, factCount: 2, validFacts: 2, wantOK: true},
		{name: "dedup only", skipped: 2, factCount: 2, validFacts: 2, wantOK: true, wantReason: "all_duplicates"},
		{name: "nothing stored", factCount: 2, validFacts: 2, wantOK: false, wantReason: "no_facts_stored"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := shouldMarkConsolidationSuccess(tc.stored, tc.skipped, tc.factCount, tc.validFacts)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestStoreConsolidationFactsRollsBackOnPartialFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	vdb := &partialFailConsolidationVectorDB{}
	facts := []helperConsolidationFact{
		{Concept: "fact-one", Content: "First fact."},
		{Concept: "fact-two", Content: "Second fact."},
	}

	stored, skipped, err := storeConsolidationFacts(logger, stm, vdb, facts)
	if err == nil {
		t.Fatal("expected partial store failure")
	}
	if stored != 0 {
		t.Fatalf("stored = %d, want 0 after rollback", stored)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d, want 0", skipped)
	}
	if len(vdb.rolledBack) != 1 || vdb.rolledBack[0] != "fact-one" {
		t.Fatalf("rolledBack = %#v, want [fact-one]", vdb.rolledBack)
	}

	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	for _, meta := range metas {
		if meta.DocID == "fact-one" {
			t.Fatalf("expected rolled-back memory_meta removed, still have %#v", meta)
		}
	}
}

func TestStoreConsolidationFactsCountsDedupAsSkipped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	vdb := &dedupConsolidationVectorDB{}
	facts := []helperConsolidationFact{
		{Concept: "nas-backup", Content: "Backup target is the NAS."},
		{Concept: "nas-backup", Content: "Duplicate concept should be skipped."},
	}

	stored, skipped, err := storeConsolidationFacts(logger, stm, vdb, facts)
	if err != nil {
		t.Fatalf("storeConsolidationFacts: %v", err)
	}
	if stored != 1 {
		t.Fatalf("stored = %d, want 1", stored)
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
}

func archiveConsolidationFixture(t *testing.T, stm *memory.SQLiteMemory, sessionID string, messages []struct{ role, content string }) consolidationWorkItem {
	t.Helper()
	for _, msg := range messages {
		if _, err := stm.InsertMessage(sessionID, msg.role, msg.content, false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if err := stm.DeleteOldMessages(sessionID, 1); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}
	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(archived) == 0 {
		t.Fatal("expected archived messages")
	}
	ids := make([]int64, 0, len(archived))
	for _, msg := range archived {
		ids = append(ids, msg.ID)
	}
	return consolidationWorkItem{messageIDs: ids, messages: archived}
}

func TestFinalizeConsolidationBatchRejectsEmptyFacts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	item := archiveConsolidationFixture(t, stm, "s1", []struct{ role, content string }{
		{"user", "hello"},
		{"assistant", "hi"},
		{"user", "remember nas"},
	})

	ok, storedCount := finalizeConsolidationBatch(logger, stm, item, nil, 0, 0, nil, 1, 1)
	if ok {
		t.Fatal("expected empty facts batch to fail finalization")
	}
	if storedCount != 0 {
		t.Fatalf("storedCount = %d, want 0", storedCount)
	}

}

func TestFinalizeConsolidationBatchAcceptsDedupOnlyFacts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	item := archiveConsolidationFixture(t, stm, "s1", []struct{ role, content string }{
		{"user", "remember nas"},
		{"assistant", "noted"},
		{"user", "backup target"},
	})
	facts := []helperConsolidationFact{{Concept: "nas-backup", Content: "Backup target is the NAS."}}

	ok, storedCount := finalizeConsolidationBatch(logger, stm, item, facts, 0, 1, nil, 1, 1)
	if !ok {
		t.Fatal("expected dedup-only batch to finalize successfully")
	}
	if storedCount != 0 {
		t.Fatalf("storedCount = %d, want 0", storedCount)
	}

	remaining, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected archived messages marked consolidated, still have %d unconsolidated", len(remaining))
	}
}

func TestFinalizeConsolidationBatchReturnsFalseWhenMarkSuccessFails(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	item := archiveConsolidationFixture(t, stm, "s1", []struct{ role, content string }{
		{"user", "hello"},
		{"assistant", "hi"},
	})
	facts := []helperConsolidationFact{{Concept: "greeting", Content: "User said hello."}}

	if err := stm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ok, storedCount := finalizeConsolidationBatch(logger, stm, item, facts, 1, 0, nil, 1, 1)
	if ok {
		t.Fatal("expected finalize to fail when MarkConsolidationSuccess cannot run")
	}
	if storedCount != 0 {
		t.Fatalf("storedCount = %d, want 0", storedCount)
	}
}