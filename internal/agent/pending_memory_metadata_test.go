package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dbutil"
	"aurago/internal/memory"
)

func TestAuditQueuedAnalysisPreservesExtractionConfidence(t *testing.T) {
	for _, kind := range []string{"fact", "preference", "correction"} {
		t.Run(kind, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			path := filepath.Join(t.TempDir(), "memory.db")
			stm, err := memory.NewSQLiteMemory(path, logger)
			if err != nil {
				t.Fatal(err)
			}
			result := memoryAnalysisResult{}
			fact := extractedFact{Content: "The NAS uses an XFS filesystem.", Category: "fact", Confidence: .99}
			switch kind {
			case "fact":
				result.Facts = []extractedFact{fact}
			case "preference":
				result.Preferences = []extractedFact{fact}
			case "correction":
				result.Corrections = []extractedFact{fact}
			}
			vdb := &fakeVectorDB{storeErr: errors.New("provider unavailable")}
			applyMemoryAnalysisResult(&config.Config{}, logger, stm, vdb, "synthetic-session", result)
			if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 1 {
				t.Fatalf("queue count=%d err=%v", count, err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			stm, err = memory.NewSQLiteMemory(path, logger)
			if err != nil {
				t.Fatal(err)
			}
			defer stm.Close()
			vdb.storeErr = nil
			if succeeded, failed := retryPendingMemoryWrites(context.Background(), logger, stm, vdb); succeeded != 1 || failed != 0 {
				t.Fatalf("retry %d successful, %d failed", succeeded, failed)
			}
			meta, err := stm.GetMemoryMeta("stored-doc")
			wantReliability := .85
			if kind == "correction" {
				wantReliability = .90
			}
			if err != nil || meta.ExtractionConfidence != .99 || meta.SourceReliability != wantReliability || meta.SourceType != "memory_analysis" || meta.VerificationStatus != "unverified" {
				t.Fatalf("retry metadata=%+v err=%v", meta, err)
			}
		})
	}
}

func TestPendingMemoryMetadataRetryRejectsCorruptionAndPreservesCuration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	details := memory.MemoryMetaUpdate{ExtractionConfidence: .99, VerificationStatus: "unverified", SourceType: "memory_analysis", SourceReliability: .85}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: "fact", Content: "content", Metadata: &details}, nil); err != nil {
		t.Fatal(err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE pending_memory_writes SET metadata_json='broken'`); err != nil {
		t.Fatal(err)
	}
	vdb := &fakeVectorDB{}
	if succeeded, failed := retryPendingMemoryWrites(context.Background(), logger, stm, vdb); succeeded != 0 || failed != 1 || len(vdb.storedConcepts) != 0 {
		t.Fatalf("corrupt retry wrote vectors: %d/%d %v", succeeded, failed, vdb.storedConcepts)
	}
	if _, err := db.Exec(`DELETE FROM pending_memory_writes`); err != nil {
		t.Fatal(err)
	}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: "fact", Content: "content", Metadata: &details}, nil); err != nil {
		t.Fatal(err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("stored-doc", memory.MemoryMetaUpdate{ExtractionConfidence: .88, VerificationStatus: "confirmed", SourceType: "user", SourceReliability: 1}); err != nil {
		t.Fatal(err)
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "stored-doc", Action: memory.MemoryCurationActionProtect}, "admin", false); err != nil {
		t.Fatal(err)
	}
	before, err := stm.GetMemoryMeta("stored-doc")
	if err != nil {
		t.Fatal(err)
	}
	if succeeded, failed := retryPendingMemoryWrites(context.Background(), logger, stm, vdb); succeeded != 1 || failed != 0 {
		t.Fatalf("curated retry = %d/%d", succeeded, failed)
	}
	after, err := stm.GetMemoryMeta("stored-doc")
	if err != nil || before.ExtractionConfidence != after.ExtractionConfidence || !after.Protected || after.VerificationStatus != "confirmed" || after.SourceType != "user" {
		t.Fatalf("retry overwrote curation: before=%+v after=%+v err=%v", before, after, err)
	}
	if due, err := stm.GetDuePendingMemoryWrites(time.Now().Add(time.Hour), 20); err != nil || len(due) != 0 {
		t.Fatalf("completed queue=%v err=%v", due, err)
	}
}
