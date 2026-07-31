package planner

import (
	"sync"
	"testing"
	"time"
)

func TestOperationalIssueArchiveThresholdsAndReviewException(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		issue       OperationalIssue
		repeat      bool
		wantArchive bool
	}{
		{
			name: "single warning after seven days",
			issue: OperationalIssue{
				Fingerprint: "heartbeat|one-off", Source: "heartbeat", Context: "heartbeat",
				Title: "One-off warning", Detail: "temporary warning", Severity: "warning",
				OccurredAt: now.Add(-7 * 24 * time.Hour),
			},
			wantArchive: true,
		},
		{
			name: "recurring warning waits thirty days",
			issue: OperationalIssue{
				Fingerprint: "heartbeat|recurring", Source: "heartbeat", Context: "heartbeat",
				Title: "Recurring warning", Detail: "temporary warning", Severity: "warning",
				OccurredAt: now.Add(-8 * 24 * time.Hour),
			},
			repeat: true,
		},
		{
			name: "error waits thirty days",
			issue: OperationalIssue{
				Fingerprint: "maintenance|error", Source: "maintenance", Context: "nightly",
				Title: "Maintenance failed", Detail: "operation failed", Severity: "error",
				OccurredAt: now.Add(-29 * 24 * time.Hour),
			},
		},
		{
			name: "review remains active",
			issue: OperationalIssue{
				Fingerprint: "memory|review", Source: "memory_reflect", Context: "review",
				Title: "Review blocked", Detail: "User decision required.", Severity: "warning",
				OccurredAt: now.Add(-90 * 24 * time.Hour),
			},
		},
		{
			name: "old recurring warning after thirty days",
			issue: OperationalIssue{
				Fingerprint: "heartbeat|old-recurring", Source: "heartbeat", Context: "heartbeat",
				Title: "Old recurring warning", Detail: "still failed", Severity: "warning",
				OccurredAt: now.Add(-31 * 24 * time.Hour),
			},
			repeat:      true,
			wantArchive: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RecordOperationalIssue(db, tc.issue); err != nil {
				t.Fatalf("RecordOperationalIssue() error = %v", err)
			}
			if tc.repeat {
				tc.issue.OccurredAt = tc.issue.OccurredAt.Add(time.Minute)
				if _, err := RecordOperationalIssue(db, tc.issue); err != nil {
					t.Fatalf("RecordOperationalIssue() repeat error = %v", err)
				}
			}
		})
	}

	archived, err := ArchiveStaleOperationalIssues(db, now)
	if err != nil {
		t.Fatalf("ArchiveStaleOperationalIssues() error = %v", err)
	}
	if archived != 2 {
		t.Fatalf("archived = %d, want 2", archived)
	}
	page, err := ListOperationalIssues(db, OperationalIssueListFilter{Status: "all", Limit: 20})
	if err != nil {
		t.Fatalf("ListOperationalIssues() error = %v", err)
	}
	statusByTitle := make(map[string]string, len(page.Items))
	for _, item := range page.Items {
		statusByTitle[item.Title] = item.Status
	}
	for _, tc := range cases {
		want := "open"
		if tc.wantArchive {
			want = "archived"
		}
		title := sanitizeOperationalIssueTitle(tc.issue.Title)
		if got := statusByTitle[title]; got != want {
			t.Errorf("%s status = %q, want %q", tc.name, got, want)
		}
	}
}

func TestOperationalIssueInvalidTimestampIsNotArchived(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	fingerprint, err := RecordOperationalIssue(db, OperationalIssue{
		Fingerprint: "maintenance|invalid-time", Source: "maintenance", Title: "Invalid time",
		Detail: "timestamp could not be parsed", Severity: "warning", OccurredAt: now.Add(-60 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}
	if _, err := db.Exec(`UPDATE operational_issues SET last_seen='not-a-time' WHERE fingerprint=?`, fingerprint); err != nil {
		t.Fatalf("set invalid last_seen: %v", err)
	}
	if archived, err := ArchiveStaleOperationalIssues(db, now); err != nil || archived != 0 {
		t.Fatalf("ArchiveStaleOperationalIssues() = %d, %v, want 0, nil", archived, err)
	}
}

func TestArchivedOperationalIssueReopensWithoutHistoryLoss(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	issue := OperationalIssue{
		Fingerprint: "follow_up|tool|save_note", Source: "follow_up", Context: "follow_up",
		Title: "Tool failed", Detail: "custom tool not found", Severity: "warning",
		OccurredAt: now.Add(-31 * 24 * time.Hour),
	}
	fingerprint, err := RecordOperationalIssue(db, issue)
	if err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}
	if archived, err := ArchiveStaleOperationalIssues(db, now); err != nil || archived != 1 {
		t.Fatalf("ArchiveStaleOperationalIssues() = %d, %v, want 1, nil", archived, err)
	}
	before, found, err := getOperationalIssueRecord(db, fingerprint)
	if err != nil || !found {
		t.Fatalf("archived record = found %v, err %v", found, err)
	}

	issue.OccurredAt = now.Add(time.Minute)
	if _, err := RecordOperationalIssue(db, issue); err != nil {
		t.Fatalf("RecordOperationalIssue() reopen error = %v", err)
	}
	after, found, err := getOperationalIssueRecord(db, fingerprint)
	if err != nil || !found {
		t.Fatalf("reopened record = found %v, err %v", found, err)
	}
	if after.Status != "open" || after.ArchivedAt != "" || after.ArchiveReason != "" {
		t.Fatalf("reopened status metadata = %#v", after)
	}
	if after.Occurrences != before.Occurrences+1 || after.Revision != before.Revision+1 {
		t.Fatalf("reopened history occurrences/revision = %d/%d, want %d/%d",
			after.Occurrences, after.Revision, before.Occurrences+1, before.Revision+1)
	}
}

func TestOperationalIssueToolFailureNeedsSecondOccurrenceForNotice(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	issue := OperationalIssue{
		Fingerprint: "heartbeat|heartbeat|tool|list_open_tasks", Source: "heartbeat", Context: "heartbeat",
		Title: "Tool failed", Detail: "custom tool not found", Severity: "warning", OccurredAt: now,
	}
	if _, err := RecordOperationalIssue(db, issue); err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}
	if notices, err := ListPendingOperationalIssueNotices(db, now.Add(time.Minute), 2); err != nil || len(notices) != 0 {
		t.Fatalf("single tool failure notices = %#v, err %v", notices, err)
	}
	issue.OccurredAt = now.Add(2 * time.Minute)
	if _, err := RecordOperationalIssue(db, issue); err != nil {
		t.Fatalf("RecordOperationalIssue() repeat error = %v", err)
	}
	if notices, err := ListPendingOperationalIssueNotices(db, now.Add(3*time.Minute), 2); err != nil || len(notices) != 1 {
		t.Fatalf("repeated tool failure notices = %#v, err %v", notices, err)
	}
}

func TestListOperationalIssuesFiltersAndPaginates(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	for _, issue := range []OperationalIssue{
		{Fingerprint: "heartbeat|tool|one", Source: "heartbeat", Title: "Tool one", Detail: "failed", Severity: "warning", OccurredAt: now},
		{Fingerprint: "maintenance|runtime|two", Source: "maintenance", Title: "Runtime two", Detail: "failed", Severity: "high", OccurredAt: now.Add(time.Minute)},
		{Fingerprint: "heartbeat|tool|three", Source: "heartbeat", Title: "Tool three", Detail: "failed", Severity: "warning", OccurredAt: now.Add(2 * time.Minute)},
	} {
		if _, err := RecordOperationalIssue(db, issue); err != nil {
			t.Fatalf("RecordOperationalIssue(%s) error = %v", issue.Fingerprint, err)
		}
	}
	page, err := ListOperationalIssues(db, OperationalIssueListFilter{
		Status: "active", Kind: OperationalIssueKindToolFailure, Source: "heartbeat", Limit: 1,
	})
	if err != nil {
		t.Fatalf("ListOperationalIssues() error = %v", err)
	}
	if len(page.Items) != 1 || page.Total != 2 || page.StatusCounts["active"] != 3 {
		t.Fatalf("filtered page = %#v", page)
	}
	second, err := ListOperationalIssues(db, OperationalIssueListFilter{
		Status: "active", Kind: OperationalIssueKindToolFailure, Source: "heartbeat", Limit: 1, Offset: 1,
	})
	if err != nil {
		t.Fatalf("ListOperationalIssues() second page error = %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].Title == page.Items[0].Title {
		t.Fatalf("second page = %#v, first = %#v", second.Items, page.Items)
	}
	errorsPage, err := ListOperationalIssues(db, OperationalIssueListFilter{Status: "active", Severity: "error"})
	if err != nil {
		t.Fatalf("ListOperationalIssues() severity page error = %v", err)
	}
	if len(errorsPage.Items) != 1 || errorsPage.Items[0].Severity != "high" {
		t.Fatalf("error severity category = %#v, want high-severity record", errorsPage.Items)
	}
}

func TestOperationalIssueArchiveAndRecordConcurrentlyKeepsRecurrenceActive(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	issue := OperationalIssue{
		Fingerprint: "heartbeat|race", Source: "heartbeat", Context: "heartbeat",
		Title: "Race warning", Detail: "temporary warning", Severity: "warning",
		OccurredAt: now.Add(-8 * 24 * time.Hour),
	}
	fingerprint, err := RecordOperationalIssue(db, issue)
	if err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}

	issue.OccurredAt = now.Add(time.Minute)
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		_, err := ArchiveStaleOperationalIssues(db, now)
		errs <- err
	}()
	go func() {
		defer wg.Done()
		_, err := RecordOperationalIssue(db, issue)
		errs <- err
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent operation error = %v", err)
		}
	}
	record, found, err := getOperationalIssueRecord(db, fingerprint)
	if err != nil || !found {
		t.Fatalf("record after concurrency = found %v, err %v", found, err)
	}
	if record.Status != "open" || record.Occurrences != 2 {
		t.Fatalf("record after concurrency = %#v, want open with two occurrences", record)
	}
}
