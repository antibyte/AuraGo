package tools

import (
	"testing"
	"time"
)

// sharedFlowHooks is fakeFlowHooks plus the optional shared next-run function.
type sharedFlowHooks struct {
	*fakeFlowHooks
	shared time.Time
	asked  []string
}

func (h *sharedFlowHooks) SharedNextFlowRun(missionID string) (time.Time, bool) {
	h.asked = append(h.asked, missionID)
	return h.shared, !h.shared.IsZero()
}

func newFlowCronManager(t *testing.T) (*MissionManagerV2, *CronManager) {
	t.Helper()
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	t.Cleanup(mm.Stop)
	return mm, cronMgr
}

// NextFlowScheduleRun answers by flow id with the earliest schedule job and its node, and
// only for a fire after now.
func TestNextFlowScheduleRunByFlowID(t *testing.T) {
	mm, cronMgr := newFlowCronManager(t)
	mm.SetFlowHooks(newFakeFlowHooks())
	specs := []FlowTriggerSpec{
		{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 0 1 1 *"},
		{NodeID: "n_bbbbbbbb", TriggerType: FlowTriggerSchedule, Schedule: "30 6 * * *"},
		{NodeID: "n_cccccccc", TriggerType: FlowTriggerDateTime},
	}
	id := publishTestFlow(t, mm, specs...)
	want, ok := cronMgr.NextRun(flowCronJobID(id, "n_bbbbbbbb"))
	if !ok {
		t.Fatal("the cron engine has no next run for the daily job")
	}
	at, node, ok := mm.NextFlowScheduleRun("flow_aaaaaaaaaa", time.Now())
	if !ok || !at.Equal(want) || node != "n_bbbbbbbb" {
		t.Fatalf("NextFlowScheduleRun = %v %q %v, want %v n_bbbbbbbb", at, node, ok, want)
	}
	if _, _, ok := mm.NextFlowScheduleRun("flow_aaaaaaaaaa", want.Add(time.Second)); ok {
		t.Fatal("a fire that is not after now was reported")
	}
	if _, _, ok := mm.NextFlowScheduleRun("flow_unknown", time.Now()); ok {
		t.Fatal("an unknown flow has a schedule")
	}
}

// With flow hooks that implement FlowNextRunner, Mission Control's next run IS the shared
// answer (no second computation); a disabled mission has none whatever the hooks say.
func TestNextRunOfAFlowMissionUsesTheSharedFunction(t *testing.T) {
	mm, cronMgr := newFlowCronManager(t)
	hooks := &sharedFlowHooks{fakeFlowHooks: newFakeFlowHooks()}
	mm.SetFlowHooks(hooks)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 0 1 1 *"})
	if _, ok := cronMgr.NextRun(flowCronJobID(id, "n_aaaaaaaa")); !ok {
		t.Fatal("setup: the cron job has no next run")
	}
	hooks.shared = time.Date(2031, 3, 4, 5, 6, 0, 0, time.UTC)
	if next, ok := mm.NextRun(id); !ok || !next.Equal(hooks.shared) {
		t.Fatalf("NextRun = %v %v, want the shared answer %v", next, ok, hooks.shared)
	}
	if len(hooks.asked) != 1 || hooks.asked[0] != id {
		t.Fatalf("the shared function was asked for %v, want [%s]", hooks.asked, id)
	}
	hooks.shared = time.Time{}
	if next, ok := mm.NextRun(id); ok {
		t.Fatalf("NextRun = %v, want none when the shared function has none", next)
	}
	hooks.shared = time.Date(2031, 3, 4, 5, 6, 0, 0, time.UTC)
	if err := mm.SetFlowMissionEnabled(id, expectedFlowID(t, mm, id), false); err != nil {
		t.Fatal(err)
	}
	if next, ok := mm.NextRun(id); ok {
		t.Fatalf("a disabled flow has a next run %v", next)
	}
}
