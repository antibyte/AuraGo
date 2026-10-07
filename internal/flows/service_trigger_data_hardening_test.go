package flows

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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

// FF1 review: a dropped data payload leaves a Debug line with the mission and the size,
// never the data.
func TestFF1GenericMissionStartLogsTheDrop(t *testing.T) {
	logs := &svcLogs{}
	s := svcRunNewService(t, &fakeTools{}, newSvcRunBridge(), slog.New(logs), ServiceConfig{})
	pub := svcRunPublish(t, s, simpleFlow("Leise"))
	res, err := s.TriggerFromMission(pub.MissionID, "", "api", map[string]any{"name": "ff1-geheim"})
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, s, res.RunID)
	logs.mu.Lock()
	defer logs.mu.Unlock()
	found := 0
	for _, r := range logs.records {
		if !strings.Contains(r.Message, "dropped") {
			continue
		}
		found++
		attrs := map[string]string{}
		r.Attrs(func(a slog.Attr) bool {
			attrs[a.Key] = a.Value.String()
			return true
		})
		if r.Level != slog.LevelDebug || attrs["mission_id"] != pub.MissionID || attrs["data_bytes"] == "" || attrs["data_bytes"] == "0" {
			t.Fatalf("drop log = %s %v", r.Message, attrs)
		}
		for _, v := range attrs {
			if strings.Contains(v, "ff1-geheim") {
				t.Fatalf("the drop log carries the data: %v", attrs)
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d drop log lines, want 1", found)
	}
}

// ff1ReconBridge is a bridge that can also list Mission Control's flow missions; none
// stands for "no Mission Control" (FlowMissions returns nil).
type ff1ReconBridge struct {
	*svcBridge
	none bool
	gone string
}

func (b *ff1ReconBridge) FlowMissions() map[string]string {
	if b.none {
		return nil
	}
	b.fakeBridge.mu.Lock()
	defer b.fakeBridge.mu.Unlock()
	out := map[string]string{}
	for id, m := range b.missions {
		if id != b.gone {
			out[id] = m.flowID
		}
	}
	return out
}

func (b *ff1ReconBridge) FlowMissionInSync(string, string, []TriggerBinding) bool { return true }

// FF1 review: RunNow says "paused" only for a mission that is switched off; without
// Mission Control it is ErrMissionControlUnavailable, and with the mission gone
// ErrFlowMissionMissing.
func TestFF1RunNowTellsWhyTheFlowIsOff(t *testing.T) {
	bridge := &ff1ReconBridge{svcBridge: newSvcBridge()}
	s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
	ctx := context.Background()
	pub := svcRunPublish(t, s, simpleFlow("Grund"))
	if err := s.SetEnabled(ctx, pub.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RunNow(ctx, pub.ID); !errors.Is(err, ErrFlowDisabled) {
		t.Fatalf("paused = %v", err)
	}
	bridge.gone = pub.MissionID
	if _, err := s.RunNow(ctx, pub.ID); !errors.Is(err, ErrFlowMissionMissing) {
		t.Fatalf("mission gone = %v", err)
	}
	bridge.none = true
	if _, err := s.RunNow(ctx, pub.ID); !errors.Is(err, ErrMissionControlUnavailable) {
		t.Fatalf("no Mission Control = %v", err)
	}
}
