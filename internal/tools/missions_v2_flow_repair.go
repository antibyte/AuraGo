package tools

import (
	"errors"
	"strings"
	"time"
)

// ErrFlowMissionConflict means the stored mission identity cannot safely be restored.
var ErrFlowMissionConflict = errors.New("flow mission identity conflicts with an existing mission")

// EnsureRestoredFlowMission restores the exact missing identity in a disabled state.
// A retry accepts the same disabled owner, but never takes over another mission.
func (m *MissionManagerV2) EnsureRestoredFlowMission(missionID, flowID, name string, specs []FlowTriggerSpec, published bool) error {
	if err := requireMissionMutationPermission(); err != nil {
		return err
	}
	if strings.TrimSpace(missionID) == "" || strings.TrimSpace(flowID) == "" {
		return ErrFlowMissionConflict
	}
	if err := validateFlowTriggerSpecs(specs); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, mission := range m.missions {
		if id != missionID && mission.FlowID == flowID {
			return ErrFlowMissionConflict
		}
	}
	mission, ok := m.missions[missionID]
	var previous MissionV2
	if ok {
		if !isFlowMission(mission) || mission.FlowID != flowID || mission.Enabled {
			return ErrFlowMissionConflict
		}
		if mission.Locked {
			return ErrMissionLocked
		}
		previous = *mission
	} else {
		mission = &MissionV2{ID: missionID, ExecutionType: ExecutionFlow, FlowID: flowID,
			Priority: "medium", Status: MissionStatusIdle, CreatedAt: time.Now(), RunnerType: MissionRunnerLocal}
		m.missions[missionID] = mission
	}
	mission.Name = name
	mission.FlowPublished = published
	mission.FlowTriggers = copyFlowTriggers(specs)
	if err := m.save(); err != nil {
		if ok {
			*mission = previous
		} else {
			delete(m.missions, missionID)
		}
		return err
	}
	if ok {
		m.unregisterFlowTriggersLocked(&previous)
	}
	return m.syncFlowTriggersLocked(mission)
}
