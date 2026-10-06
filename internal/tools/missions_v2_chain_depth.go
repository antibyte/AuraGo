package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"
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

// completionChainPreviewKey marks trigger data that a size bound replaced by a preview (the
// flow run record keeps {"_preview": …} beyond flows.MaxStoredOutputBytes).
const completionChainPreviewKey = "_preview"

// CompletionChainDepth returns the chain depth of a run that a trigger of triggerType started
// with data: 0 for every trigger other than mission_completed; else the chain_depth number
// of data, clamped to 1…maxCompletionChainDepth. Data without chain_depth is 1 (data written
// before chain depths existed, such as a persisted queue item), unless it is a cut record
// (a _preview key): that is maxCompletionChainDepth, so a size bound that ever cuts
// mission_completed data stops the chain with the visible note instead of restarting it.
// The flow bridge (internal/server) passes a finished run's record.
func CompletionChainDepth(triggerType string, data map[string]any) int {
	if triggerType != string(TriggerMissionCompleted) {
		return 0
	}
	depth, ok := data[completionChainDepthKey]
	if !ok {
		if _, cut := data[completionChainPreviewKey]; cut {
			return maxCompletionChainDepth
		}
	}
	return clampCompletionChainDepth(depth)
}

// completionChainDepthRaw is CompletionChainDepth for trigger data held as JSON text, as
// queue items hold it. Text that is not a JSON object counts as data without the field.
func completionChainDepthRaw(triggerType, data string) int {
	if triggerType != string(TriggerMissionCompleted) {
		return 0
	}
	var fields struct {
		ChainDepth json.RawMessage `json:"chain_depth"`
		Preview    json.RawMessage `json:"_preview"`
	}
	if err := json.Unmarshal([]byte(data), &fields); err != nil {
		return 1
	}
	if fields.ChainDepth == nil {
		if fields.Preview != nil {
			return maxCompletionChainDepth
		}
		return 1
	}
	var depth any
	if err := json.Unmarshal(fields.ChainDepth, &depth); err != nil {
		return 1
	}
	return clampCompletionChainDepth(depth)
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

// completionChainWarnInterval is how often a stopped chain warns per source mission: the
// first stop warns, further stops of the same source within the interval log at Debug (a
// loop that fans out stops many branches at once, 2^5 of them per start at depth 10).
const completionChainWarnInterval = 10 * time.Minute

// stopCompletionChainLocked ends a chain at sourceID, whose finished run had depth: it logs
// (completionChainWarnInterval) and puts completionChainStoppedNote in front of the
// mission's LastOutput, which Mission Control shows. The run's output follows the note, cut
// at a rune boundary so that LastOutput keeps its cap of flowLastOutputMaxBytes (500 bytes,
// as prompt missions keep it too); the run's history entry keeps the whole output. Caller
// holds m.mu and saves the missions.
func (m *MissionManagerV2) stopCompletionChainLocked(sourceID string, depth int) {
	note := fmt.Sprintf(completionChainStoppedNote, depth)
	args := []any{"mission_id", sourceID, "depth", depth, "max_depth", maxCompletionChainDepth}
	if m.warnCompletionChainStopLocked(sourceID, time.Now()) {
		slog.Warn("[MissionV2] "+note, args...)
	} else {
		slog.Debug("[MissionV2] "+note, args...)
	}
	mission, ok := m.missions[sourceID]
	if !ok {
		return
	}
	if mission.LastOutput == "" {
		mission.LastOutput = note
		return
	}
	mission.LastOutput = cutWithMarker(note+"\n\n"+mission.LastOutput, flowLastOutputMaxBytes, completionTruncatedMarker)
}

// warnCompletionChainStopLocked reports whether a stop of sourceID's chain at now warns,
// and records the warning when it does. Entries older than completionChainWarnInterval are
// dropped on the way. Caller holds m.mu for writing.
func (m *MissionManagerV2) warnCompletionChainStopLocked(sourceID string, now time.Time) bool {
	for id, at := range m.chainStopWarned {
		if now.Sub(at) >= completionChainWarnInterval {
			delete(m.chainStopWarned, id)
		}
	}
	if _, recent := m.chainStopWarned[sourceID]; recent {
		return false
	}
	if m.chainStopWarned == nil {
		m.chainStopWarned = make(map[string]time.Time)
	}
	m.chainStopWarned[sourceID] = now
	return true
}
