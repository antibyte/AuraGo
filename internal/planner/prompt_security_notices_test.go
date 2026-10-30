package planner

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPromptSecurityNoticeLeaseSurvivesRestartAndRetriesInterruptedDelivery(t *testing.T) {
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
	now := time.Now()
	if _, err := RecordOperationalIssue(db, OperationalIssue{Source: OperationalIssueSourcePromptSecurity, Fingerprint: "check", Title: "Internal prompt check", Severity: "warning", OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	notices, err := ListPendingPromptSecurityNotices(db, now, 2)
	if err != nil || len(notices) != 1 {
		t.Fatalf("%v %v", notices, err)
	}
	var claimed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		conn := db
		if i%2 != 0 {
			conn = other
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := ClaimPromptSecurityNoticeBatch(conn, notices, now)
			if err != nil {
				t.Error(err)
			}
			if got {
				claimed.Add(1)
			}
		}()
	}
	wg.Wait()
	if claimed.Load() != 1 {
		t.Fatalf("%d concurrent contacts claimed one batch", claimed.Load())
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got, err := ClaimPromptSecurityNoticeBatch(db, notices, now.Add(time.Minute)); err != nil || got {
		t.Fatalf("lease lost on restart: %v %v", got, err)
	}
	if pending, err := ListPendingPromptSecurityNotices(db, now.Add(time.Minute), 2); err != nil || len(pending) != 1 {
		t.Fatalf("lease falsely acknowledged delivery: %v %v", pending, err)
	}
	if got, err := ClaimPromptSecurityNoticeBatch(db, notices, now.Add(6*time.Minute)); err != nil || !got {
		t.Fatalf("interrupted delivery cannot retry: %v %v", got, err)
	}
	if err := MarkOperationalIssuesNotified(db, []OperationalIssueNoticeRef{{Fingerprint: notices[0].Fingerprint, Revision: notices[0].Revision}}, now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if pending, err := ListPendingPromptSecurityNotices(db, now.Add(7*time.Minute), 2); err != nil || len(pending) != 0 {
		t.Fatalf("delivered notice remained pending: %v %v", pending, err)
	}
}
