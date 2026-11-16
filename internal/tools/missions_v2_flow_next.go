package tools

import "time"

// FlowNextRunner is an optional extension of FlowHooks, found by type assertion. SharedNextFlowRun
// answers the next time the flow behind missionID starts by itself, over both sources (Date/Time
// timers and schedule jobs); in AuraGo it is flows.Service.NextRunForMission, the function the
// flow API also uses, so Mission Control and EasyDrag always show the same time. Without it
// nextFlowRun combines the schedule jobs with FlowHooks.NextFlowRun itself.
type FlowNextRunner interface {
	SharedNextFlowRun(missionID string) (time.Time, bool)
}

// flowScheduleRun returns the earliest next fire among the cron jobs of the schedule triggers in
// specs and the node id that owns it. Only flowCronSource jobs count: a prompt mission whose id
// is "<flow>__<node>" owns a job with the same id. cron may be nil (no engine: nothing).
func flowScheduleRun(cron *CronManager, missionID string, specs []FlowTriggerSpec) (time.Time, string, bool) {
	if cron == nil || len(specs) == 0 {
		return time.Time{}, "", false
	}
	flowJobs := make(map[string]bool, len(specs))
	for _, job := range cron.GetJobs() {
		if job.Source == flowCronSource {
			flowJobs[job.ID] = true
		}
	}
	var next time.Time
	var node string
	for _, spec := range specs {
		if spec.TriggerType != FlowTriggerSchedule {
			continue
		}
		jobID := flowCronJobID(missionID, spec.NodeID)
		if !flowJobs[jobID] {
			continue
		}
		at, ok := cron.NextRun(jobID)
		if !ok || at.IsZero() {
			continue
		}
		if next.IsZero() || at.Before(next) || (at.Equal(next) && spec.NodeID < node) {
			next, node = at, spec.NodeID
		}
	}
	return next, node, !next.IsZero()
}

// NextFlowScheduleRun implements flows.ScheduleNexter for the server: the earliest fire after now
// among the schedule jobs of the flow with this id, and the schedule node that owns it. It
// answers for the mission as registered, whatever its switch says (flows.Service.NextRun
// checks the switch). Like nextFlowRun it holds m.mu only while it reads the mission.
func (m *MissionManagerV2) NextFlowScheduleRun(flowID string, now time.Time) (time.Time, string, bool) {
	m.mu.RLock()
	var (
		missionID string
		specs     []FlowTriggerSpec
	)
	for id, mission := range m.missions {
		if isFlowMission(mission) && mission.FlowID == flowID {
			missionID = id
			specs = copyFlowTriggers(mission.FlowTriggers)
			break
		}
	}
	cron := m.cron
	m.mu.RUnlock()
	if missionID == "" {
		return time.Time{}, "", false
	}
	at, node, ok := flowScheduleRun(cron, missionID, specs)
	if !ok || !at.After(now) {
		return time.Time{}, "", false
	}
	return at, node, true
}
