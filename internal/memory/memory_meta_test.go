package memory

import (
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"testing"
)

func TestUpsertMemoryMetaWithDetailsPersistsQualityFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-quality", MemoryMetaUpdate{
		ExtractionConfidence: 0.93,
		VerificationStatus:   "confirmed",
		SourceType:           "memory_analysis",
		SourceReliability:    0.88,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}

	metas, err := stm.GetAllMemoryMeta(100, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("len(metas) = %d, want 1", len(metas))
	}
	meta := metas[0]
	if meta.DocID != "doc-quality" {
		t.Fatalf("DocID = %q, want doc-quality", meta.DocID)
	}
	if meta.ExtractionConfidence != 0.93 {
		t.Fatalf("ExtractionConfidence = %v, want 0.93", meta.ExtractionConfidence)
	}
	if meta.VerificationStatus != "confirmed" {
		t.Fatalf("VerificationStatus = %q, want confirmed", meta.VerificationStatus)
	}
	if meta.SourceType != "memory_analysis" {
		t.Fatalf("SourceType = %q, want memory_analysis", meta.SourceType)
	}
	if meta.SourceReliability != 0.88 {
		t.Fatalf("SourceReliability = %v, want 0.88", meta.SourceReliability)
	}
}

func TestUpsertMemoryMetaDefaultsQualityFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMeta("doc-default"); err != nil {
		t.Fatalf("UpsertMemoryMeta: %v", err)
	}

	metas, err := stm.GetAllMemoryMeta(100, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("len(metas) = %d, want 1", len(metas))
	}
	meta := metas[0]
	if meta.ExtractionConfidence != 0.75 {
		t.Fatalf("ExtractionConfidence = %v, want 0.75", meta.ExtractionConfidence)
	}
	if meta.VerificationStatus != "unverified" {
		t.Fatalf("VerificationStatus = %q, want unverified", meta.VerificationStatus)
	}
	if meta.SourceType != "system" {
		t.Fatalf("SourceType = %q, want system", meta.SourceType)
	}
	if meta.SourceReliability != 0.70 {
		t.Fatalf("SourceReliability = %v, want 0.70", meta.SourceReliability)
	}
}

func TestRecordMemoryEffectivenessPersistsCounters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.RecordMemoryEffectiveness("doc-effectiveness", true); err != nil {
		t.Fatalf("RecordMemoryEffectiveness useful: %v", err)
	}
	if err := stm.RecordMemoryEffectiveness("doc-effectiveness", true); err != nil {
		t.Fatalf("RecordMemoryEffectiveness useful repeat: %v", err)
	}
	if err := stm.RecordMemoryEffectiveness("doc-effectiveness", false); err != nil {
		t.Fatalf("RecordMemoryEffectiveness useless: %v", err)
	}

	metas, err := stm.GetAllMemoryMeta(100, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("len(metas) = %d, want 1", len(metas))
	}
	meta := metas[0]
	if meta.UsefulCount != 2 {
		t.Fatalf("UsefulCount = %d, want 2", meta.UsefulCount)
	}
	if meta.UselessCount != 1 {
		t.Fatalf("UselessCount = %d, want 1", meta.UselessCount)
	}
	if meta.LastEffectivenessAt == "" {
		t.Fatal("LastEffectivenessAt should be populated")
	}
}

func TestCleanupDeletedVectorDocumentReferencesPreservesMemoryMeta(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	docID := "doc-cleanup"
	if err := stm.UpsertMemoryMeta(docID); err != nil {
		t.Fatalf("UpsertMemoryMeta: %v", err)
	}
	if _, err := stm.db.Exec(`INSERT INTO file_embedding_docs (file_path, collection, doc_id) VALUES (?, ?, ?)`, "notes.md", "docs", docID); err != nil {
		t.Fatalf("insert file_embedding_docs: %v", err)
	}
	if _, err := stm.db.Exec(`INSERT INTO memory_conflicts (doc_id_left, doc_id_right, conflict_key, status) VALUES (?, ?, ?, ?)`, docID, "other-doc", "duplicate", "open"); err != nil {
		t.Fatalf("insert memory_conflicts: %v", err)
	}

	if err := stm.CleanupDeletedVectorDocumentReferences(docID); err != nil {
		t.Fatalf("CleanupDeletedVectorDocumentReferences: %v", err)
	}

	var metaCount int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_meta WHERE doc_id = ?`, docID).Scan(&metaCount); err != nil {
		t.Fatalf("count memory_meta: %v", err)
	}
	if metaCount != 1 {
		t.Fatalf("memory_meta rows = %d, want preserved row", metaCount)
	}

	var fileRows int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM file_embedding_docs WHERE doc_id = ?`, docID).Scan(&fileRows); err != nil {
		t.Fatalf("count file_embedding_docs: %v", err)
	}
	if fileRows != 0 {
		t.Fatalf("file_embedding_docs rows = %d, want 0", fileRows)
	}

	var conflictRows int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_conflicts WHERE doc_id_left = ? OR doc_id_right = ?`, docID, docID).Scan(&conflictRows); err != nil {
		t.Fatalf("count memory_conflicts: %v", err)
	}
	if conflictRows != 0 {
		t.Fatalf("memory_conflicts rows = %d, want 0", conflictRows)
	}
}

func TestCleanupDeletedVectorDocumentReferencesBatchNormalizesWithoutMutatingInput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	docIDs := []string{" left-id ", "", "right-id", "left-id", " right-id "}
	wantInput := append([]string(nil), docIDs...)
	for _, docID := range []string{"left-id", "right-id"} {
		if _, err := stm.db.Exec(
			`INSERT INTO file_embedding_docs (file_path, collection, doc_id) VALUES (?, ?, ?)`,
			docID+".md", "docs", docID,
		); err != nil {
			t.Fatalf("insert file_embedding_docs %s: %v", docID, err)
		}
	}
	for i, pair := range [][2]string{
		{"left-id", "outside-a"},
		{"outside-b", "left-id"},
		{"right-id", "outside-c"},
		{"outside-d", "right-id"},
		{"keep-left", "keep-right"},
	} {
		if _, err := stm.db.Exec(`
			INSERT INTO memory_conflicts (doc_id_left, doc_id_right, conflict_key, status)
			VALUES (?, ?, ?, 'open')
		`, pair[0], pair[1], fmt.Sprintf("conflict-%d", i)); err != nil {
			t.Fatalf("insert memory_conflicts %d: %v", i, err)
		}
	}

	if err := stm.CleanupDeletedVectorDocumentReferencesBatch(docIDs); err != nil {
		t.Fatalf("CleanupDeletedVectorDocumentReferencesBatch: %v", err)
	}
	if !reflect.DeepEqual(docIDs, wantInput) {
		t.Fatalf("input mutated: got %#v, want %#v", docIDs, wantInput)
	}

	var fileRows int
	if err := stm.db.QueryRow(`
		SELECT COUNT(*) FROM file_embedding_docs WHERE doc_id IN ('left-id', 'right-id')
	`).Scan(&fileRows); err != nil {
		t.Fatalf("count file_embedding_docs: %v", err)
	}
	if fileRows != 0 {
		t.Fatalf("file_embedding_docs rows = %d, want 0", fileRows)
	}

	var matchingConflicts int
	if err := stm.db.QueryRow(`
		SELECT COUNT(*) FROM memory_conflicts
		WHERE doc_id_left IN ('left-id', 'right-id')
		   OR doc_id_right IN ('left-id', 'right-id')
	`).Scan(&matchingConflicts); err != nil {
		t.Fatalf("count matching memory_conflicts: %v", err)
	}
	if matchingConflicts != 0 {
		t.Fatalf("matching memory_conflicts rows = %d, want 0", matchingConflicts)
	}

	var keptConflicts int
	if err := stm.db.QueryRow(`
		SELECT COUNT(*) FROM memory_conflicts
		WHERE doc_id_left = 'keep-left' AND doc_id_right = 'keep-right'
	`).Scan(&keptConflicts); err != nil {
		t.Fatalf("count kept memory_conflicts: %v", err)
	}
	if keptConflicts != 1 {
		t.Fatalf("kept memory_conflicts rows = %d, want 1", keptConflicts)
	}
}

func TestUpsertMemoryMetaConcurrentSameDocID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	const workers = 12
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := stm.UpsertMemoryMeta("doc-concurrent"); err != nil {
				t.Errorf("UpsertMemoryMeta: %v", err)
			}
		}()
	}
	wg.Wait()

	var count int
	if err := stm.db.QueryRow("SELECT COUNT(*) FROM memory_meta WHERE doc_id = 'doc-concurrent'").Scan(&count); err != nil {
		t.Fatalf("count memory_meta rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("memory_meta rows = %d, want 1", count)
	}
}
