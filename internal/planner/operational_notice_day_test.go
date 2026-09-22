package planner

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOperationalNoticeDayClaimPersistsAndSerializesConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planner.db")
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	other, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	now := time.Date(2030, 9, 22, 23, 59, 0, 0, time.FixedZone("local", 2*60*60))
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn := db
			if i%2 == 1 {
				conn = other
			}
			claimed, err := ClaimOperationalIssueReminderForDay(conn, now)
			if err != nil {
				t.Error(err)
				return
			}
			if claimed {
				winners.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("got %d daily winners, want 1", winners.Load())
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if claimed, err := ClaimOperationalIssueReminderForDay(db, now); err != nil || claimed {
		t.Fatalf("restart allowed another notice: %t %v", claimed, err)
	}
	if claimed, err := ClaimOperationalIssueReminderForDay(db, now.Add(2*time.Minute)); err != nil || !claimed {
		t.Fatalf("local midnight did not open next day: %t %v", claimed, err)
	}
	if claimed, err := ClaimOperationalIssueReminderForDay(db, now); err != nil || claimed {
		t.Fatalf("late previous-day caller rewound the gate: %t %v", claimed, err)
	}
}

func TestOperationalNoticeDayHonorsLegacyDeliveryInLocalDay(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	now := time.Date(2030, 9, 22, 8, 0, 0, 0, time.FixedZone("local", 2*60*60))
	id, err := RecordOperationalIssue(db, OperationalIssue{Source: "heartbeat", Reference: "loopback", Title: "Timeout", Severity: "error", OccurredAt: now.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	// This delivery is on the previous UTC date, but on today's local date.
	notices, err := ListPendingOperationalIssueNotices(db, now, 2)
	if err != nil || len(notices) != 1 {
		t.Fatalf("%v %v", notices, err)
	}
	delivered := time.Date(2030, 9, 21, 23, 15, 0, 0, time.UTC)
	if err := MarkOperationalIssuesNotified(db, []OperationalIssueNoticeRef{{Fingerprint: id, Revision: notices[0].Revision}}, delivered); err != nil {
		t.Fatal(err)
	}
	if claimed, err := ClaimOperationalIssueReminderForDay(db, now); err != nil || claimed {
		t.Fatalf("ignored legacy delivery: %t %v", claimed, err)
	}
	if _, err := db.Exec(`DELETE FROM operational_issues`); err != nil {
		t.Fatal(err)
	}
	if claimed, err := ClaimOperationalIssueReminderForDay(db, now); err != nil || claimed {
		t.Fatalf("issue cleanup reset the daily slot: %t %v", claimed, err)
	}
}
