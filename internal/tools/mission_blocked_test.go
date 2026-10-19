package tools

import (
	"testing"

	"aurago/internal/memory"
)

func TestMissionActionRuleRequiredCompletesAsBlocked(t *testing.T) {
	m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mission := plannerEventMission()
	mission.Status = MissionStatusRunning
	m.missions[mission.ID] = mission
	m.queue.Enqueue(mission.ID, "medium", "manual", "")
	m.queue.Dequeue()
	var audit memory.AuditEvent
	m.auditRecorder = func(event memory.AuditEvent) error { audit = event; return nil }
	m.activeRunID[mission.ID] = "run"
	m.OnMissionComplete(mission.ID, MissionResultError, "action_rule_required")
	if mission.LastResult != MissionResultBlocked || mission.Status != MissionStatusIdle || m.queue.GetRunning() != "" {
		t.Fatalf("blocked completion=%+v", mission)
	}
	if audit.Status != memory.AuditStatusBlocked {
		t.Fatalf("audit status=%s", audit.Status)
	}
}
