package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/planner"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// Bounds of what the bridge hands on to Mission Control, the planner and the logs.
const (
	// flowMissionOutputMaxBytes caps the output text of a finished live run that Mission
	// Control gets. On success it is the JSON of the run's final outputs, which can reach
	// flows.MaxRunOutputBytes (32 MiB). The agent path caps nothing before
	// MissionManagerV2.SetResult, and Mission Control keeps at most 2000 bytes of the text
	// (history output, the "output" of mission_completed dependents, the audit detail;
	// LastOutput 500), but the audit recorder scrubs the whole text before it cuts it. 16 KiB
	// leaves every consumer its full share. A longer text is cut at a rune boundary and ends
	// with flowCutMarker.
	flowMissionOutputMaxBytes = 16 << 10
	// flowCutMarker ends a text that flowCutText cut.
	flowCutMarker = "…"
	// flowScrubMaxDepth bounds the nesting scrubFlowValue walks. Flow values are
	// JSON-normalised (the engine encodes and decodes every node output, and trigger data
	// comes from json.Unmarshal in NormalizeTriggerData or the store), and encoding/json does
	// not decode nesting deeper than 10000 levels (its maxNestingDepth), which is the bound
	// flows.ParseToolOutput and the engine inherit. A deeper value is dropped (nil) rather
	// than passed on unscrubbed.
	flowScrubMaxDepth = 10000
	// flowNotifyNameRunes bounds the flow name in the planner issue title and in failure
	// notifications.
	flowNotifyNameRunes = 80
	// flowHookTimeout bounds FlowMissionDeleted and FlowEnabledChanged. Both wait for the
	// flow's lock; without a bound, a stuck lock holder would keep one goroutine per Mission
	// Control delete or switch for ever. Once a hook holds the lock the Service finishes
	// without the deadline (it detaches), so the bound covers the lookup and the wait.
	flowHookTimeout = 2 * time.Minute
)

// flowMissionTriggerTypes are the Mission Control trigger types a flows.BindingMission may
// use: the ones MissionManagerV2 registers for a flow mission (ensureFlowTriggersLocked:
// webhook, email, MQTT) and the ones it starts flows for through notifyFlowsLocked (the
// Notify* events and mission_completed). MissionManagerV2.SyncFlowMission checks only the
// node ids, so a spec of any other type would be stored and never fire. Egg, nest and
// planner_operational_issue events start no flows; schedule, datetime and manual are
// separate binding kinds. TestC15CatalogTriggerConfigsDecode keeps the catalog and this
// set in step.
var flowMissionTriggerTypes = map[tools.TriggerType]bool{
	tools.TriggerWebhook:               true,
	tools.TriggerEmailReceived:         true,
	tools.TriggerMQTTMessage:           true,
	tools.TriggerSystemStartup:         true,
	tools.TriggerDeviceConnected:       true,
	tools.TriggerDeviceDisconnected:    true,
	tools.TriggerFritzBoxCall:          true,
	tools.TriggerBudgetWarning:         true,
	tools.TriggerBudgetExceeded:        true,
	tools.TriggerHomeAssistantState:    true,
	tools.TriggerPlannerAppointmentDue: true,
	tools.TriggerPlannerTodoOverdue:    true,
	tools.TriggerMissionCompleted:      true,
}

// flowMissionBridge implements flows.MissionBridge on top of MissionManagerV2, the mission
// history, the planner and notifications.
//
// Like every MissionBridge it never calls a Service method that takes a flow lock
// (Publish, SetEnabled, DeleteFlow, DeleteFlowForMission, MissionEnabledChanged): the
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
// flows.MaxStoredOutputBytes of JSON) is scrubbed value by value before it is encoded for
// the history, which keeps 16 KiB of it.
func (b flowMissionBridge) FlowRunStarted(missionID string, rec flows.RunRecord) string {
	mm, err := b.missions()
	if err != nil {
		return ""
	}
	data, _ := json.Marshal(scrubFlowMap(rec.TriggerData))
	id := mm.FlowRunStarted(missionID, rec.TriggerType, string(data))
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
	info.Outputs = scrubFlowMap(info.Outputs)
	result, output := flowRunOutcome(info)
	// The success text comes from the scrubbed outputs; this pass covers the run's error
	// text, which the engine caps at 1000 runes, so the cut in flowRunOutcome never reaches it.
	output = security.Scrub(output)
	mm.FlowRunFinished(info.MissionID, info.HistoryID, result, output, info.Outputs)
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
// A mission binding must name a trigger type Mission Control starts flows for
// (flowMissionTriggerTypes), and its Config must decode into tools.TriggerConfig without
// unknown keys (flowTriggerConfig).
func flowTriggerSpecs(bindings []flows.TriggerBinding) ([]tools.FlowTriggerSpec, error) {
	specs := make([]tools.FlowTriggerSpec, 0, len(bindings))
	for _, b := range bindings {
		node := flowBoundRunes(b.NodeID, flowNameEchoRunes)
		spec := tools.FlowTriggerSpec{NodeID: b.NodeID}
		switch b.Kind {
		case flows.BindingMission:
			trigger := tools.TriggerType(b.MissionTrigger)
			if !flowMissionTriggerTypes[trigger] {
				return nil, fmt.Errorf("trigger %s waits for the event %s, which Mission Control cannot start flows for",
					node, flowQuoteName(b.MissionTrigger))
			}
			cfg, err := flowTriggerConfig(b.Config)
			if err != nil {
				return nil, fmt.Errorf("trigger %s: %w", node, err)
			}
			spec.TriggerType, spec.TriggerConfig = trigger, cfg
		case flows.BindingCron:
			spec.TriggerType, spec.Schedule = tools.FlowTriggerSchedule, b.Schedule
		case flows.BindingTimer:
			spec.TriggerType = tools.FlowTriggerDateTime
		case flows.BindingManual:
			spec.TriggerType = tools.FlowTriggerManual
		default:
			return nil, fmt.Errorf("trigger %s has an unknown binding %s", node, flowQuoteName(string(b.Kind)))
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// flowTriggerConfig decodes the Config of a mission binding into Mission Control's
// TriggerConfig. Every key must be the exact JSON name of a TriggerConfig field: a key
// Mission Control does not know would otherwise be dropped silently, and with it a filter
// the flow's author set. The first unknown key in sorted order is reported, quoted and
// bounded; encoding/json's own check (DisallowUnknownFields) stays on as well, and a value
// of the wrong type names its setting.
func flowTriggerConfig(raw map[string]any) (*tools.TriggerConfig, error) {
	cfg := &tools.TriggerConfig{}
	if len(raw) == 0 {
		return cfg, nil
	}
	known := flowTriggerConfigFields()
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !known[k] {
			return nil, fmt.Errorf("unknown setting %s", flowQuoteName(k))
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("the settings cannot be encoded: %s", flowBoundRunes(err.Error(), flowErrorRunes))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return nil, fmt.Errorf("the setting %s has the wrong type", flowQuoteName(typeErr.Field))
		}
		return nil, fmt.Errorf("invalid settings: %s", flowBoundRunes(err.Error(), flowErrorRunes))
	}
	return cfg, nil
}

// flowTriggerConfigFields returns the JSON names of the fields of tools.TriggerConfig.
var flowTriggerConfigFields = sync.OnceValue(func() map[string]bool {
	t := reflect.TypeFor[tools.TriggerConfig]()
	out := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out[name] = true
		}
	}
	return out
})

// flowRunOutcome maps a finished run to a Mission Control result and output text. The text
// is at most flowMissionOutputMaxBytes long (flowCutText). info.Outputs is encoded once, as
// given: FlowRunFinished scrubs it first (scrubFlowMap).
func flowRunOutcome(info flows.RunFinishedInfo) (string, string) {
	result, text := flowRunText(info)
	return result, flowCutText(text, flowMissionOutputMaxBytes)
}

func flowRunText(info flows.RunFinishedInfo) (string, string) {
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

// flowCutText returns s when it has at most limit bytes, else its longest prefix that
// leaves room for flowCutMarker without splitting a UTF-8 sequence, followed by the marker.
func flowCutText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit - len(flowCutMarker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + flowCutMarker
}

// scrubFlowMap is scrubFlowValue for a map; a nil map stays nil.
func scrubFlowMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out, _ := scrubFlowValue(m).(map[string]any)
	return out
}

// scrubFlowValue returns a copy of v in which security.Scrub has replaced the registered
// secrets in every string value and every map key, with the structure kept. Use it for any
// flow value (run outputs, trigger data, step data) before it leaves the flow service
// towards Mission Control, the API or a log.
//
// It scrubs the values, not their JSON text: a secret that holds a quote, a backslash or a
// control character appears escaped in JSON, where the scrubber does not find it, and a
// redaction inside the text can break the JSON. v is not modified (run outputs and trigger
// data are shared and read-only). Strings, maps and lists are walked; bools and numbers are
// kept; any other Go type is normalised through JSON first and becomes nil when it cannot
// be encoded. Nesting deeper than flowScrubMaxDepth becomes nil. Strings shorter than
// flowCredentialMinBytes are kept as they are: the scrubber registers no shorter value, and
// its encoded forms are longer still. Keys that scrub to the same text keep one value, the
// one of the first such key in sorted order.
func scrubFlowValue(v any) any { return scrubFlowValueAt(v, 0) }

func scrubFlowValueAt(v any, depth int) any {
	switch x := v.(type) {
	case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return x
	case string:
		return scrubFlowText(x)
	case map[string]any:
		if x == nil {
			return x
		}
		if depth >= flowScrubMaxDepth {
			return nil
		}
		out := make(map[string]any, len(x))
		var renamed []string
		for k, item := range x {
			if scrubFlowText(k) != k {
				renamed = append(renamed, k)
				continue
			}
			out[k] = scrubFlowValueAt(item, depth+1)
		}
		sort.Strings(renamed)
		for _, k := range renamed {
			key := scrubFlowText(k)
			if _, taken := out[key]; !taken {
				out[key] = scrubFlowValueAt(x[k], depth+1)
			}
		}
		return out
	case []any:
		if x == nil {
			return x
		}
		if depth >= flowScrubMaxDepth {
			return nil
		}
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = scrubFlowValueAt(item, depth+1)
		}
		return out
	default:
		data, err := json.Marshal(x)
		if err != nil {
			return nil
		}
		var normalised any
		if json.Unmarshal(data, &normalised) != nil {
			return nil
		}
		return scrubFlowValueAt(normalised, depth)
	}
}

func scrubFlowText(s string) string {
	if len(s) < flowCredentialMinBytes {
		return s
	}
	return security.Scrub(s)
}

// flowDisplayName names a run's flow in the planner issue title and in notifications: the
// flow's name cut to flowNotifyNameRunes runes, or its id when the name is not known (the
// flow and its documents are gone).
func flowDisplayName(info flows.RunFinishedInfo) string {
	name := strings.TrimSpace(info.FlowName)
	if name == "" {
		name = info.Record.FlowID
	}
	return flowBoundRunes(name, flowNotifyNameRunes)
}

// flowIssue records (or resolves) the planner operational issue of a flow, like missions
// do. The fingerprint is per flow; a run without a flow id has none and records nothing.
// Reference is the mission id and may be empty (the planner stores it trimmed, unchecked).
func (s *Server) flowIssue(info flows.RunFinishedInfo, failed bool, message string) {
	if s.PlannerDB == nil || info.Record.FlowID == "" {
		return
	}
	fingerprint := "flow|" + info.Record.FlowID
	if !failed {
		if _, err := planner.ResolveOperationalIssue(s.PlannerDB, fingerprint, "The flow ran successfully.", time.Now()); err != nil {
			s.Logger.Warn("Flow operational issue could not be resolved", "flow", info.Record.FlowID, "error", err)
		}
		return
	}
	issue := planner.OperationalIssue{Source: "flow", Context: info.Record.FlowID, Title: fmt.Sprintf("Flow %s failed", flowDisplayName(info)),
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
