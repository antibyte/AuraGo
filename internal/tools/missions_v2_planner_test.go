package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func plannerEventMission() *MissionV2 {
	return &MissionV2{ID: "planner", Name: "Planner", Prompt: "remind", ExecutionType: ExecutionTriggered, TriggerType: TriggerPlannerAppointmentDue, Enabled: true, Priority: "medium", Status: MissionStatusIdle}
}

func TestPlannerMissionAdmissionDedupeSurvivesQueueCompletionAndRestart(t *testing.T) {
	dir := tempSystemTaskDir(t)
	m := NewMissionManagerV2(dir, nil)
	m.missions["planner"] = plannerEventMission()
	if err := m.NotifyPlannerAppointmentDueEvent("event-1", "apt", "Reminder", "2026-10-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := m.NotifyPlannerAppointmentDueEvent("event-1", "apt", "Reminder", "2026-10-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if len(m.queue.List()) != 1 {
		t.Fatal("same event admitted twice")
	}
	if _, ok := m.queue.TryStartNext(); !ok {
		t.Fatal("no admitted mission")
	}
	m.queue.Done()
	m.mu.Lock()
	if err := m.saveQueueLocked(); err != nil {
		t.Fatal(err)
	}
	m.mu.Unlock()
	restarted := NewMissionManagerV2(dir, nil)
	restarted.missions["planner"] = plannerEventMission()
	if _, err := restarted.loadQueueLocked(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.NotifyPlannerAppointmentDueEvent("event-1", "apt", "Reminder", "2026-10-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if len(restarted.queue.List()) != 0 {
		t.Fatal("completed event replayed after restart")
	}
	if err := restarted.NotifyPlannerAppointmentDueEvent("event-2", "apt", "Reminder", "2026-10-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.NotifyPlannerAppointmentDueEvent("event-3", "apt", "Reminder", "2026-10-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if len(restarted.queue.List()) != 2 {
		t.Fatal("distinct due revisions were coalesced")
	}
}

func TestPlannerMissionAdmissionSaveFailureRollsBackAndDoesNotAcknowledge(t *testing.T) {
	m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	m.missions["planner"] = plannerEventMission()
	m.queueFile = filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(m.queueFile, 0700); err != nil {
		t.Fatal(err)
	}
	if err := m.NotifyPlannerAppointmentDueEvent("event", "apt", "Reminder", "2026-10-19T00:00:00Z"); err == nil {
		t.Fatal("queue save error suppressed")
	}
	if len(m.queue.List()) != 0 || len(m.plannerEvents) != 0 || m.missions["planner"].Status != MissionStatusIdle {
		t.Fatal("unpersisted admission remained runnable")
	}
}

func TestPlannerRunningMissionRecoveryDoesNotReplayUncertainExecution(t *testing.T) {
	dir := tempSystemTaskDir(t)
	m := NewMissionManagerV2(dir, nil)
	m.missions["planner"] = plannerEventMission()
	if err := m.NotifyPlannerAppointmentDueEvent("event", "apt", "Reminder", "2026-10-19T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.queue.TryStartNext(); !ok {
		t.Fatal("no admitted task")
	}
	m.mu.Lock()
	if err := m.saveQueueLocked(); err != nil {
		t.Fatal(err)
	}
	m.mu.Unlock()
	restarted := NewMissionManagerV2(dir, nil)
	restarted.missions["planner"] = plannerEventMission()
	if _, err := restarted.loadQueueLocked(); err != nil {
		t.Fatal(err)
	}
	if len(restarted.queue.List()) != 0 || restarted.missions["planner"].LastResult != MissionResultError {
		t.Fatal("uncertain planner mission replayed")
	}
}
