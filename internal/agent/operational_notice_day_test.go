package agent

import (
	"aurago/internal/planner"
	"fmt"
	"testing"
	"time"
)

func TestOperationalNoticeFirstContactDoesNotDrainBacklog(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()
	now := time.Date(2030, 9, 22, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
			Fingerprint: fmt.Sprintf("issue-%d", i), Source: "mission", Title: fmt.Sprintf("Failure %d", i), Severity: "error", OccurredAt: now.Add(time.Duration(i-10) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	run := RunConfig{PlannerDB: db, MessageSource: "web_chat", SessionID: "chat-a"}
	first := prepareOperationalIssueNoticeAt(run, "Hallo", now, nil)
	if len(first.Items) != 2 {
		t.Fatalf("first contact: %+v", first)
	}
	if err := planner.MarkOperationalIssuesNotified(db, first.Refs, now); err != nil {
		t.Fatal(err)
	}
	for _, channel := range []string{"web_chat", "telegram", "virtual_desktop_chat"} {
		run.MessageSource, run.SessionID = channel, "another-session"
		if state := prepareOperationalIssueNoticeAt(run, "next message", now.Add(time.Minute), nil); len(state.Items) != 0 || state.Text != "" || state.PromptContext != "" {
			t.Fatalf("backlog leaked through %s: %+v", channel, state)
		}
	}
	pending, err := planner.ListPendingOperationalIssueNotices(db, now, 2)
	if err != nil || len(pending) != 2 {
		t.Fatalf("undelivered issues lost: %v %v", pending, err)
	}
	if state := prepareOperationalIssueNoticeAt(run, "next day", now.AddDate(0, 0, 1), nil); len(state.Items) != 2 {
		t.Fatalf("pending issues missing next day: %+v", state)
	}
}

func TestOperationalNoticeEmptyFirstContactDefersLaterIssues(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()
	now := time.Date(2030, 9, 22, 8, 0, 0, 0, time.UTC)
	run := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	if got := prepareOperationalIssueNoticeAt(run, "hello", now, nil); len(got.Items) != 0 {
		t.Fatal(got)
	}
	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{Source: "heartbeat", Title: "Later failure", Severity: "error", OccurredAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if got := prepareOperationalIssueNoticeAt(run, "hello again", now.Add(2*time.Hour), nil); len(got.Items) != 0 {
		t.Fatalf("late issue interrupted same day: %+v", got)
	}
	if got := prepareOperationalIssueNoticeAt(run, "tomorrow", now.AddDate(0, 0, 1), nil); len(got.Items) != 1 {
		t.Fatalf("late issue lost: %+v", got)
	}
}

func TestOperationalNoticeBackgroundAndPreviewDoNotClaimContact(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()
	now := time.Now()
	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{Source: "heartbeat", Title: "Failure", Severity: "error"}); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"heartbeat", "mqtt", "mission", "maintenance"} {
		if got := prepareOperationalIssueNoticeAt(RunConfig{PlannerDB: db, MessageSource: source}, "background", now, nil); len(got.Items) != 0 {
			t.Fatal(got)
		}
	}
	run := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	if operationalIssueReminderText(run, "preview", false, nil) == "" {
		t.Fatal("read-only preview missing")
	}
	if got := prepareOperationalIssueNoticeAt(run, "first contact", now, nil); len(got.Items) != 1 {
		t.Fatalf("background or preview consumed contact: %+v", got)
	}
	// A reserved slot is not evidence of delivery, including failed fallback.
	pending, err := planner.ListPendingOperationalIssueNotices(db, now, 2)
	if err != nil || len(pending) != 1 {
		t.Fatalf("preparation marked delivery: %v %v", pending, err)
	}
}
