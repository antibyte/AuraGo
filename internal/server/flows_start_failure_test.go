package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/planner"
	"aurago/internal/tools"
)

// A flow mission whose flow is not published fires its trigger twice. The first start failure
// is recorded as one failed run (FLOW_START_FAILED) with one desktop notification and one
// planner issue; the second belongs to the same streak and records nothing.
func TestFlowStartFailureIsRecordedOncePerStreak(t *testing.T) {
	allowMissionMutationsForTest(t)
	c15LoadI18n()
	s := c16StartedServer(t, nil)
	rec := createTestFlow(t, s, waitFlowJSON) // a draft; it is never published
	plannerDB, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner db: %v", err)
	}
	t.Cleanup(func() { _ = plannerDB.Close() })
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	s.PlannerDB, s.DesktopHub = plannerDB, hub

	drain := func() (notifications, changed int) {
		for {
			select {
			case ev := <-events:
				switch ev.Type {
				case "notification":
					notifications++
				case "flows_changed":
					changed++
				}
			default:
				return
			}
		}
	}
	issues := func() []planner.OperationalIssueListItem {
		page, err := planner.ListOperationalIssues(s.PlannerDB, planner.OperationalIssueListFilter{Status: "all", Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	failedRuns := func() []flows.RunRecord {
		runs, err := s.Flows.Runs(context.Background(), rec.ID, flows.RunFilter{Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		var out []flows.RunRecord
		for _, r := range runs {
			if r.ErrorCode == flows.StartFailedCode {
				out = append(out, r)
			}
		}
		return out
	}
	hooks := flowMissionHooks{s: s}
	fire := func() {
		t.Helper()
		_, err := hooks.StartFlowRunReported(rec.MissionID, "", "webhook", "")
		if !errors.Is(err, flows.ErrNotPublished) || !errors.Is(err, tools.ErrFlowStartFailed) {
			t.Fatalf("a trigger of an unpublished flow: err = %v, want ErrNotPublished marked as tools.ErrFlowStartFailed", err)
		}
	}

	fire()
	// The record and the report run on the Service's goroutine: wait for the planner issue,
	// the last thing FlowStartFailed does before the notification.
	deadline := time.Now().Add(5 * time.Second)
	for len(issues()) == 0 || len(failedRuns()) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the start failure was not recorded: %d runs, %d issues", len(failedRuns()), len(issues()))
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Shutdown waits for every record goroutine, so what is counted next is complete.
	fire()
	if err := s.Flows.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	runs := failedRuns()
	if len(runs) != 1 || runs[0].Status != flows.RunError || runs[0].Mode != flows.ModeLive || runs[0].Revision != 0 {
		t.Fatalf("failed runs after two fires = %+v, want exactly one", runs)
	}
	if got := issues(); len(got) != 1 || got[0].Occurrences != 1 {
		t.Fatalf("planner issues after two fires = %+v, want one issue with one occurrence", got)
	}
	if notifications, changed := drain(); notifications != 1 || changed != 1 {
		t.Fatalf("desktop events: %d notifications and %d flows_changed, want 1 and 1", notifications, changed)
	}
}
