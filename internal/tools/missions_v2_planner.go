package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type plannerFlowAdmission struct {
	MissionID string `json:"mission_id"`
	NodeID    string `json:"node_id"`
	State     string `json:"state"`
}
type plannerMissionAdmission struct {
	Flows []plannerFlowAdmission `json:"flows,omitempty"`
	Data  string                 `json:"data,omitempty"`
}

// NotifyPlannerAppointmentDueEvent admits an event and all matching destinations
// in the same durable queue snapshot. The event tombstone survives completion.
func (m *MissionManagerV2) NotifyPlannerAppointmentDueEvent(eventID, appointmentID, title, dateTime string) error {
	if strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("planner event ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx.Err() != nil {
		return fmt.Errorf("mission manager stopped")
	}
	if _, ok := m.plannerEvents[eventID]; ok {
		return nil
	}
	if m.plannerEvents == nil {
		m.plannerEvents = make(map[string]*plannerMissionAdmission)
	}
	now := time.Now()
	raw, _ := json.Marshal(map[string]string{"appointment_id": appointmentID, "title": title, "date_time": dateTime, "time": now.Format(time.RFC3339)})
	event := &plannerMissionAdmission{}
	priorStatuses := map[string]string{}
	priorResults := map[string]string{}
	priorFires := make(map[string]time.Time, len(m.lastTriggerFire))
	for key, value := range m.lastTriggerFire {
		priorFires[key] = value
	}
	for _, mission := range m.missions {
		if !mission.Enabled || isRemoteMission(mission) {
			continue
		}
		if isFlowMission(mission) {
			for _, spec := range mission.FlowTriggers {
				if flowEventMatches(spec, TriggerPlannerAppointmentDue, flowEvent{Title: title}) && (spec.TriggerConfig == nil || spec.TriggerConfig.PlannerAppointmentID == "" || spec.TriggerConfig.PlannerAppointmentID == appointmentID) && m.shouldFireFlowSpecLocked(mission.ID, spec, now) {
					event.Flows = append(event.Flows, plannerFlowAdmission{MissionID: mission.ID, NodeID: spec.NodeID, State: "pending"})
				}
			}
			continue
		}
		if mission.ExecutionType != ExecutionTriggered || mission.TriggerType != TriggerPlannerAppointmentDue {
			continue
		}
		cfg := mission.TriggerConfig
		if cfg == nil {
			cfg = &TriggerConfig{}
		}
		if (cfg.PlannerAppointmentID != "" && cfg.PlannerAppointmentID != appointmentID) || (cfg.PlannerTitleContains != "" && !strings.Contains(strings.ToLower(title), strings.ToLower(cfg.PlannerTitleContains))) {
			continue
		}
		if !m.shouldFireTriggerLocked(mission, string(TriggerPlannerAppointmentDue), now) {
			continue
		}
		if m.queue.enqueueItem(QueueItem{EventID: eventID, MissionID: mission.ID, Priority: prioFromString(mission.Priority), EnqueuedAt: now, TriggerType: string(TriggerPlannerAppointmentDue), TriggerData: string(raw)}) {
			priorStatuses[mission.ID] = mission.Status
			priorResults[mission.ID] = mission.LastResult
			mission.Status = MissionStatusQueued
			mission.LastResult = ""
		}
	}
	if len(event.Flows) > 0 {
		event.Data = string(raw)
	}
	m.plannerEvents[eventID] = event
	if err := m.saveQueueLocked(); err != nil {
		delete(m.plannerEvents, eventID)
		m.lastTriggerFire = priorFires
		for id, status := range priorStatuses {
			m.missions[id].Status = status
			m.missions[id].LastResult = priorResults[id]
		}
		m.queue.mu.Lock()
		retained := m.queue.items[:0]
		for _, item := range m.queue.items {
			if item.EventID != eventID {
				retained = append(retained, item)
			}
		}
		m.queue.items = retained
		m.queue.mu.Unlock()
		return err
	}
	if err := m.save(); err != nil {
		slog.Error("Planner admission persisted; mission status mirror could not be saved", "error", err)
	}
	return nil
}

// processPlannerFlowAdmissions starts at most one durable flow target per tick.
// A dispatch marker survives process loss; an uncertain hook result is never replayed.
func (m *MissionManagerV2) processPlannerFlowAdmissions() {
	m.mu.Lock()
	if m.ctx.Err() != nil || m.flowHooks == nil {
		m.mu.Unlock()
		return
	}
	for _, event := range m.plannerEvents {
		for i := range event.Flows {
			target := &event.Flows[i]
			if target.State != "pending" {
				continue
			}
			mission := m.missions[target.MissionID]
			if mission == nil || !mission.Enabled || !isFlowMission(mission) {
				target.State = "canceled"
				_ = m.saveQueueLocked()
				continue
			}
			target.State = "dispatching"
			if err := m.saveQueueLocked(); err != nil {
				target.State = "pending"
				m.mu.Unlock()
				return
			}
			hooks, id, node, data := m.flowHooks, target.MissionID, target.NodeID, event.Data
			m.mu.Unlock()
			err := func() (err error) {
				defer func() {
					if recover() != nil {
						err = fmt.Errorf("planner flow dispatch outcome unknown")
					}
				}()
				return hooks.StartFlowRun(id, node, string(TriggerPlannerAppointmentDue), data)
			}()
			m.mu.Lock()
			if err != nil {
				target.State = "unknown"
				if mission := m.missions[id]; mission != nil {
					mission.LastOutput = "Planner flow dispatch outcome unknown; verify before retrying"
				}
				slog.Warn("Planner flow admission outcome unknown", "mission_id", id)
				if err := m.save(); err != nil {
					slog.Error("Failed to persist planner flow review notice", "error", err)
				}
			} else {
				target.State = "accepted"
			}
			if err := m.saveQueueLocked(); err != nil {
				slog.Error("Failed to save planner flow dispatch outcome", "error", err)
			}
			m.mu.Unlock()
			return
		}
	}
	m.mu.Unlock()
}
