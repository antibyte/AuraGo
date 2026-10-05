package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// FlowRunStarted marks a flow mission as running and records the live run in the mission
// history. It returns the history run id ("" without a history database).
func (m *MissionManagerV2) FlowRunStarted(missionID, triggerType, triggerData string) string {
	m.mu.Lock()
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		m.mu.Unlock()
		return ""
	}
	if m.flowActive == nil {
		m.flowActive = make(map[string]int)
	}
	m.flowActive[missionID]++
	mission.Status = MissionStatusRunning
	mission.LastRun = time.Now()
	name := mission.Name
	historyDB, recorder := m.historyDB, m.auditRecorder
	if err := m.save(); err != nil {
		slog.Warn("[MissionV2] Failed to persist flow run start", "mission_id", missionID, "error", err)
	}
	m.mu.Unlock()
	if historyDB == nil {
		return ""
	}
	if triggerType == "" {
		triggerType = "manual"
	}
	runID, err := RecordMissionStart(historyDB, missionID, name, triggerType, triggerData)
	if err != nil {
		slog.Warn("[MissionV2] Failed to record flow run start", "mission_id", missionID, "error", err)
		return ""
	}
	recordMissionAuditStart(recorder, runID, missionID, name, triggerType, triggerData)
	return runID
}

// FlowRunFinished records the result of a live flow run, releases the running state and
// fires dependent missions and flows. It never touches the agent queue's running slot.
func (m *MissionManagerV2) FlowRunFinished(missionID, historyID, result, output string, outputs map[string]any) {
	m.mu.Lock()
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		m.mu.Unlock()
		return
	}
	if m.flowActive[missionID] > 1 {
		m.flowActive[missionID]--
	} else {
		delete(m.flowActive, missionID)
		mission.Status = MissionStatusIdle
	}
	mission.LastResult = result
	mission.LastOutput = truncateString(output, 500)
	mission.RunCount++
	name := mission.Name
	historyDB, recorder, completeCB := m.historyDB, m.auditRecorder, m.onMissionComplete
	m.enqueueCompletionDependentsLocked(missionID, result, output, outputs)
	if err := m.save(); err != nil {
		slog.Error("[MissionV2] Failed to persist flow run result", "mission_id", missionID, "error", err)
	}
	if err := m.saveQueueLocked(); err != nil {
		slog.Error("[MissionV2] Failed to persist queue after flow run", "mission_id", missionID, "error", err)
	}
	m.mu.Unlock()

	if historyID != "" {
		recordMissionAuditCompletion(recorder, historyID, missionID, name, result, output)
		if historyDB != nil {
			var err error
			if result == MissionResultSuccess {
				err = RecordMissionCompletion(historyDB, historyID, "success", output)
			} else {
				err = RecordMissionError(historyDB, historyID, output)
			}
			if err != nil {
				slog.Error("[MissionV2] Failed to record flow run history", "run_id", historyID, "error", err)
			}
		}
	}
	if completeCB != nil {
		go completeCB(missionID, result, output)
	}
}

// enqueueCompletionDependentsLocked queues prompt missions and starts flows that wait for
// sourceID. The trigger data carries the output (≤ 2000 chars) and, for flow sources, the
// outputs of the flow's final nodes. Caller holds m.mu.
func (m *MissionManagerV2) enqueueCompletionDependentsLocked(sourceID, result, output string, outputs map[string]any) {
	data := map[string]any{"source_mission": sourceID, "result": result, "output": truncateString(output, 2000)}
	if outputs != nil {
		data["outputs"] = outputs
	}
	raw, _ := json.Marshal(data)
	now := time.Now()
	for _, mission := range m.missions {
		if !mission.Enabled || mission.ExecutionType != ExecutionTriggered || mission.TriggerType != TriggerMissionCompleted {
			continue
		}
		cfg := mission.TriggerConfig
		if cfg == nil || cfg.SourceMissionID != sourceID {
			continue
		}
		if cfg.RequireSuccess && result != MissionResultSuccess {
			continue
		}
		if !m.shouldFireTriggerLocked(mission, string(TriggerMissionCompleted), now) {
			continue
		}
		m.queue.Enqueue(mission.ID, mission.Priority, "mission_completed", string(raw))
		mission.Status = MissionStatusQueued
	}
	m.notifyFlowsLocked(TriggerMissionCompleted, flowEvent{SourceMissionID: sourceID, Result: result}, data)
}

// flowMissionRoute reports whether missionID is a flow mission and returns the hooks that
// start its runs.
func (m *MissionManagerV2) flowMissionRoute(missionID string) (FlowHooks, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		return nil, false, nil
	}
	if !mission.Enabled {
		return nil, true, fmt.Errorf("mission is disabled")
	}
	if m.flowHooks == nil {
		return nil, true, fmt.Errorf("flows are not available")
	}
	return m.flowHooks, true, nil
}

// updateFlowMissionLocked applies the only changes Mission Control may make to a flow
// mission — the enabled switch and the lock. Everything else is owned by EasyDrag.
func (m *MissionManagerV2) updateFlowMissionLocked(mission, updated *MissionV2) error {
	if updated.ExecutionType != "" && updated.ExecutionType != ExecutionFlow {
		return ErrFlowMissionManaged
	}
	if updated.Enabled && !mission.FlowPublished {
		return fmt.Errorf("activating an unpublished flow is not supported; publish it in EasyDrag first")
	}
	enabledChanged := mission.Enabled != updated.Enabled
	mission.Enabled = updated.Enabled
	mission.Locked = updated.Locked
	regErr := m.syncFlowTriggersLocked(mission)
	saveErr := m.save()
	if enabledChanged && m.flowHooks != nil {
		hooks, id, enabled := m.flowHooks, mission.ID, mission.Enabled
		go hooks.FlowEnabledChanged(id, enabled)
	}
	return errors.Join(regErr, saveErr)
}

// nextFlowRun returns the earliest schedule or Date/Time trigger of an enabled flow
// mission. isFlow is false for every other mission.
func (m *MissionManagerV2) nextFlowRun(id string) (next time.Time, ok bool, isFlow bool) {
	m.mu.RLock()
	mission, exists := m.missions[id]
	if !exists || !isFlowMission(mission) {
		m.mu.RUnlock()
		return time.Time{}, false, false
	}
	enabled := mission.Enabled
	var jobs []string
	for _, spec := range mission.FlowTriggers {
		if spec.TriggerType == FlowTriggerSchedule {
			jobs = append(jobs, flowCronJobID(id, spec.NodeID))
		}
	}
	hooks, cron := m.flowHooks, m.cron
	m.mu.RUnlock()
	if !enabled {
		return time.Time{}, false, true
	}
	consider := func(t time.Time, found bool) {
		if found && !t.IsZero() && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}
	if cron != nil {
		for _, job := range jobs {
			consider(cron.NextRun(job))
		}
	}
	if hooks != nil {
		consider(hooks.NextFlowRun(id))
	}
	return next, !next.IsZero(), true
}
