package flows

import (
	"context"
	"testing"
)

// ff1Greeting waits for the run and returns the greeting of simpleFlow's set node.
func ff1Greeting(t *testing.T, s *Service, runID string) (any, map[string]any) {
	t.Helper()
	waitRun(t, s, runID)
	detail, err := s.Run(context.Background(), runID, false)
	if err != nil || len(detail.Steps) != 2 {
		t.Fatalf("run detail = %+v, %v", detail, err)
	}
	return detail.Steps[1].Output["greeting"], detail.Run.TriggerData
}

// FF1: Mission Control's generic start (no node: its Run, and TriggerMissionWithOptions,
// which the daemon wake-up and POST /api/missions/v2/{id}/trigger use) never passes the
// caller's data into the flow. The lint treats the manual and the other built-in triggers
// as trusted, so caller data there would reach {{trigger.…}} in a shell step unwarned.
// The run starts like Run now: the manual trigger with its sample. A start that names the
// node (a trigger registration of Mission Control) keeps its data.
func TestFF1GenericMissionStartDropsCallerData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	pub, start := publishedSimpleFlow(t, s)

	res, err := s.TriggerFromMission(pub.MissionID, "", "api", map[string]any{"name": "Angreifer"})
	if err != nil {
		t.Fatalf("TriggerFromMission: %v", err)
	}
	greeting, data := ff1Greeting(t, s, res.RunID)
	if greeting != "Hallo Welt" || data["name"] != "Welt" {
		t.Fatalf("generic start: greeting %v, trigger data %v; want the sample", greeting, data)
	}
	run, err := s.Run(context.Background(), res.RunID, false)
	if err != nil || run.Run.TriggerType != "api" {
		t.Fatalf("the trigger type is still recorded: %+v, %v", run, err)
	}

	res, err = s.TriggerFromMission(pub.MissionID, start, "webhook", map[string]any{"name": "Bote"})
	if err != nil {
		t.Fatalf("TriggerFromMission with a node: %v", err)
	}
	if greeting, _ := ff1Greeting(t, s, res.RunID); greeting != "Hallo Bote" {
		t.Fatalf("a start that names its node: greeting %v", greeting)
	}
}

// FF1: without a manual trigger the generic start picks the first trigger and starts it
// with empty data, never the caller's.
func TestFF1GenericMissionStartWithoutManualTriggerHasNoData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Plan")
	tick := b.node("tick", TypeTriggerSchedule, map[string]any{"mode": "daily", "time": "07:00"})
	greet := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "greeting", "value": "Hallo {{trigger.data.name}}"}}})
	b.edge(tick, PortOut, greet)
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Plan"})
	if err != nil {
		t.Fatal(err)
	}
	rev, _, err := s.SaveDraft(ctx, rec.ID, b.build(), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Publish(ctx, rec.ID, rev); err != nil {
		t.Fatal(err)
	}
	res, err := s.TriggerFromMission(rec.MissionID, "", "daemon", map[string]any{"name": "Angreifer"})
	if err != nil {
		t.Fatalf("TriggerFromMission: %v", err)
	}
	greeting, data := ff1Greeting(t, s, res.RunID)
	if greeting == "Hallo Angreifer" || len(data) != 0 {
		t.Fatalf("generic start of a schedule flow: greeting %v, trigger data %v", greeting, data)
	}
}
