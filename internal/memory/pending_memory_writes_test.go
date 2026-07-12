package memory

import (
	"errors"
	"testing"
	"time"
)

func TestPendingMemoryWriteQueueDeduplicatesAndRetries(t *testing.T) {
	stm := setupLearnedRulesTest(t)
	now := time.Now().UTC()
	write := PendingMemoryWrite{
		Concept: "[fact] build failed",
		Content: "source:memory_analysis session:test",
		Domain:  "memory_analysis",
	}
	if err := stm.EnqueuePendingMemoryWrite(write, errors.New("embedding timeout")); err != nil {
		t.Fatalf("enqueue pending write: %v", err)
	}
	if err := stm.EnqueuePendingMemoryWrite(write, errors.New("embedding timeout again")); err != nil {
		t.Fatalf("deduplicate pending write: %v", err)
	}
	count, err := stm.CountPendingMemoryWrites()
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v, want 1", count, err)
	}

	due, err := stm.GetDuePendingMemoryWrites(now.Add(time.Minute), 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("due=%+v err=%v", due, err)
	}
	if err := stm.MarkPendingMemoryWriteFailed(due[0].ID, errors.New("still unavailable"), now); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	due, err = stm.GetDuePendingMemoryWrites(now.Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("get due during backoff: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("write retried before backoff elapsed: %+v", due)
	}
	due, err = stm.GetDuePendingMemoryWrites(now.Add(10*time.Minute), 10)
	if err != nil || len(due) != 1 || due[0].Attempts != 1 {
		t.Fatalf("due after backoff=%+v err=%v", due, err)
	}
	if err := stm.CompletePendingMemoryWrite(due[0].ID); err != nil {
		t.Fatalf("complete pending write: %v", err)
	}
	if count, _ := stm.CountPendingMemoryWrites(); count != 0 {
		t.Fatalf("pending count after completion = %d", count)
	}
}

func TestPendingMemoryWriteQueueStopsAfterAttemptLimit(t *testing.T) {
	stm := setupLearnedRulesTest(t)
	if err := stm.EnqueuePendingMemoryWrite(PendingMemoryWrite{Concept: "fact", Content: "content"}, errors.New("down")); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := stm.db.QueryRow(`SELECT id FROM pending_memory_writes`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`UPDATE pending_memory_writes SET attempts = 5, next_attempt_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if err := stm.MarkPendingMemoryWriteFailed(id, errors.New("final failure"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if due, err := stm.GetDuePendingMemoryWrites(time.Now().UTC().Add(48*time.Hour), 10); err != nil || len(due) != 0 {
		t.Fatalf("exhausted write remained retryable: due=%+v err=%v", due, err)
	}
}
