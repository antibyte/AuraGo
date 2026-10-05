package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/planner"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// flowMissionBridge implements flows.MissionBridge on top of MissionManagerV2, the mission
// history, the planner and notifications.
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

// FlowRunStarted implements flows.MissionBridge.
func (b flowMissionBridge) FlowRunStarted(missionID string, rec flows.RunRecord) string {
	mm, err := b.missions()
	if err != nil {
		return ""
	}
	data, _ := json.Marshal(rec.TriggerData)
	id := mm.FlowRunStarted(missionID, rec.TriggerType, security.Scrub(string(data)))
	broadcastMissionState(b.s)
	return id
}

// FlowRunFinished implements flows.MissionBridge: history, dependents, planner issue and
// the failure notification chosen by settings.notify_on_error.
func (b flowMissionBridge) FlowRunFinished(info flows.RunFinishedInfo) {
	mm, err := b.missions()
	if err != nil {
		return
	}
	result, output := flowRunOutcome(info)
	output = security.Scrub(output)
	mm.FlowRunFinished(info.MissionID, info.HistoryID, result, output, scrubFlowOutputs(info.Outputs))
	if info.Result.Status != flows.RunCancelled {
		b.s.flowIssue(info, result != tools.MissionResultSuccess, output)
		if result != tools.MissionResultSuccess {
			b.s.notifyFlowFailure(info, output)
		}
	}
	broadcastMissionState(b.s)
	b.s.broadcastFlowsChanged(info.Record.FlowID, "run_finished")
}

// flowTriggerSpecs converts published trigger bindings into Mission Control trigger specs.
func flowTriggerSpecs(bindings []flows.TriggerBinding) ([]tools.FlowTriggerSpec, error) {
	specs := make([]tools.FlowTriggerSpec, 0, len(bindings))
	for _, b := range bindings {
		spec := tools.FlowTriggerSpec{NodeID: b.NodeID}
		switch b.Kind {
		case flows.BindingMission:
			spec.TriggerType = tools.TriggerType(b.MissionTrigger)
			cfg := &tools.TriggerConfig{}
			if len(b.Config) > 0 {
				data, err := json.Marshal(b.Config)
				if err != nil {
					return nil, fmt.Errorf("trigger %s: %w", b.NodeID, err)
				}
				if err := json.Unmarshal(data, cfg); err != nil {
					return nil, fmt.Errorf("trigger %s: %w", b.NodeID, err)
				}
			}
			spec.TriggerConfig = cfg
		case flows.BindingCron:
			spec.TriggerType, spec.Schedule = tools.FlowTriggerSchedule, b.Schedule
		case flows.BindingTimer:
			spec.TriggerType = tools.FlowTriggerDateTime
		case flows.BindingManual:
			spec.TriggerType = tools.FlowTriggerManual
		default:
			return nil, fmt.Errorf("trigger %s has an unknown binding %q", b.NodeID, b.Kind)
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// flowRunOutcome maps a finished run to a Mission Control result and output text.
func flowRunOutcome(info flows.RunFinishedInfo) (string, string) {
	switch info.Result.Status {
	case flows.RunSuccess:
		if len(info.Outputs) == 0 {
			return tools.MissionResultSuccess, ""
		}
		data, _ := json.Marshal(info.Outputs)
		return tools.MissionResultSuccess, string(data)
	case flows.RunCancelled:
		msg := strings.TrimSpace(info.Result.ErrorMessage)
		if msg == "" || info.Result.ErrorCode == "" {
			msg = tools.MissionCancelledOutput
		}
		return tools.MissionResultError, msg
	default:
		msg := strings.TrimSpace(info.Result.ErrorMessage)
		if msg == "" {
			msg = "the flow failed"
		}
		if info.Result.ErrorCode != "" {
			msg = info.Result.ErrorCode + ": " + msg
		}
		return tools.MissionResultError, msg
	}
}

func scrubFlowOutputs(outputs map[string]any) map[string]any {
	if outputs == nil {
		return nil
	}
	data, err := json.Marshal(outputs)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal([]byte(security.Scrub(string(data))), &out) != nil {
		return nil
	}
	return out
}

// flowIssue records (or resolves) the planner operational issue of a flow, like missions do.
func (s *Server) flowIssue(info flows.RunFinishedInfo, failed bool, message string) {
	if s.PlannerDB == nil {
		return
	}
	fingerprint := "flow|" + info.Record.FlowID
	if !failed {
		if _, err := planner.ResolveOperationalIssue(s.PlannerDB, fingerprint, "The flow ran successfully.", time.Now()); err != nil {
			s.Logger.Warn("Flow operational issue could not be resolved", "flow", info.Record.FlowID, "error", err)
		}
		return
	}
	issue := planner.OperationalIssue{Source: "flow", Context: info.Record.FlowID, Title: fmt.Sprintf("Flow %s failed", info.FlowName),
		Detail: message, Severity: "error", Kind: planner.OperationalIssueKindRuntimeFailure, Reference: info.MissionID,
		Fingerprint: fingerprint, OccurredAt: time.Now()}
	issueID, err := planner.RecordOperationalIssue(s.PlannerDB, issue)
	if err != nil {
		s.Logger.Warn("Flow operational issue could not be recorded", "flow", info.Record.FlowID, "error", err)
		return
	}
	if s.MissionManagerV2 != nil {
		s.MissionManagerV2.NotifyPlannerOperationalIssue(issueID, issue.Source, issue.Severity, issue.Title)
	}
}

// flowFailureNotification builds the desktop notification for a failed live run.
func flowFailureNotification(lang string, info flows.RunFinishedInfo, message string) map[string]any {
	return map[string]any{
		"title":   i18n.T(lang, "easydrag.notify.run_failed_title"),
		"message": i18n.T(lang, "easydrag.notify.run_failed_message", map[string]any{"name": info.FlowName, "error": message}),
		"type":    "error",
		"appId":   "easydrag",
		"context": map[string]any{"flow_id": info.Record.FlowID, "run_id": info.Record.ID},
	}
}

// notifyFlowFailure sends the failure notification chosen by settings.notify_on_error:
// desktop (default), push, telegram or off.
func (s *Server) notifyFlowFailure(info flows.RunFinishedInfo, message string) {
	cfg := s.ConfigSnapshot()
	if cfg == nil || info.NotifyOnError == "off" {
		return
	}
	payload := flowFailureNotification(cfg.Server.UILanguage, info, message)
	switch info.NotifyOnError {
	case "push", "telegram":
		title, _ := payload["title"].(string)
		body, _ := payload["message"].(string)
		go tools.SendNotification(cfg, s.Logger, info.NotifyOnError, title, body, "high", nil)
	default:
		broadcastDesktopEvent(s, s.DesktopHub, desktop.Event{Type: "notification", Payload: payload, CreatedAt: time.Now().UTC()})
	}
}

// flowMissionHooks lets Mission Control start and manage flow runs (tools.FlowHooks).
type flowMissionHooks struct{ s *Server }

// StartFlowRun implements tools.FlowHooks.
func (h flowMissionHooks) StartFlowRun(missionID, nodeID, triggerType, triggerData string) error {
	if h.s.Flows == nil {
		return errors.New("flows are not available")
	}
	_, err := h.s.Flows.TriggerFromMission(missionID, nodeID, triggerType, flows.NormalizeTriggerData(triggerType, triggerData))
	return err
}

// FlowMissionDeleted implements tools.FlowHooks.
func (h flowMissionHooks) FlowMissionDeleted(missionID string) {
	if h.s.Flows == nil {
		return
	}
	if err := h.s.Flows.DeleteFlowForMission(context.Background(), missionID); err != nil {
		h.s.Logger.Warn("The flow of a deleted mission could not be removed", "mission_id", missionID, "error", err)
		return
	}
	h.s.broadcastFlowsChanged("", "deleted")
}

// FlowEnabledChanged implements tools.FlowHooks.
func (h flowMissionHooks) FlowEnabledChanged(missionID string, _ bool) {
	if h.s.Flows == nil {
		return
	}
	if err := h.s.Flows.MissionEnabledChanged(context.Background(), missionID); err != nil {
		h.s.Logger.Warn("Flow timers could not follow Mission Control", "mission_id", missionID, "error", err)
		return
	}
	h.s.broadcastFlowsChanged("", "enabled")
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
