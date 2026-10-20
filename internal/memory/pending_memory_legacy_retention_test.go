package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestLegacyPendingMemoryWritesKeepSessionTombstonesAcrossCleanupAndPurge(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	terminalAt := time.Now().UTC().Add(-31 * 24 * time.Hour)
	metadataWrite := PendingMemoryWrite{
		Concept: "legacy metadata fact", Content: "legacy metadata payload", Domain: "memory_analysis",
		Metadata: &MemoryMetaUpdate{SourceSessionID: "legacy-session-a", SourceType: "memory_analysis"},
	}
	envelopeWrite := PendingMemoryWrite{
		Concept: "legacy envelope fact", Content: "source:memory_analysis session:legacy-session-b", Domain: "memory_analysis",
	}
	if err := insertLegacyExhaustedPendingMemoryWrite(t, stm, metadataWrite, terminalAt); err != nil {
		t.Fatal(err)
	}
	if err := insertLegacyExhaustedPendingMemoryWrite(t, stm, envelopeWrite, terminalAt); err != nil {
		t.Fatal(err)
	}

	// A duplicate enqueue must leave old exhausted rows terminal even before
	// their payloads have reached the 30-day tombstone cleanup.
	for _, write := range []PendingMemoryWrite{metadataWrite, envelopeWrite} {
		if err := stm.EnqueuePendingMemoryWriteContext(context.Background(), write, nil); err != nil {
			t.Fatalf("duplicate enqueue before cleanup for %q: %v", write.Concept, err)
		}
		var status string
		var attempts int
		if err := stm.db.QueryRow(`SELECT status, attempts FROM pending_memory_writes WHERE concept=?`, write.Concept).Scan(&status, &attempts); err != nil {
			t.Fatal(err)
		}
		if status != "exhausted" || attempts != pendingMemoryWriteMaxAttempts {
			t.Fatalf("duplicate enqueue reactivated %q: status=%q attempts=%d", write.Concept, status, attempts)
		}
	}

	if deleted, err := stm.CleanupExpiredPendingMemoryWrites(context.Background(), time.Now().UTC(), 100); err != nil || deleted != 2 {
		t.Fatalf("legacy cleanup deleted=%d err=%v; want two rows", deleted, err)
	}

	metadataFingerprint := pendingMemoryWriteFingerprint(metadataWrite, PendingMemorySessionFingerprint("legacy-session-a"))
	envelopeFingerprint := pendingMemoryWriteFingerprint(envelopeWrite, PendingMemorySessionFingerprint("legacy-session-b"))
	for _, expected := range []struct {
		fingerprint string
		sessionHash string
	}{
		{metadataFingerprint, PendingMemorySessionFingerprint("legacy-session-a")},
		{envelopeFingerprint, PendingMemorySessionFingerprint("legacy-session-b")},
	} {
		var sessionHash string
		if err := stm.db.QueryRow(`SELECT source_session_hash FROM pending_memory_write_tombstones WHERE fingerprint=?`, expected.fingerprint).Scan(&sessionHash); err != nil {
			var actual []struct{ fingerprint, sessionHash string }
			rows, queryErr := stm.db.Query(`SELECT fingerprint,source_session_hash FROM pending_memory_write_tombstones ORDER BY fingerprint`)
			if queryErr == nil {
				for rows.Next() {
					var item struct{ fingerprint, sessionHash string }
					if scanErr := rows.Scan(&item.fingerprint, &item.sessionHash); scanErr != nil {
						queryErr = scanErr
						break
					}
					actual = append(actual, item)
				}
				queryErr = errors.Join(queryErr, rows.Err(), rows.Close())
			}
			t.Fatalf("read normalized tombstone %q: %v; actual=%+v query_error=%v", expected.fingerprint, err, actual, queryErr)
		}
		if sessionHash != expected.sessionHash {
			t.Fatalf("tombstone source hash=%q; want %q", sessionHash, expected.sessionHash)
		}
	}

	for _, write := range []PendingMemoryWrite{metadataWrite, envelopeWrite} {
		if err := stm.EnqueuePendingMemoryWriteContext(context.Background(), write, nil); !errors.Is(err, ErrAutomaticMemorySkipped) {
			t.Fatalf("replay after cleanup for %q error=%v; want exhausted tombstone rejection", write.Concept, err)
		}
	}

	if err := stm.PurgeChatSession("legacy-session-a"); err != nil {
		t.Fatalf("purge legacy session: %v", err)
	}
	var purged, retained int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_write_tombstones WHERE fingerprint=?`, metadataFingerprint).Scan(&purged); err != nil {
		t.Fatal(err)
	}
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_write_tombstones WHERE fingerprint=?`, envelopeFingerprint).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if purged != 0 || retained != 1 {
		t.Fatalf("session purge removed wrong tombstones: target=%d other=%d", purged, retained)
	}
}

func insertLegacyExhaustedPendingMemoryWrite(t *testing.T, stm *SQLiteMemory, write PendingMemoryWrite, terminalAt time.Time) error {
	t.Helper()
	var metadata any
	if write.Metadata != nil {
		encoded, err := json.Marshal(pendingMemoryWriteMetadata{Version: 1, Details: write.Metadata})
		if err != nil {
			return err
		}
		metadata = string(encoded)
	}
	_, err := stm.db.Exec(`
		INSERT INTO pending_memory_writes
			(concept,content,domain,metadata_json,attempts,status,terminal_at)
		VALUES(?,?,?,?,?,'exhausted',?)`,
		write.Concept, write.Content, write.Domain, metadata, pendingMemoryWriteMaxAttempts, terminalAt)
	return err
}

func TestLegacyPendingMemoryWriteWithoutSourceRetainsContentFreeTombstone(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	write := PendingMemoryWrite{Concept: "legacy unscoped fact", Content: "legacy unscoped payload", Domain: "test"}
	if err := insertLegacyExhaustedPendingMemoryWrite(t, stm, write, time.Now().UTC().Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := stm.CleanupExpiredPendingMemoryWrites(context.Background(), time.Now().UTC(), 100); err != nil || deleted != 1 {
		t.Fatalf("legacy cleanup deleted=%d err=%v", deleted, err)
	}
	fingerprint := pendingMemoryWriteFingerprint(write, "")
	var count int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_write_tombstones WHERE fingerprint=? AND source_session_hash=''`, fingerprint).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unscoped legacy tombstone count=%d err=%v", count, err)
	}
}

func TestLegacyPendingMemoryCleanupNormalizesOnlyExpiredBatch(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	now := time.Now().UTC()
	oldest := PendingMemoryWrite{Concept: "oldest legacy", Content: "source:memory_analysis session:batch-a", Domain: "memory_analysis"}
	second := PendingMemoryWrite{Concept: "second legacy", Content: "source:memory_analysis session:batch-b", Domain: "memory_analysis"}
	fresh := PendingMemoryWrite{Concept: "fresh legacy", Content: "source:memory_analysis session:batch-c", Domain: "memory_analysis"}
	for _, row := range []struct {
		write      PendingMemoryWrite
		terminalAt time.Time
	}{
		{oldest, now.Add(-40 * 24 * time.Hour)},
		{second, now.Add(-35 * 24 * time.Hour)},
		{fresh, now.Add(-2 * 24 * time.Hour)},
	} {
		if err := insertLegacyExhaustedPendingMemoryWrite(t, stm, row.write, row.terminalAt); err != nil {
			t.Fatal(err)
		}
	}

	if deleted, err := stm.CleanupExpiredPendingMemoryWrites(context.Background(), now, 1); err != nil || deleted != 1 {
		t.Fatalf("bounded legacy cleanup deleted=%d err=%v; want one row", deleted, err)
	}
	for _, row := range []struct {
		write   PendingMemoryWrite
		removed bool
	}{{oldest, true}, {second, false}, {fresh, false}} {
		var count int
		if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes WHERE concept=?`, row.write.Concept).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if (count == 0) != row.removed {
			t.Fatalf("legacy row %q present count=%d; removed=%v", row.write.Concept, count, row.removed)
		}
		if !row.removed {
			var fingerprint, sessionHash string
			if err := stm.db.QueryRow(`SELECT source_fingerprint,source_session_hash FROM pending_memory_writes WHERE concept=?`, row.write.Concept).Scan(&fingerprint, &sessionHash); err != nil {
				t.Fatal(err)
			}
			if fingerprint != "" || sessionHash != "" {
				t.Fatalf("out-of-batch legacy row %q was normalized: fingerprint=%q session_hash=%q", row.write.Concept, fingerprint, sessionHash)
			}
		}
	}
}
