package server

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

const (
	// flowHookTimeout bounds FlowMissionDeleted and FlowEnabledChanged. Both wait for the
	// flow's lock; without a bound, a stuck lock holder would keep one goroutine per Mission
	// Control delete or switch for ever. Once a hook holds the lock the Service finishes
	// without the deadline (it detaches), so the bound covers the lookup and the wait.
	flowHookTimeout = 2 * time.Minute
)

// flowMissionBridge implements flows.MissionBridge on top of MissionManagerV2, the mission
// history, the planner and notifications.
//
// Like every MissionBridge it never calls a Service method that takes a flow lock
// (Publish, SetEnabled, DeleteFlow, DeleteFlowForMission, MissionEnabledChanged,
// ReconcileMissions): the
// Service calls it while it holds one. broadcastMissionState, which most methods call,
// reads the missions and the queue under the manager's own lock and asks each flow
// mission's next run through FlowHooks.NextFlowRun, which ends in the lock-free
// Service.NextTimer.
type flowMissionBridge struct{ s *Server }

func (b flowMissionBridge) missions() (*tools.MissionManagerV2, error) {
	if b.s.MissionManagerV2 == nil {
		return nil, errors.New("Mission Control is not available")
	}
	return b.s.MissionManagerV2, nil
}

// CreateFlowMission implements flows.MissionBridge.
func (b flowMissionBridge) CreateFlowMission(flowID, name string) (string, error) {
	mm, err := b.missions()
	if err != nil {
		return "", err
	}
	id, err := mm.CreateFlowMission(flowID, name)
	broadcastMissionState(b.s)
	return id, err
}

// SyncFlowMission implements flows.MissionBridge.
func (b flowMissionBridge) SyncFlowMission(missionID, name string, bindings []flows.TriggerBinding) error {
	mm, err := b.missions()
	if err != nil {
		return err
	}
	specs, err := flowTriggerSpecs(bindings)
	if err != nil {
		return err
	}
	err = mm.SyncFlowMission(missionID, name, specs)
	broadcastMissionState(b.s)
	return err
}

// SetFlowMissionEnabled implements flows.MissionBridge.
func (b flowMissionBridge) SetFlowMissionEnabled(missionID string, enabled bool) error {
	mm, err := b.missions()
	if err != nil {
		return err
	}
	err = mm.SetFlowMissionEnabled(missionID, enabled)
	broadcastMissionState(b.s)
	return err
}

// FlowMissionEnabled implements flows.MissionBridge.
func (b flowMissionBridge) FlowMissionEnabled(missionID string) bool {
	mm, err := b.missions()
	if err != nil {
		return false
	}
	m, ok := mm.Get(missionID)
	return ok && m.Enabled
}

var _ flows.MissionReconciler = flowMissionBridge{}

// FlowMissions implements flows.MissionReconciler: the flow missions Mission Control
// holds, mission id → flow id; nil without Mission Control.
func (b flowMissionBridge) FlowMissions() map[string]string {
	mm, err := b.missions()
	if err != nil {
		return nil
	}
	out := map[string]string{}
	for _, m := range mm.List() {
		if m.ExecutionType == tools.ExecutionFlow {
			out[m.ID] = m.FlowID
		}
	}
	return out
}

// FlowMissionInSync implements flows.MissionReconciler: the mission is a published flow
// mission whose name and trigger specs equal what SyncFlowMission would store (a blank
// name keeps the stored one there, so it matches any).
func (b flowMissionBridge) FlowMissionInSync(missionID, name string, bindings []flows.TriggerBinding) bool {
	mm, err := b.missions()
	if err != nil {
		return false
	}
	specs, err := flowTriggerSpecs(bindings)
	if err != nil {
		return false
	}
	m, ok := mm.Get(missionID)
	if !ok || m.ExecutionType != tools.ExecutionFlow || !m.FlowPublished {
		return false
	}
	if strings.TrimSpace(name) != "" && m.Name != name {
		return false
	}
	if len(m.FlowTriggers) == 0 && len(specs) == 0 {
		return true
	}
	return reflect.DeepEqual(m.FlowTriggers, specs)
}

// DeleteFlowMission implements flows.MissionBridge.
func (b flowMissionBridge) DeleteFlowMission(missionID string) error {
	mm, err := b.missions()
	if err != nil {
		return err
	}
	err = mm.DeleteFlowMission(missionID)
	broadcastMissionState(b.s)
	return err
}

// FlowRunStarted implements flows.MissionBridge. The trigger data (untrusted, at most
// flows.MaxStoredOutputBytes of JSON) is scrubbed value by value, within the budget of what
// the history keeps (boundFlowTriggerData), before it is encoded for the history.
func (b flowMissionBridge) FlowRunStarted(missionID string, rec flows.RunRecord) string {
	mm, err := b.missions()
	if err != nil {
		return ""
	}
	id := mm.FlowRunStarted(missionID, rec.TriggerType, boundFlowTriggerData(rec.TriggerData))
	broadcastMissionState(b.s)
	return id
}

// FlowRunFinished implements flows.MissionBridge: history, dependents, planner issue and
// the failure notification chosen by settings.notify_on_error.
//
// Only a started run (info.Started) reaches Mission Control: mm.FlowRunFinished releases
// one running slot, counts the run and fires dependents. A run that never started
// (cancelled while queued, or ended by a shutdown) only tells open editors to refresh.
// That path stays free of database, file and network work, because it runs synchronously
// inside DeleteFlow (with the flow lock held), CancelMissionRuns and Runner.Shutdown, once
// per queued run.
//
// MissionID and HistoryID may be empty for a started run (its flow is gone, or no history
// entry was made); mm.FlowRunFinished then completes what it can, and the planner issue
// carries no mission reference.
func (b flowMissionBridge) FlowRunFinished(info flows.RunFinishedInfo) {
	if !info.Started {
		b.s.Logger.Debug("Flow run ended before it started; Mission Control does not record it",
			"flow", info.Record.FlowID, "run", info.Record.ID, "status", string(info.Result.Status))
		b.s.broadcastFlowsChanged(info.Record.FlowID, "run_finished")
		return
	}
	mm, err := b.missions()
	if err != nil {
		return
	}
	// One bounded, scrubbed copy of the outputs, encoded once, gives the dependents' outputs
	// and the success text; the work does not grow with the outputs (up to 32 MiB).
	outputs := boundFlowOutputs(info.Outputs)
	result, output := flowOutcome(info, outputs)
	mm.FlowRunFinished(info.MissionID, info.HistoryID, result, output, outputs.mission)
	if info.Result.Status != flows.RunCancelled {
		failed := result != tools.MissionResultSuccess
		b.s.flowIssue(info, failed, output)
		if failed {
			b.s.notifyFlowFailure(info, output)
		} else {
			b.s.flowNotify.recovered(info.Record.FlowID)
		}
	}
	broadcastMissionState(b.s)
	b.s.broadcastFlowsChanged(info.Record.FlowID, "run_finished")
}

// flowMissionHooks lets Mission Control start and manage flow runs (tools.FlowHooks).
//
// How MissionManagerV2 (internal/tools) calls the hooks, and why none of them can wait for
// a flow lock that its caller holds:
//   - FlowMissionDeleted and FlowEnabledChanged take the flow's lock (through
//     Service.DeleteFlowForMission and Service.MissionEnabledChanged). The manager calls them
//     only on goroutines of their own, after its own lock is released or never held by
//     them: `go hooks.FlowMissionDeleted(id)` in MissionManagerV2.Delete (missions_v2.go)
//     and `go hooks.FlowEnabledChanged(id, enabled)` in updateFlowMissionLocked
//     (missions_v2_flow_runs.go). No bridge method calls them.
//   - StartFlowRun is called synchronously, outside the manager's lock: by RunNow and
//     TriggerMissionWithOptions (missions_v2.go), by fireFlowEvent and fireFlowSchedule
//     (missions_v2_flows.go, the webhook, email, MQTT and cron registrations), and, for the
//     Notify* events and mission_completed dependents, by the dispatcher goroutine that
//     notifyFlowsLocked feeds (dispatchFlowEvents). It calls Service.TriggerFromMission,
//     which takes no flow lock, so a run that finishes inside a flow delete can start its
//     dependents.
//   - NextFlowRun is called synchronously by nextFlowRun (missions_v2_flow_runs.go) after
//     the manager released its lock, also from broadcastMissionState while the Service holds
//     a flow lock. It calls Service.NextTimer, which takes no flow lock.
type flowMissionHooks struct{ s *Server }

// StartFlowRun implements tools.FlowHooks.
func (h flowMissionHooks) StartFlowRun(missionID, nodeID, triggerType, triggerData string) error {
	if h.s.Flows == nil {
		return errors.New("flows are not available")
	}
	_, err := h.s.Flows.TriggerFromMission(missionID, nodeID, triggerType, flows.NormalizeTriggerData(triggerType, triggerData))
	return err
}

// FlowMissionDeleted implements tools.FlowHooks. It takes the flow's lock, waiting at most
// flowHookTimeout.
func (h flowMissionHooks) FlowMissionDeleted(missionID string) {
	if h.s.Flows == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), flowHookTimeout)
	defer cancel()
	if err := h.s.Flows.DeleteFlowForMission(ctx, missionID); err != nil {
		h.logError("The flow of a deleted mission could not be removed", missionID, err)
		return
	}
	h.s.broadcastFlowsChanged("", "deleted")
}

// FlowEnabledChanged implements tools.FlowHooks. It takes the flow's lock, waiting at most
// flowHookTimeout.
func (h flowMissionHooks) FlowEnabledChanged(missionID string, _ bool) {
	if h.s.Flows == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), flowHookTimeout)
	defer cancel()
	if err := h.s.Flows.MissionEnabledChanged(ctx, missionID); err != nil {
		h.logError("Flow timers could not follow Mission Control", missionID, err)
		return
	}
	h.s.broadcastFlowsChanged("", "enabled")
}

// logError logs a failed FlowMissionDeleted or FlowEnabledChanged: a flow that is gone
// already (flows.ErrNotFound, a normal race with DeleteFlow) at Debug, a timeout and any
// other error at Warn, the error text bounded.
func (h flowMissionHooks) logError(msg, missionID string, err error) {
	switch {
	case errors.Is(err, flows.ErrNotFound):
		h.s.Logger.Debug(msg+": the flow is gone already", "mission_id", missionID)
	case errors.Is(err, context.DeadlineExceeded):
		h.s.Logger.Warn(msg+": the flow stayed busy", "mission_id", missionID, "timeout", flowHookTimeout.String())
	default:
		h.s.Logger.Warn(msg, "mission_id", missionID, "error", flowBoundRunes(err.Error(), flowErrorRunes))
	}
}

// NextFlowRun implements tools.FlowHooks.
func (h flowMissionHooks) NextFlowRun(missionID string) (time.Time, bool) {
	if h.s.Flows == nil {
		return time.Time{}, false
	}
	return h.s.Flows.NextTimer(context.Background(), missionID)
}

// broadcastFlowsChanged tells open editors to refresh their flow list.
func (s *Server) broadcastFlowsChanged(flowID, reason string) {
	broadcastDesktopEvent(s, s.DesktopHub, desktop.Event{Type: "flows_changed",
		Payload: map[string]interface{}{"flow_id": flowID, "reason": reason}, CreatedAt: time.Now().UTC()})
}
