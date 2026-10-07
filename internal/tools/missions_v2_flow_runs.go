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

// flowCompletionOutputsMaxBytes bounds the encoded `outputs` field (the outputs of a flow's
// final nodes) that mission_completed hands to every dependent; the `output` text has its own
// cap, completionOutputMaxBytes. Outputs can reach 32 MiB (flows.MaxRunOutputBytes), and the
// queue that carries them to prompt missions is persisted. Larger outputs become
// {"_truncated": true, "_preview": "<first flowCompletionOutputsPreviewBytes of the JSON>"},
// close to the {"_preview": …} shape of the flow run log.
const (
	flowCompletionOutputsMaxBytes     = 64 << 10
	flowCompletionOutputsPreviewBytes = 4 << 10
)

// agentCompletionOutputsMaxBytes bounds the encoded `outputs` that mission_completed hands
// to AGENT missions: their trigger data goes into the agent's prompt
// (appendIsolatedTriggerContext), the history row and the persisted queue. Larger outputs
// become {"_truncated": true, "_preview": "<as much of the JSON as fits>"} within the cap
// (capCompletionOutputsForAgents). Flow dependents keep the outputs up to
// flowCompletionOutputsMaxBytes.
const agentCompletionOutputsMaxBytes = 8 << 10

// capCompletionOutputsForAgents returns outputs when they fit agentCompletionOutputsMaxBytes,
// else the truncation marker with the longest preview whose encoding fits (the preview is
// JSON text, so its quotes are escaped again). Outputs that boundedCompletionOutputs cut
// already (the marker object alone) get their preview cut, never a preview of the marker.
// Caller may hold m.mu: outputs are at most flowCompletionOutputsMaxBytes.
func capCompletionOutputsForAgents(outputs json.RawMessage) json.RawMessage {
	if len(outputs) <= agentCompletionOutputsMaxBytes {
		return outputs
	}
	preview := string(outputs)
	var marked map[string]json.RawMessage
	if json.Unmarshal(outputs, &marked) == nil && len(marked) == 2 && string(marked["_truncated"]) == "true" {
		var inner string
		if json.Unmarshal(marked["_preview"], &inner) == nil {
			preview = inner
		}
	}
	limit := agentCompletionOutputsMaxBytes
	for {
		enc, _ := json.Marshal(map[string]any{"_truncated": true, "_preview": cutAtRuneBoundary(preview, limit)})
		if len(enc) <= agentCompletionOutputsMaxBytes || limit == 0 {
			return enc
		}
		next := limit * agentCompletionOutputsMaxBytes / len(enc)
		if next >= limit {
			next = limit - 1
		}
		limit = max(next-32, 0)
	}
}

// completionOutputMaxBytes caps the `output` text of mission_completed trigger data, and
// flowLastOutputMaxBytes the LastOutput of a flow mission. Both cut at a rune boundary and
// end with completionTruncatedMarker.
const (
	completionOutputMaxBytes  = 2000
	flowLastOutputMaxBytes    = 500
	completionTruncatedMarker = "..."
)

// cutWithMarker returns s when it has at most limit bytes, else its longest prefix that
// leaves room for marker without splitting a UTF-8 sequence, followed by marker.
func cutWithMarker(s string, limit int, marker string) string {
	if len(s) <= limit {
		return s
	}
	return cutAtRuneBoundary(s, limit-len(marker)) + marker
}

// CutWithMarker is cutWithMarker for other packages (the flow bridge in internal/server).
func CutWithMarker(s string, limit int, marker string) string { return cutWithMarker(s, limit, marker) }

// flowHistoryTriggerData returns data cut to flowHistoryTriggerDataMaxBytes at a rune
// boundary, ending with flowHistoryTruncatedMarker when it was cut.
func flowHistoryTriggerData(data string) string {
	return cutWithMarker(data, flowHistoryTriggerDataMaxBytes, flowHistoryTruncatedMarker)
}

// boundedCompletionOutputs encodes a flow's outputs once for its mission_completed
// dependents, as a preview beyond flowCompletionOutputsMaxBytes. It can take a while for
// large outputs, so callers run it without m.mu held. A truncation is logged at Debug with
// sizes only.
func boundedCompletionOutputs(outputs map[string]any) json.RawMessage {
	enc, err := json.Marshal(outputs)
	if err == nil && len(enc) <= flowCompletionOutputsMaxBytes {
		return enc
	}
	preview := "<unserializable>"
	if err == nil {
		preview = cutAtRuneBoundary(string(enc), flowCompletionOutputsPreviewBytes)
		slog.Debug("[MissionV2] Flow outputs for mission_completed dependents truncated", "size_bytes", len(enc),
			"limit_bytes", flowCompletionOutputsMaxBytes, "preview_bytes", len(preview))
	} else {
		slog.Debug("[MissionV2] Flow outputs for mission_completed dependents could not be encoded", "keys", len(outputs))
	}
	enc, _ = json.Marshal(map[string]any{"_truncated": true, "_preview": preview})
	return enc
}

// FlowRunStarted marks a flow mission as running and records the live run in the mission
// history. It returns the history run id ("" without a history database). The history and
// the audit start keep at most flowHistoryTriggerDataMaxBytes of the trigger data.
//
// It does not rewrite the missions file: the running state lives in memory (save writes
// flow missions idle), and LastRun is persisted with the result by FlowRunFinishedAtDepth.
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

// FlowRunFinishedAtDepth records the result of a live flow run, releases the running state
// and fires dependent missions and flows. It never touches the agent queue's running slot.
// depth is the run's mission_completed chain depth, CompletionChainDepth of the trigger type
// and data that started it (the flow bridge reads them from the run record); the dependents
// run one deeper, and none fire beyond maxCompletionChainDepth.
//
// Call it ONLY for runs whose FlowRunStarted was called: it releases one running slot. A
// run that never started (queued and cancelled, or stopped by a shutdown) is not reported
// here. Without a running slot to release the call changes no counter or status beyond
// clearing them, counts no run and fires no dependents.
//
// The history entry historyID is completed even when the mission is gone, because deleting a
// flow deletes its mission before it cancels the flow's runs.
func (m *MissionManagerV2) FlowRunFinishedAtDepth(missionID, historyID, result, output string, outputs map[string]any, depth int) {
	// Outputs can reach 32 MiB: encode them before taking the lock.
	var encodedOutputs json.RawMessage
	if outputs != nil {
		encodedOutputs = boundedCompletionOutputs(outputs)
	}
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
		// Only the in-memory status changes; the missions file holds flow missions idle.
		delete(m.flowActive, missionID)
		mission.Status = MissionStatusIdle
		name := mission.Name
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
	mission.LastOutput = cutWithMarker(output, flowLastOutputMaxBytes, completionTruncatedMarker)
	mission.RunCount++
	name := mission.Name
	completeCB := m.onMissionComplete
	queued := m.enqueueCompletionDependentsAtDepthLocked(missionID, result, output, encodedOutputs, depth)
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
		m.runAsync(func() { completeCB(missionID, result, output) })
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

// enqueueCompletionDependentsAtDepthLocked queues prompt missions and starts flows that wait
// for sourceID, whose finished run had the mission_completed chain depth depth. The trigger
// data carries chain_depth (depth + 1), the output (≤ completionOutputMaxBytes, rune-safe)
// and, for flow sources, the outputs of the flow's final nodes, already encoded and bounded
// by boundedCompletionOutputs (prompt sources pass nil). It returns the number of prompt
// missions it queued. Caller holds m.mu.
//
// This is the one place that bounds mission_completed chains, for prompt missions and flows
// alike: when depth + 1 exceeds maxCompletionChainDepth nothing fires, and, when a
// dependent waits, stopCompletionChainLocked warns and notes the stop on sourceID.
func (m *MissionManagerV2) enqueueCompletionDependentsAtDepthLocked(sourceID, result, output string, outputs json.RawMessage, depth int) int {
	return m.enqueueCompletionDependentsForOwnerLocked(sourceID, result, output, outputs, depth, QueueItem{})
}

// enqueueCompletionDependentsForOwnerLocked is enqueueCompletionDependentsAtDepthLocked for
// a finished run that may hold a Desktop owner (owner.RequiresOwner, see QueueOwnedMission).
// Dependent prompt missions of an owned run retain that owner, and remote ones are skipped,
// because remote execution cannot honour a revocation. Flows that wait for the completion
// start as usual: their runs have the independent lifecycle of the flow service. Caller
// holds m.mu.
func (m *MissionManagerV2) enqueueCompletionDependentsForOwnerLocked(sourceID, result, output string, outputs json.RawMessage, depth int, owner QueueItem) int {
	ev := flowEvent{SourceMissionID: sourceID, Result: result}
	next := depth + 1
	if next > maxCompletionChainDepth {
		for _, mission := range m.missions {
			if isCompletionDependent(mission, ev) {
				m.stopCompletionChainLocked(sourceID, depth)
				break
			}
		}
		return 0
	}
	data := map[string]any{"source_mission": sourceID, "result": result,
		"output": cutWithMarker(output, completionOutputMaxBytes, completionTruncatedMarker), completionChainDepthKey: next}
	if outputs != nil {
		data["outputs"] = outputs
	}
	raw, _ := json.Marshal(data)
	// Agent missions get the outputs capped (agentCompletionOutputsMaxBytes), built once for
	// the first of them; flows get raw.
	var agentRaw []byte
	queued := 0
	now := time.Now()
	for _, mission := range m.missions {
		if isFlowMission(mission) || !isCompletionDependent(mission, ev) {
			continue
		}
		if !m.shouldFireTriggerLocked(mission, string(TriggerMissionCompleted), now) {
			continue
		}
		if agentRaw == nil {
			agentRaw = raw
			if capped := capCompletionOutputsForAgents(outputs); len(capped) != len(outputs) {
				data["outputs"] = capped
				agentRaw, _ = json.Marshal(data)
				data["outputs"] = outputs
			}
		}
		item := QueueItem{MissionID: mission.ID, Priority: prioFromString(mission.Priority), EnqueuedAt: now,
			TriggerType: "mission_completed", TriggerData: string(agentRaw)}
		if owner.RequiresOwner {
			if owner.retainOwner == nil || isRemoteMission(mission) {
				continue
			}
			item.RequiresOwner, item.ownerContext, item.retainOwner = true, owner.ownerContext, owner.retainOwner
			item.releaseOwner = owner.retainOwner()
		}
		if !m.queue.enqueueItem(item) {
			if item.releaseOwner != nil {
				item.releaseOwner()
			}
			continue
		}
		mission.Status = MissionStatusQueued
		queued++
	}
	m.notifyFlowsLocked(TriggerMissionCompleted, ev, json.RawMessage(raw))
	return queued
}

// isCompletionDependent reports whether mission waits for the completion ev describes: an
// enabled prompt mission with a matching mission_completed trigger, or an enabled flow
// mission with a matching trigger node (the filter of notifyFlowsLocked). Min-interval
// limits are not consulted.
func isCompletionDependent(mission *MissionV2, ev flowEvent) bool {
	if mission == nil || !mission.Enabled {
		return false
	}
	if isFlowMission(mission) {
		for _, spec := range mission.FlowTriggers {
			if flowEventMatches(spec, TriggerMissionCompleted, ev) {
				return true
			}
		}
		return false
	}
	if mission.ExecutionType != ExecutionTriggered || mission.TriggerType != TriggerMissionCompleted {
		return false
	}
	cfg := mission.TriggerConfig
	return cfg != nil && cfg.SourceMissionID == ev.SourceMissionID &&
		(!cfg.RequireSuccess || ev.Result == MissionResultSuccess)
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
		return nil, true, ErrFlowsUnavailable
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
	// Only the enabled switch changes the registrations; a lock change keeps them (and the
	// MQTT min-interval state) and only registers what is missing.
	var regErr error
	if enabledChanged {
		regErr = m.syncFlowTriggersLocked(mission)
	} else {
		regErr = m.ensureFlowTriggersLocked(mission, false)
	}
	saveErr := m.save()
	if enabledChanged && m.flowHooks != nil {
		hooks, id, enabled := m.flowHooks, mission.ID, mission.Enabled
		m.runAsync(func() { hooks.FlowEnabledChanged(id, enabled) })
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
