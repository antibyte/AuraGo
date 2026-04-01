package memory

import (
	"io"
	"log/slog"
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
