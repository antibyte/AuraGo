package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
)

// maxCompletionChainDepth bounds chains of missions that start each other on completion,
// prompt missions and flows alike. A run started any other way (manually, by a schedule or
// by an event) has depth 0, and every mission_completed dependent runs one deeper than the
// run that fired it. The completion of a run at this depth fires no dependents, so a loop
// between missions (flow A and flow B, a flow and a prompt mission, a prompt mission and
// itself) ends after maxCompletionChainDepth + 1 runs, while a linear chain of up to
// maxCompletionChainDepth links runs whole.
const maxCompletionChainDepth = 10

// completionChainDepthKey is the field of mission_completed trigger data that holds the
// chain depth of the run it starts.
const completionChainDepthKey = "chain_depth"

// completionChainStoppedNote (with the depth) goes in front of the LastOutput of a mission
// whose completion did not fire its dependents because the chain reached
// maxCompletionChainDepth.
const completionChainStoppedNote = "Stopped a chain of missions triggered by completions after %d steps; check for a loop between missions"

// CompletionChainDepth returns the chain depth of a run that a trigger of triggerType started
// with data: 0 for every trigger other than mission_completed; else the chain_depth number
// of data, clamped to 1…maxCompletionChainDepth; 1 when data has no such number
// (mission_completed data written before chain depths existed, such as a persisted queue
// item). The flow bridge (internal/server) passes a finished run's record.
func CompletionChainDepth(triggerType string, data map[string]any) int {
	if triggerType != string(TriggerMissionCompleted) {
		return 0
	}
	return clampCompletionChainDepth(data[completionChainDepthKey])
}

// completionChainDepthRaw is CompletionChainDepth for trigger data held as JSON text, as
// queue items hold it. Text that is not a JSON object counts as data without the field.
func completionChainDepthRaw(triggerType, data string) int {
	if triggerType != string(TriggerMissionCompleted) {
		return 0
	}
	var fields struct {
		ChainDepth any `json:"chain_depth"`
	}
	if err := json.Unmarshal([]byte(data), &fields); err != nil {
		return 1
	}
	return clampCompletionChainDepth(fields.ChainDepth)
}

// clampCompletionChainDepth reads a chain_depth value: a JSON number (float64, or int as
// the manager stores it) clamped to 1…maxCompletionChainDepth, and 1 for anything else.
func clampCompletionChainDepth(v any) int {
	var f float64
	switch n := v.(type) {
	case float64:
		f = n
	case int:
		f = float64(n)
	default:
		return 1
	}
	switch {
	case math.IsNaN(f) || f < 1:
		return 1
	case f > maxCompletionChainDepth:
		return maxCompletionChainDepth
	}
	return int(f)
}

// setActiveChainDepthLocked remembers the chain depth of the prompt mission run that starts
// now, for OnMissionComplete. The agent queue runs one mission at a time, so the mission
// id identifies the run. Depth 0 is not stored. Caller holds m.mu.
func (m *MissionManagerV2) setActiveChainDepthLocked(missionID string, depth int) {
	if depth <= 0 {
		delete(m.activeChainDepth, missionID)
		return
	}
	if m.activeChainDepth == nil {
		m.activeChainDepth = make(map[string]int)
	}
	m.activeChainDepth[missionID] = depth
}

// stopCompletionChainLocked ends a chain at sourceID, whose finished run had depth: it logs
// a warning and puts completionChainStoppedNote in front of the mission's LastOutput, which
// Mission Control shows (the run's history entry keeps its own output). Caller holds m.mu
// and saves the missions.
func (m *MissionManagerV2) stopCompletionChainLocked(sourceID string, depth int) {
	note := fmt.Sprintf(completionChainStoppedNote, depth)
	slog.Warn("[MissionV2] "+note, "mission_id", sourceID, "depth", depth, "max_depth", maxCompletionChainDepth)
	mission, ok := m.missions[sourceID]
	if !ok {
		return
	}
	if mission.LastOutput == "" {
		mission.LastOutput = note
		return
	}
	mission.LastOutput = note + "\n\n" + mission.LastOutput
}
