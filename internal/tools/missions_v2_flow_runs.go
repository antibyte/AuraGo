package tools

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/memory"
)

// flowHistoryTriggerDataMaxBytes caps the trigger data of a flow run that the mission history
// and the audit start keep. Prompt missions store theirs uncapped, but a flow's trigger data
// can reach 256 KiB (a webhook or mail payload preview of the flow service).
const flowHistoryTriggerDataMaxBytes = 16 << 10

// flowHistoryTruncatedMarker ends trigger data that flowHistoryTriggerData cut.
const flowHistoryTruncatedMarker = "...[truncated]"

// flowCompletionOutputsMaxBytes bounds the encoded outputs of a flow that mission_completed
// hands to every dependent. Outputs can reach 32 MiB (flows.MaxRunOutputBytes), and the queue
// that carries them to prompt missions is persisted. Larger outputs become
// {"_truncated": true, "_preview": "<first flowCompletionOutputsPreviewBytes of the JSON>"},
// close to the {"_preview": …} shape of the flow run log.
const (
	flowCompletionOutputsMaxBytes     = 64 << 10
	flowCompletionOutputsPreviewBytes = 4 << 10
)

// flowHistoryTriggerData returns data cut to flowHistoryTriggerDataMaxBytes at a rune
// boundary, ending with flowHistoryTruncatedMarker when it was cut.
func flowHistoryTriggerData(data string) string {
	if len(data) <= flowHistoryTriggerDataMaxBytes {
		return data
	}
	return cutAtRuneBoundary(data, flowHistoryTriggerDataMaxBytes-len(flowHistoryTruncatedMarker)) + flowHistoryTruncatedMarker
}

// boundedCompletionOutputs encodes a flow's outputs once for its mission_completed
// dependents, as a preview beyond flowCompletionOutputsMaxBytes.
func boundedCompletionOutputs(outputs map[string]any) json.RawMessage {
	enc, err := json.Marshal(outputs)
	if err == nil && len(enc) <= flowCompletionOutputsMaxBytes {
		return enc
	}
	preview := "<unserializable>"
	if err == nil {
		preview = cutAtRuneBoundary(string(enc), flowCompletionOutputsPreviewBytes)
	}
	enc, _ = json.Marshal(map[string]any{"_truncated": true, "_preview": preview})
	return enc
}

// FlowRunStarted marks a flow mission as running and records the live run in the mission
// history. It returns the history run id ("" without a history database). The history and
// the audit start keep at most flowHistoryTriggerDataMaxBytes of the trigger data.
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
	triggerData = flowHistoryTriggerData(triggerData)
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
//
// Call it ONLY for runs whose FlowRunStarted was called: it releases one running slot. A
// run that never started (queued and cancelled, or stopped by a shutdown) is not reported
// here. Without a running slot to release the call changes no counter or status beyond
// clearing them, counts no run and fires no dependents.
//
// The history entry historyID is completed even when the mission is gone, because deleting a
// flow deletes its mission before it cancels the flow's runs.
func (m *MissionManagerV2) FlowRunFinished(missionID, historyID, result, output string, outputs map[string]any) {
	m.mu.Lock()
	historyDB, recorder := m.historyDB, m.auditRecorder
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		m.mu.Unlock()
		slog.Debug("[MissionV2] Flow run finished for a mission that is gone", "mission_id", missionID, "run_id", historyID)
		completeFlowRunHistory(historyDB, recorder, historyID, missionID, "", result, output)
		return
	}
	if m.flowActive[missionID] <= 0 {
		delete(m.flowActive, missionID)
		changed := mission.Status != MissionStatusIdle
		mission.Status = MissionStatusIdle
		name := mission.Name
		if changed {
			if err := m.save(); err != nil {
				slog.Error("[MissionV2] Failed to persist flow mission state", "mission_id", missionID, "error", err)
			}
		}
		m.mu.Unlock()
		slog.Debug("[MissionV2] Flow run finished without a running slot; not counted", "mission_id", missionID, "run_id", historyID)
		completeFlowRunHistory(historyDB, recorder, historyID, missionID, name, result, output)
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
	completeCB := m.onMissionComplete
	queued := m.enqueueCompletionDependentsLocked(missionID, result, output, outputs)
	if err := m.save(); err != nil {
		slog.Error("[MissionV2] Failed to persist flow run result", "mission_id", missionID, "error", err)
	}
	if queued > 0 {
		if err := m.saveQueueLocked(); err != nil {
			slog.Error("[MissionV2] Failed to persist queue after flow run", "mission_id", missionID, "error", err)
		}
	}
	m.mu.Unlock()

	completeFlowRunHistory(historyDB, recorder, historyID, missionID, name, result, output)
	if completeCB != nil {
		go completeCB(missionID, result, output)
	}
}

// completeFlowRunHistory writes the audit completion and completes the history entry of a
// flow run. Without a name it uses the mission name stored with the entry, else the mission
// id. Nothing happens without a history id. Call it without m.mu held.
func completeFlowRunHistory(historyDB *sql.DB, recorder func(memory.AuditEvent) error, historyID, missionID, name, result, output string) {
	if historyID == "" {
		return
	}
	if name == "" && historyDB != nil {
		if run, err := GetMissionRun(historyDB, historyID); err == nil {
			name = run.MissionName
		}
	}
	recordMissionAuditCompletion(recorder, historyID, missionID, name, result, output)
	if historyDB == nil {
		return
	}
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

// enqueueCompletionDependentsLocked queues prompt missions and starts flows that wait for
// sourceID. The trigger data carries the output (≤ 2000 chars) and, for flow sources, the
// outputs of the flow's final nodes (bounded, see boundedCompletionOutputs). It returns the
// number of prompt missions it queued. Caller holds m.mu.
func (m *MissionManagerV2) enqueueCompletionDependentsLocked(sourceID, result, output string, outputs map[string]any) int {
	data := map[string]any{"source_mission": sourceID, "result": result, "output": truncateString(output, 2000)}
	if outputs != nil {
		data["outputs"] = boundedCompletionOutputs(outputs)
	}
	raw, _ := json.Marshal(data)
	queued := 0
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
		queued++
	}
	m.notifyFlowsLocked(TriggerMissionCompleted, flowEvent{SourceMissionID: sourceID, Result: result}, json.RawMessage(raw))
	return queued
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
// mission. isFlow is false for every other mission. Only flowCronSource jobs count: a
// prompt mission whose id is "<flow>__<node>" owns a job with the same id.
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
	if cron != nil && len(jobs) > 0 {
		flowJobs := make(map[string]bool, len(jobs))
		for _, job := range cron.GetJobs() {
			if job.Source == flowCronSource {
				flowJobs[job.ID] = true
			}
		}
		for _, job := range jobs {
			if flowJobs[job] {
				consider(cron.NextRun(job))
			}
		}
	}
	if hooks != nil {
		consider(hooks.NextFlowRun(id))
	}
	return next, !next.IsZero(), true
}
