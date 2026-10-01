package memory

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/dbutil"
)

func TestPendingMemoryMetadataDedupAndInvalidPayloads(t *testing.T) {
	stm := setupLearnedRulesTest(t)
	write := PendingMemoryWrite{Concept: "fact", Content: "content", Domain: "memory_analysis"}
	if err := stm.EnqueuePendingMemoryWrite(write, nil); err != nil {
		t.Fatal(err)
	}
	details := MemoryMetaUpdate{ExtractionConfidence: .99, SourceReliability: .85, SourceType: "memory_analysis", VerificationStatus: "unverified"}
	write.Metadata = &details
	if err := stm.EnqueuePendingMemoryWrite(write, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	due, err := stm.GetDuePendingMemoryWrites(now.Add(time.Minute), 20)
	if err != nil || len(due) != 1 || due[0].Metadata == nil || *due[0].Metadata != details {
		t.Fatalf("enriched queue = %+v, %v", due, err)
	}
	if err := stm.MarkPendingMemoryWriteFailed(due[0].ID, errors.New("down"), now); err != nil {
		t.Fatal(err)
	}
	other := details
	other.ExtractionConfidence = .5
	write.Metadata = &other
	if err := stm.EnqueuePendingMemoryWrite(write, nil); err != nil {
		t.Fatal(err)
	}
	if due, err := stm.GetDuePendingMemoryWrites(now.Add(time.Minute), 20); err != nil || len(due) != 0 {
		t.Fatalf("dedupe reset backoff: %v, %v", due, err)
	}
	due, err = stm.GetDuePendingMemoryWrites(now.Add(time.Hour), 20)
	if err != nil || len(due) != 1 || due[0].Attempts != 1 || *due[0].Metadata != details {
		t.Fatalf("dedupe changed first metadata: %+v, %v", due, err)
	}
	for _, bad := range []string{"broken", `{"version":2,"details":{}}`, `{"version":1,"details":null}`} {
		if _, err := stm.db.Exec(`UPDATE pending_memory_writes SET metadata_json=?`, bad); err != nil {
			t.Fatal(err)
		}
		due, err = stm.GetDuePendingMemoryWrites(now.Add(time.Hour), 20)
		if err != nil || len(due) != 1 || due[0].MetadataError == nil || due[0].Metadata != nil {
			t.Fatalf("invalid payload fell back: %+v, %v", due, err)
		}
	}
}

func TestPendingMemoryMetadataMigrationBacksUpAndPreservesQueue(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`DROP TABLE pending_memory_writes;
	CREATE TABLE pending_memory_writes (
	 id INTEGER PRIMARY KEY AUTOINCREMENT, concept TEXT NOT NULL, content TEXT NOT NULL,
	 domain TEXT NOT NULL DEFAULT '', attempts INTEGER NOT NULL DEFAULT 0,
	 next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, last_error TEXT NOT NULL DEFAULT '',
	 status TEXT NOT NULL DEFAULT 'pending', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(concept,content,domain));
	INSERT INTO pending_memory_writes (concept,content,attempts,next_attempt_at,last_error)
	VALUES ('legacy','content',2,datetime('now','+10 minutes'),'original error');`); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	stm, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	if err := stm.InitPendingMemoryWritesTable(); err != nil {
		t.Fatal(err)
	}
	due, err := stm.GetDuePendingMemoryWrites(time.Now().Add(time.Hour), 20)
	if err != nil || len(due) != 1 || due[0].Attempts != 2 || due[0].LastError != "original error" || due[0].Metadata != nil || due[0].MetadataError != nil {
		t.Fatalf("migrated queue = %+v, %v", due, err)
	}
	backups, err := filepath.Glob(path + ".pending-memory-metadata-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backups = %v, %v", backups, err)
	}
	backup, err := dbutil.Open(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var attempts, columns int
	if err := backup.QueryRow(`SELECT attempts FROM pending_memory_writes WHERE concept='legacy'`).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("backup lost queue: %d, %v", attempts, err)
	}
	if err := backup.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('pending_memory_writes') WHERE name='metadata_json'`).Scan(&columns); err != nil || columns != 0 {
		t.Fatalf("backup was made after migration: %d, %v", columns, err)
	}
}
