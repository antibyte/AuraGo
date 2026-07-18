package memory

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestAgoDeskKnowledgeLedgerPersistsTransitionsAndMetadata(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "short-term.db")
	stm, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	record := AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-ledger",
		PrepareID:          "prepare-ledger",
		PrepareFingerprint: "fingerprint",
		OwnerDeviceID:      "device-a",
		Filename:           "manual.txt",
		StoragePath:        filepath.Join(t.TempDir(), "manual.txt"),
		Collection:         "file_index",
		Title:              "Manual",
		Tags:               []string{"docs", "test"},
		DeclaredMime:       "text/plain",
		DeclaredSizeBytes:  12,
		CreatedAt:          now,
		ExpiresAt:          now.Add(5 * time.Minute),
	}
	if err := stm.PrepareAgoDeskKnowledgeBatch([]AgoDeskKnowledgeDocument{record}); err != nil {
		t.Fatalf("PrepareAgoDeskKnowledgeBatch: %v", err)
	}
	if _, err := stm.MarkAgoDeskKnowledgeUploading(record.DocumentID, now.Add(time.Second)); err != nil {
		t.Fatalf("MarkAgoDeskKnowledgeUploading: %v", err)
	}
	if _, err := stm.MarkAgoDeskKnowledgeProcessing(record.DocumentID, "text/plain", 12, "abc", now.Add(2*time.Second)); err != nil {
		t.Fatalf("MarkAgoDeskKnowledgeProcessing: %v", err)
	}
	if err := stm.UpsertFileIndexMetadata(record.StoragePath, record.Collection, map[string]string{
		"archive_document_id": record.DocumentID,
		"archive_title":       record.Title,
	}); err != nil {
		t.Fatalf("UpsertFileIndexMetadata: %v", err)
	}
	if _, err := stm.MarkAgoDeskKnowledgeReady(record.DocumentID, 3, now.Add(3*time.Second)); err != nil {
		t.Fatalf("MarkAgoDeskKnowledgeReady: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("reopen NewSQLiteMemory: %v", err)
	}
	defer reopened.Close()
	got, err := reopened.GetAgoDeskKnowledgeDocument(record.DocumentID)
	if err != nil {
		t.Fatalf("GetAgoDeskKnowledgeDocument: %v", err)
	}
	if got == nil || got.Status != AgoDeskKnowledgeStatusReady || got.ChunkCount != 3 ||
		len(got.Tags) != 2 || got.Tags[0] != "docs" || got.OwnerDeviceID != "device-a" {
		t.Fatalf("persisted document = %+v", got)
	}
	metadata, err := reopened.GetFileIndexMetadata(record.StoragePath, record.Collection)
	if err != nil {
		t.Fatalf("GetFileIndexMetadata: %v", err)
	}
	if metadata["archive_document_id"] != record.DocumentID || metadata["archive_title"] != "Manual" {
		t.Fatalf("persisted metadata = %+v", metadata)
	}
	replay, err := reopened.ListAgoDeskKnowledgeReplay("device-a", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListAgoDeskKnowledgeReplay: %v", err)
	}
	if len(replay) != 1 || replay[0].DocumentID != record.DocumentID {
		t.Fatalf("replay = %+v", replay)
	}
	other, err := reopened.ListAgoDeskKnowledgeReplay("device-b", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListAgoDeskKnowledgeReplay other owner: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("other owner replay = %+v", other)
	}
}

func TestAgoDeskKnowledgeLedgerExpiresPreparedDocuments(t *testing.T) {
	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	now := time.Now().UTC()
	record := AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-expired",
		PrepareID:          "prepare-expired",
		PrepareFingerprint: "fingerprint",
		OwnerDeviceID:      "device-a",
		Filename:           "old.txt",
		StoragePath:        filepath.Join(t.TempDir(), "old.txt"),
		Collection:         "file_index",
		Title:              "Old",
		DeclaredMime:       "text/plain",
		DeclaredSizeBytes:  1,
		CreatedAt:          now.Add(-10 * time.Minute),
		ExpiresAt:          now.Add(-time.Minute),
	}
	if err := stm.PrepareAgoDeskKnowledgeBatch([]AgoDeskKnowledgeDocument{record}); err != nil {
		t.Fatalf("PrepareAgoDeskKnowledgeBatch: %v", err)
	}
	expired, err := stm.ExpireAgoDeskKnowledgeDocuments(now, "KNOWLEDGE_EXPIRED")
	if err != nil {
		t.Fatalf("ExpireAgoDeskKnowledgeDocuments: %v", err)
	}
	if len(expired) != 1 || expired[0].Status != AgoDeskKnowledgeStatusFailed || expired[0].ErrorCode != "KNOWLEDGE_EXPIRED" {
		t.Fatalf("expired = %+v", expired)
	}
	if _, err := stm.MarkAgoDeskKnowledgeUploading(record.DocumentID, now); err == nil {
		t.Fatal("expired document unexpectedly transitioned back to uploading")
	}
}

func TestAgoDeskKnowledgeLedgerReservesBatchPathsAtomically(t *testing.T) {
	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	now := time.Now().UTC()
	storagePath := filepath.Join(t.TempDir(), "shared.txt")
	first := AgoDeskKnowledgeDocument{
		DocumentID:         "kdoc-first",
		PrepareID:          "prepare-first",
		PrepareFingerprint: "fingerprint-first",
		OwnerDeviceID:      "device-a",
		Filename:           "shared.txt",
		StoragePath:        storagePath,
		Collection:         "file_index",
		Title:              "First",
		DeclaredMime:       "text/plain",
		DeclaredSizeBytes:  1,
		CreatedAt:          now,
		ExpiresAt:          now.Add(5 * time.Minute),
	}
	second := first
	second.DocumentID = "kdoc-second"
	second.PrepareID = "prepare-second"
	second.PrepareFingerprint = "fingerprint-second"
	second.OwnerDeviceID = "device-b"

	if err := stm.PrepareAgoDeskKnowledgeBatch([]AgoDeskKnowledgeDocument{first}); err != nil {
		t.Fatalf("prepare first batch: %v", err)
	}
	if err := stm.PrepareAgoDeskKnowledgeBatch([]AgoDeskKnowledgeDocument{second}); err == nil {
		t.Fatal("second batch reserved the same storage path")
	}
	secondRows, err := stm.ListAgoDeskKnowledgeByPrepare(second.OwnerDeviceID, second.PrepareID)
	if err != nil {
		t.Fatalf("list rejected second batch: %v", err)
	}
	if len(secondRows) != 0 {
		t.Fatalf("rejected batch persisted partial rows: %+v", secondRows)
	}

	if _, err := stm.MarkAgoDeskKnowledgeFailed(first.DocumentID, "KNOWLEDGE_REJECTED", "failed", now); err != nil {
		t.Fatalf("fail first reservation: %v", err)
	}
	if err := stm.PrepareAgoDeskKnowledgeBatch([]AgoDeskKnowledgeDocument{second}); err != nil {
		t.Fatalf("storage reservation was not released after failure: %v", err)
	}
}
