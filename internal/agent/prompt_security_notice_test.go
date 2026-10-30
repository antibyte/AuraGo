package agent

import (
	"testing"
	"time"

	"aurago/internal/planner"
)

func TestPromptSecurityIncidentNotifiesAtNextContactAfterDailySlot(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()
	now := time.Now()
	run := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	if state := prepareOperationalIssueNoticeAt(run, "first contact", now, nil); len(state.Items) != 0 {
		t.Fatal(state)
	}
	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{Source: planner.OperationalIssueSourcePromptSecurity, Fingerprint: "prompt-check", Title: "Internal prompt check", Detail: "The reviewer allowed the task.", Severity: "warning", OccurredAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	background := run
	background.MessageSource = "heartbeat"
	if state := prepareOperationalIssueNoticeAt(background, "background", now.Add(time.Minute), nil); len(state.Items) != 0 {
		t.Fatal(state)
	}
	state := prepareOperationalIssueNoticeAt(run, "next contact", now.Add(2*time.Minute), nil)
	if len(state.Items) != 1 || state.PromptContext == "" || state.Advisory {
		t.Fatalf("incident absent from agent context: %+v", state)
	}
	// Preparing is not delivery. A fallback is acknowledged only after the final
	// answer was persisted, exactly as for ordinary operational notices.
	state.FallbackRequired = true
	if got := prependOperationalIssueNotice(state, "Hallo"); got == "Hallo" {
		t.Fatal("user fallback notice missing")
	}
	markPersistedOperationalIssueNotice(state, run, nil)
	if next := prepareOperationalIssueNoticeAt(run, "another contact", now.Add(3*time.Minute), nil); len(next.Items) != 0 {
		t.Fatalf("delivered incident repeated: %+v", next)
	}
}
