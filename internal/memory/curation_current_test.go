package memory

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestAutomaticCurationRechecksCurrentProtectionAndRevision(t *testing.T) {
	for _, change := range []string{"protect", "keep_forever", "archive", "confidence", "same-second-events"} {
		t.Run(change, func(t *testing.T) {
			s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if err := s.EnsureMemoryMetaWithDetails("doc", MemoryMetaUpdate{ExtractionConfidence: .2}); err != nil {
				t.Fatal(err)
			}
			if err := s.SetMemoryMetaLastAccessed("doc", time.Now().AddDate(0, 0, -180)); err != nil {
				t.Fatal(err)
			}
			metas, err := s.GetAllMemoryMeta(10, 0)
			if err != nil {
				t.Fatal(err)
			}
			plan := BuildMemoryCurationPlan(metas, MemoryUsageStats{}, MemoryCurationOptions{})
			if len(plan.AutoArchive) != 1 {
				t.Fatalf("plan=%+v", plan)
			}
			switch change {
			case "protect", "archive":
				if err := s.ApplyMemoryCurationAction(MemoryCurationAction{DocID: "doc", Action: change}, "admin", false); err != nil {
					t.Fatal(err)
				}
			case "keep_forever":
				if _, err := s.db.Exec(`UPDATE memory_meta SET keep_forever=1 WHERE doc_id='doc'`); err != nil {
					t.Fatal(err)
				}
			case "confidence":
				if _, err := s.db.Exec(`UPDATE memory_meta SET extraction_confidence=.99 WHERE doc_id='doc'`); err != nil {
					t.Fatal(err)
				}
			case "same-second-events":
				// Return every metadata field to its original value, retaining only the curation event revision.
				if err := s.ApplyMemoryCurationAction(MemoryCurationAction{DocID: "doc", Action: "protect"}, "admin", false); err != nil {
					t.Fatal(err)
				}
				if err := s.ApplyMemoryCurationAction(MemoryCurationAction{DocID: "doc", Action: "unprotect"}, "admin", false); err != nil {
					t.Fatal(err)
				}
				m := metas[0]
				if _, err := s.db.Exec(`UPDATE memory_meta SET last_event_at=?,last_reviewed_at=NULL,review_note='' WHERE doc_id='doc'`, m.LastEventAt); err != nil {
					t.Fatal(err)
				}
			}
			before, err := s.GetMemoryMeta("doc")
			if err != nil {
				t.Fatal(err)
			}
			changed, err := s.ApplyAutomaticMemoryCurationAction(plan.AutoArchive[0])
			if err != nil || changed {
				t.Fatalf("stale action changed=%v err=%v", changed, err)
			}
			after, err := s.GetMemoryMeta("doc")
			if err != nil || !memoryMetaEqual(before, after) {
				t.Fatalf("metadata changed: before=%+v after=%+v err=%v", before, after, err)
			}
		})
	}
}

func TestAutomaticCurationRequiresSnapshotAndReportsApplied(t *testing.T) {
	s, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if changed, err := s.ApplyAutomaticMemoryCurationAction(MemoryCurationAction{DocID: "doc", Action: "archive"}); err == nil || changed {
		t.Fatal("snapshot-free action accepted")
	}
	if err := s.EnsureMemoryMeta("doc"); err != nil {
		t.Fatal(err)
	}
	meta, err := s.GetMemoryMeta("doc")
	if err != nil {
		t.Fatal(err)
	}
	action := MemoryCurationAction{DocID: "doc", Action: "archive", ExpectedMeta: &meta}
	if changed, err := s.ApplyAutomaticMemoryCurationAction(action); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if changed, err := s.ApplyAutomaticMemoryCurationAction(action); err != nil || changed {
		t.Fatalf("replay changed=%v err=%v", changed, err)
	}
	// Explicit administrative reactivation remains available through its trusted entry point.
	if err := s.ApplyMemoryCurationAction(MemoryCurationAction{DocID: "doc", Action: "confirm"}, "admin", false); err != nil {
		t.Fatal(err)
	}
}
