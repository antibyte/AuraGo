package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

func publishedSimpleFlow(t *testing.T, s *Service) (*FlowRecord, string) {
	t.Helper()
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Gruß"})
	if err != nil {
		t.Fatal(err)
	}
	doc := simpleFlow("Gruß")
	rev, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatal(err)
	}
	// FF1: Run now needs the flow switched on.
	if err := s.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	return pub, doc.Nodes[0].ID
}

func TestServiceTestRuns(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Test"})
	doc := simpleFlow("Test")
	if _, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	res, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{})
	if err != nil {
		t.Fatalf("StartTestRun: %v", err)
	}
	if run := waitRun(t, s, res.RunID); run.Status != RunSuccess || run.Mode != ModeTest {
		t.Fatalf("test run = %+v", run)
	}
	detail, err := s.Run(ctx, res.RunID, true)
	if err != nil || detail.Doc == nil || len(detail.Steps) != 2 || detail.Steps[1].Output["greeting"] != "Hallo Welt" {
		t.Fatalf("detail = %+v, %v", detail, err)
	}
	if len(bridge.startedRuns()) != 0 {
		t.Fatal("test runs are never reported to Mission Control")
	}
	start := doc.Nodes[0].ID
	res, err = s.StartTestRun(ctx, rec.ID, TestRunRequest{TriggerData: map[string]any{"name": "Andi"}, RememberData: true})
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, s, res.RunID)
	if data, err := s.TriggerSampleData(ctx, rec.ID, start); err != nil || data["name"] != "Andi" {
		t.Fatalf("remembered sample = %v, %v", data, err)
	}
	if _, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{TriggerNode: doc.Nodes[1].ID}); !errors.Is(err, ErrNoTrigger) {
		t.Fatalf("a non-trigger node cannot start a test = %v", err)
	}
}

func TestServiceLiveRuns(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	draftOnly, _ := s.CreateFlow(ctx, CreateRequest{Name: "Entwurf"})
	if _, err := s.RunNow(ctx, draftOnly.ID); !errors.Is(err, ErrNotPublished) {
		t.Fatalf("RunNow on a draft = %v", err)
	}
	pub, start := publishedSimpleFlow(t, s)
	res, err := s.RunNow(ctx, pub.ID)
	if err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	info := bridge.waitFinished(t)
	greet, _ := info.Outputs["greet"].(map[string]any)
	if info.MissionID != pub.MissionID || info.HistoryID != "hist_"+res.RunID || info.Result.Status != RunSuccess ||
		greet["greeting"] != "Hallo Welt" || info.NotifyOnError != DefaultNotifyOnError || info.FlowName != "Gruß" {
		t.Fatalf("finished info = %+v", info)
	}
	if started := bridge.startedRuns(); len(started) != 1 || started[0].Mode != ModeLive || started[0].TriggerType != "manual" {
		t.Fatalf("started = %+v", started)
	}

	if _, err := s.TriggerFromMission(pub.MissionID, start, "api", map[string]any{"name": "API"}); err != nil {
		t.Fatalf("TriggerFromMission: %v", err)
	}
	info = bridge.waitFinished(t)
	if greet, _ := info.Outputs["greet"].(map[string]any); greet["greeting"] != "Hallo API" || info.Record.TriggerType != "api" {
		t.Fatalf("mission trigger info = %+v", info)
	}
	if _, err := s.TriggerFromMission(pub.MissionID, "n_zzzzzzzz", "api", nil); !errors.Is(err, ErrNoTrigger) {
		t.Fatalf("unknown trigger node = %v", err)
	}
	if _, err := s.TriggerFromMission("mission_unknown", "", "api", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown mission = %v", err)
	}
	runs, err := s.Runs(ctx, pub.ID, RunFilter{Mode: ModeLive})
	if err != nil || len(runs) != 2 {
		t.Fatalf("Runs = %d, %v", len(runs), err)
	}
}

func TestServiceTimerStartsRun(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	s, bridge := newServiceFixture(t, clock)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Wecker"})
	b := newFlow("Wecker")
	when := b.node("when", TypeTriggerDateTime, map[string]any{"at": "2026-10-03 08:00"})
	greet := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "at", "value": "{{trigger.data.scheduled_for}}"}}})
	b.edge(when, PortOut, greet)
	rev, _, err := s.SaveDraft(ctx, rec.ID, b.build(), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Publish(ctx, rec.ID, rev); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	clock.WaitForWaiters(t, 1)
	clock.Advance(time.Hour)
	info := bridge.waitFinished(t)
	out, _ := info.Outputs["greet"].(map[string]any)
	if info.Record.TriggerType != "datetime" || info.Record.TriggerNode != when || out["at"] != "2026-10-03T08:00:00Z" {
		t.Fatalf("timer run = %+v", info)
	}
}

func TestServiceStartMarksInterruptedRuns(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, _ := s.CreateFlow(ctx, CreateRequest{Name: "Alt"})
	if err := s.Store().CreateRun(ctx, RunRecord{ID: "run_aaaaaaaaafaa", FlowID: rec.ID, Mode: ModeLive, Status: RunRunning, StartedAt: storeNow}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.Run(ctx, "run_aaaaaaaaafaa", false)
	if detail.Run.Status != RunCancelled || detail.Run.ErrorCode != "FLOW_RESTARTED" {
		t.Fatalf("interrupted run = %+v", detail.Run)
	}
}

func TestLeafOutputs(t *testing.T) {
	f := simpleFlow("x")
	res := RunResult{Outputs: map[string]map[string]any{"start": {"a": 1.0}, "greet": {"greeting": "hi"}}}
	out := leafOutputs(f, res)
	if len(out) != 1 || out["greet"].(map[string]any)["greeting"] != "hi" {
		t.Fatalf("leafOutputs = %#v", out)
	}
}
