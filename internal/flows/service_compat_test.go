package flows

import (
	"context"
	"sync"
	"testing"
	"time"
)

// b3NoteBridge is fakeBridge with the optional LiveIssuesNoter; it records each noted mission.
type b3NoteBridge struct {
	*fakeBridge
	noteMu sync.Mutex
	noted  []string
	clears []string
}

func (b *b3NoteBridge) ClearFlowLiveIssues(missionID string) {
	b.noteMu.Lock()
	b.clears = append(b.clears, missionID)
	b.noteMu.Unlock()
}

func (b *b3NoteBridge) cleared() []string {
	b.noteMu.Lock()
	defer b.noteMu.Unlock()
	return append([]string(nil), b.clears...)
}

func (b *b3NoteBridge) NoteFlowLiveIssues(missionID string, _ []Issue) {
	b.noteMu.Lock()
	b.noted = append(b.noted, missionID)
	b.noteMu.Unlock()
}

func (b *b3NoteBridge) notes() []string {
	b.noteMu.Lock()
	defer b.noteMu.Unlock()
	return append([]string(nil), b.noted...)
}

// R2b: after an update the live revisions are checked once against the rules of this build. A
// flow whose live revision now has errors is noted on its mission and listed with LiveIssues
// until it is published again. Time-bound rules judge a revision at its publish time, so a
// one-off date that has passed since is no finding.
func TestB3CheckLiveRevisionsFlagsWhatTheCurrentRulesRefuse(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := &b3NoteBridge{fakeBridge: newFakeBridge()}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC},
		bridge, ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	// Published under older rules: the draft passes the draft rules; the store makes it live
	// without today's publish check.
	old, _ := secretCallFlow(map[string]any{"url": "{{trigger.data.link}}", "auth_secret": "partner_token"})
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: old})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	if _, err := s.store.Publish(ctx, rec.ID, rec.DraftRevision, clock.Now()); err != nil {
		t.Fatalf("store publish: %v", err)
	}
	// A one-off date two hours after its publication, checked three hours later.
	b := newFlow("Once")
	at := b.node("at", TypeTriggerDateTime, map[string]any{"at": "2026-10-03 09:00"})
	greet := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "g", "value": "hi"}}})
	b.edge(at, PortOut, greet)
	once := svcRunPublish(t, s, b.build())
	clock.Advance(3 * time.Hour)

	reports, err := s.CheckLiveRevisions(ctx)
	if err != nil || len(reports) != 1 || reports[0].FlowID != rec.ID || reports[0].Revision != 1 || reports[0].MissionID != rec.MissionID {
		t.Fatalf("reports = %+v, %v; want only the flow published under older rules", reports, err)
	}
	if findIssue(reports[0].Issues, IssueUntrustedSecretTarget, "") == nil {
		t.Errorf("the report misses the secret-target error: %+v", reports[0].Issues)
	}
	for _, is := range reports[0].Issues {
		if is.Severity != SeverityError {
			t.Errorf("a warning in the report: %+v", is)
		}
	}
	if got := bridge.notes(); len(got) != 1 || got[0] != rec.MissionID {
		t.Errorf("noted missions = %v, want [%s]", got, rec.MissionID)
	}
	list, err := s.ListFlows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]FlowSummary{}
	for _, f := range list {
		byID[f.ID] = f
	}
	if len(byID[rec.ID].LiveIssues) == 0 || len(byID[once.ID].LiveIssues) != 0 {
		t.Errorf("live issues = %v and %v, want some for the old flow only", byID[rec.ID].LiveIssues, byID[once.ID].LiveIssues)
	}

	// Published again, fixed: the new live revision passed today's rules, the finding is gone.
	fixed, _ := secretCallFlow(map[string]any{"url": "https://api.example.org/v1?next={{trigger.data.link}}", "auth_secret": "partner_token"})
	cur, _ := s.GetFlow(ctx, rec.ID)
	rev, _, err := s.SaveDraft(ctx, rec.ID, fixed, cur.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if _, _, err := s.Publish(ctx, rec.ID, rev); err != nil {
		t.Fatalf("Publish of the fixed draft: %v", err)
	}
	cur, _ = s.GetFlow(ctx, rec.ID)
	if got := s.LiveIssues(cur); got != nil {
		t.Errorf("LiveIssues after a new publication = %+v, want none", got)
	}
}

// R2b: a new publication and a clean re-check both take the note off the mission, and a fixed
// flow is not noted again.
func TestB3LiveNoteClearsOnRepublishAndCleanCheck(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := &b3NoteBridge{fakeBridge: newFakeBridge()}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC},
		bridge, ServiceConfig{}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })

	old, _ := secretCallFlow(map[string]any{"url": "{{trigger.data.link}}", "auth_secret": "partner_token"})
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: old})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.Publish(ctx, rec.ID, rec.DraftRevision, clock.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckLiveRevisions(ctx); err != nil {
		t.Fatal(err)
	}
	if len(bridge.notes()) != 1 || len(bridge.cleared()) != 0 {
		t.Fatalf("notes %v clears %v after the first check", bridge.notes(), bridge.cleared())
	}

	fixed, _ := secretCallFlow(map[string]any{"url": "https://api.example.org/v1?next={{trigger.data.link}}", "auth_secret": "partner_token"})
	cur, _ := s.GetFlow(ctx, rec.ID)
	rev, _, err := s.SaveDraft(ctx, rec.ID, fixed, cur.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Publish(ctx, rec.ID, rev); err != nil {
		t.Fatal(err)
	}
	if got := bridge.cleared(); len(got) != 1 || got[0] != rec.MissionID {
		t.Fatalf("clears after the republish = %v, want [%s]", got, rec.MissionID)
	}
	if _, err := s.CheckLiveRevisions(ctx); err != nil {
		t.Fatal(err)
	}
	if got := bridge.cleared(); len(got) != 2 || got[1] != rec.MissionID {
		t.Fatalf("clears after the clean re-check = %v", got)
	}
	if len(bridge.notes()) != 1 {
		t.Fatalf("a fixed flow was noted again: %v", bridge.notes())
	}
}
