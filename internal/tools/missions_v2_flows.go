package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ExecutionFlow marks missions that represent an EasyDrag flow. The flow service owns them:
// they never enter the agent queue and their triggers live in FlowTriggers.
const ExecutionFlow ExecutionType = "flow"

// Flow trigger types that Mission Control schedules or only displays. Event triggers reuse
// the TriggerType constants of prompt missions.
const (
	FlowTriggerSchedule TriggerType = "schedule"
	FlowTriggerDateTime TriggerType = "datetime"
	FlowTriggerManual   TriggerType = "manual"
)

// flowCronSeparator separates mission and node in flow cron job ids: "mission_<mission>__<node>".
const flowCronSeparator = "__"

// FlowTriggerSpec is one enabled trigger node of a published flow.
type FlowTriggerSpec struct {
	NodeID        string         `json:"node_id"`
	TriggerType   TriggerType    `json:"trigger_type"`
	TriggerConfig *TriggerConfig `json:"trigger_config,omitempty"`
	Schedule      string         `json:"schedule,omitempty"`
}

// FlowHooks connects Mission Control with the flow service (implemented in internal/server).
// The manager never calls a hook while it holds its lock.
type FlowHooks interface {
	// StartFlowRun starts a live run of the flow behind missionID. An empty nodeID picks the
	// flow's manual trigger or its first trigger.
	StartFlowRun(missionID, nodeID, triggerType, triggerData string) error
	// FlowMissionDeleted tells the flow service that Mission Control deleted the mission.
	FlowMissionDeleted(missionID string)
	// FlowEnabledChanged tells the flow service that Mission Control switched the mission.
	FlowEnabledChanged(missionID string, enabled bool)
	// NextFlowRun returns the next Date/Time trigger of the flow.
	NextFlowRun(missionID string) (time.Time, bool)
}

// ErrFlowMissionManaged rejects changes to flow missions outside EasyDrag. The text contains
// "not supported" so the mission API answers 400.
var ErrFlowMissionManaged = errors.New("flow missions are managed in EasyDrag; changing them here is not supported")

func isFlowMission(m *MissionV2) bool { return m != nil && m.ExecutionType == ExecutionFlow }

func copyFlowTriggers(in []FlowTriggerSpec) []FlowTriggerSpec {
	if in == nil {
		return nil
	}
	out := make([]FlowTriggerSpec, len(in))
	for i, spec := range in {
		out[i] = spec
		if spec.TriggerConfig != nil {
			tc := *spec.TriggerConfig
			out[i].TriggerConfig = &tc
		}
	}
	return out
}

func flowSpec(m *MissionV2, nodeID string) (FlowTriggerSpec, bool) {
	for _, spec := range m.FlowTriggers {
		if spec.NodeID == nodeID {
			return spec, true
		}
	}
	return FlowTriggerSpec{}, false
}

func flowCronJobID(missionID, nodeID string) string {
	return "mission_" + missionID + flowCronSeparator + nodeID
}

// splitFlowCronJobID parses "mission_<mission>__<node>".
func splitFlowCronJobID(jobID string) (missionID, nodeID string, ok bool) {
	rest, found := strings.CutPrefix(jobID, "mission_")
	if !found {
		return "", "", false
	}
	i := strings.LastIndex(rest, flowCronSeparator)
	if i <= 0 || i+len(flowCronSeparator) >= len(rest) {
		return "", "", false
	}
	return rest[:i], rest[i+len(flowCronSeparator):], true
}

// flowSlot names the registration of one flow trigger node (also the keyed MQTT key).
func flowSlot(missionID, nodeID string, trigger TriggerType) string {
	return missionID + "|flow|" + nodeID + "|" + string(trigger)
}

// SetFlowHooks installs the flow service. Call it before Start so that startup triggers
// and registrations find it.
func (m *MissionManagerV2) SetFlowHooks(h FlowHooks) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flowHooks = h
}

func (m *MissionManagerV2) newMissionIDLocked() string {
	base := time.Now().UnixNano()
	for i := int64(0); ; i++ {
		id := fmt.Sprintf("mission_%d", base+i)
		if _, exists := m.missions[id]; !exists {
			return id
		}
	}
}

// CreateFlowMission creates the disabled, local mission that represents a flow.
func (m *MissionManagerV2) CreateFlowMission(flowID, name string) (string, error) {
	if err := requireMissionMutationPermission(); err != nil {
		return "", err
	}
	if strings.TrimSpace(flowID) == "" {
		return "", fmt.Errorf("flow id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mission := &MissionV2{
		ID: m.newMissionIDLocked(), Name: name, ExecutionType: ExecutionFlow, FlowID: flowID,
		Priority: "medium", Status: MissionStatusIdle, CreatedAt: time.Now(), RunnerType: MissionRunnerLocal,
	}
	m.missions[mission.ID] = mission
	if err := m.save(); err != nil {
		delete(m.missions, mission.ID)
		return "", err
	}
	return mission.ID, nil
}

// SyncFlowMission stores the name and trigger specs of a published flow and registers them.
func (m *MissionManagerV2) SyncFlowMission(missionID, name string, specs []FlowTriggerSpec) error {
	if err := requireMissionMutationPermission(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, err := m.flowMissionLocked(missionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) != "" {
		mission.Name = name
	}
	mission.FlowTriggers = copyFlowTriggers(specs)
	mission.FlowPublished = true
	regErr := m.syncFlowTriggersLocked(mission)
	return errors.Join(regErr, m.save())
}

// SetFlowMissionEnabled switches the Mission Control triggers of a flow on or off.
func (m *MissionManagerV2) SetFlowMissionEnabled(missionID string, enabled bool) error {
	if err := requireMissionMutationPermission(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, err := m.flowMissionLocked(missionID)
	if err != nil {
		return err
	}
	mission.Enabled = enabled
	regErr := m.syncFlowTriggersLocked(mission)
	return errors.Join(regErr, m.save())
}

// DeleteFlowMission removes a flow mission for the flow service. Unlike Delete it does not
// call FlowHooks.FlowMissionDeleted. A missing mission is not an error.
func (m *MissionManagerV2) DeleteFlowMission(missionID string) error {
	if err := requireMissionMutationPermission(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mission, ok := m.missions[missionID]
	if !ok {
		return nil
	}
	if !isFlowMission(mission) {
		return fmt.Errorf("mission %s is not a flow mission", missionID)
	}
	if mission.Locked {
		return fmt.Errorf("mission is locked")
	}
	m.unregisterFlowTriggersLocked(mission)
	delete(m.missions, missionID)
	delete(m.flowActive, missionID)
	return m.save()
}

func (m *MissionManagerV2) flowMissionLocked(missionID string) (*MissionV2, error) {
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		return nil, fmt.Errorf("flow mission not found: %s", missionID)
	}
	return mission, nil
}

// syncFlowTriggersLocked re-registers the triggers of a flow mission; a disabled mission
// keeps none. Caller holds m.mu.
func (m *MissionManagerV2) syncFlowTriggersLocked(mission *MissionV2) error {
	m.unregisterFlowTriggersLocked(mission)
	if !mission.Enabled {
		return nil
	}
	var errs []error
	for _, spec := range mission.FlowTriggers {
		switch spec.TriggerType {
		case FlowTriggerSchedule:
			errs = append(errs, m.addFlowCronLocked(mission.ID, spec))
		case TriggerWebhook:
			m.registerFlowWebhookLocked(mission.ID, spec)
		case TriggerEmailReceived:
			m.registerFlowEmailLocked(mission.ID, spec)
		case TriggerMQTTMessage:
			m.registerFlowMQTTLocked(mission.ID, spec)
		}
	}
	return errors.Join(errs...)
}

// unregisterFlowTriggersLocked removes the cron jobs and keyed MQTT registrations of a flow
// mission. Webhook and email registrations cannot be removed: their slots stay (so
// re-registering never duplicates them) and their callbacks re-check the spec.
func (m *MissionManagerV2) unregisterFlowTriggersLocked(mission *MissionV2) {
	if mission == nil {
		return
	}
	if m.cron != nil {
		prefix := "mission_" + mission.ID + flowCronSeparator
		for _, job := range m.cron.GetJobs() {
			if strings.HasPrefix(job.ID, prefix) {
				_, _ = m.cron.ManageSchedule("remove", job.ID, "", "", "")
			}
		}
	}
	keyed, ok := m.mqttMgr.(KeyedMQTTManagerInterface)
	if !ok {
		return
	}
	prefix := mission.ID + "|flow|"
	suffix := "|" + string(TriggerMQTTMessage)
	for slot := range m.registeredTriggers {
		if strings.HasPrefix(slot, prefix) && strings.HasSuffix(slot, suffix) {
			keyed.UnregisterMissionTrigger(slot)
			delete(m.registeredTriggers, slot)
		}
	}
}

func (m *MissionManagerV2) addFlowCronLocked(missionID string, spec FlowTriggerSpec) error {
	if strings.TrimSpace(spec.Schedule) == "" {
		return nil
	}
	if m.cron == nil {
		return fmt.Errorf("cron manager is not configured")
	}
	out, err := m.cron.ManageScheduleWithSource("add", flowCronJobID(missionID, spec.NodeID), spec.Schedule, "EasyDrag flow trigger", "", "mission")
	if err != nil {
		return fmt.Errorf("register flow schedule %s: %w", spec.NodeID, err)
	}
	if strings.Contains(out, `"status": "error"`) {
		return fmt.Errorf("register flow schedule %s: %s", spec.NodeID, out)
	}
	return nil
}

func (m *MissionManagerV2) markFlowRegistrationLocked(slot, key string) bool {
	if m.registeredTriggers == nil {
		m.registeredTriggers = make(map[string]string)
	}
	if m.registeredTriggers[slot] == key {
		return false
	}
	m.registeredTriggers[slot] = key
	return true
}

func (m *MissionManagerV2) registerFlowWebhookLocked(missionID string, spec FlowTriggerSpec) {
	if m.webhookMgr == nil || spec.TriggerConfig == nil || spec.TriggerConfig.WebhookID == "" {
		return
	}
	nodeID, webhookID := spec.NodeID, spec.TriggerConfig.WebhookID
	if !m.markFlowRegistrationLocked(flowSlot(missionID, nodeID, TriggerWebhook), "webhook|"+webhookID) {
		return
	}
	m.webhookMgr.RegisterMissionTrigger(webhookID, func(payload []byte) {
		m.fireFlowEvent(missionID, nodeID, TriggerWebhook, "webhook", string(payload), func(c *TriggerConfig) bool {
			return c.WebhookID == webhookID
		})
	})
}

func (m *MissionManagerV2) registerFlowEmailLocked(missionID string, spec FlowTriggerSpec) {
	if m.emailWatcher == nil {
		return
	}
	cfg := spec.TriggerConfig
	if cfg == nil {
		cfg = &TriggerConfig{}
	}
	nodeID := spec.NodeID
	folder, subject, from := cfg.EmailFolder, cfg.EmailSubjectContains, cfg.EmailFromContains
	if !m.markFlowRegistrationLocked(flowSlot(missionID, nodeID, TriggerEmailReceived), fmt.Sprintf("email|%s|%s|%s", folder, subject, from)) {
		return
	}
	m.emailWatcher.RegisterMissionTrigger(folder, subject, from, func(subj, sender, body string) {
		data, _ := json.Marshal(map[string]string{"subject": subj, "from": sender, "body": body})
		m.fireFlowEvent(missionID, nodeID, TriggerEmailReceived, "email", string(data), func(c *TriggerConfig) bool {
			return c.EmailFolder == folder && c.EmailSubjectContains == subject && c.EmailFromContains == from
		})
	})
}

func (m *MissionManagerV2) registerFlowMQTTLocked(missionID string, spec FlowTriggerSpec) {
	if m.mqttMgr == nil || spec.TriggerConfig == nil || spec.TriggerConfig.MQTTTopic == "" {
		return
	}
	cfg := spec.TriggerConfig
	nodeID := spec.NodeID
	topicFilter, contains := cfg.MQTTTopic, cfg.MQTTPayloadContains
	interval := cfg.MQTTMinIntervalSeconds
	if interval <= 0 {
		interval = cfg.MinIntervalSeconds
	}
	slot := flowSlot(missionID, nodeID, TriggerMQTTMessage)
	if !m.markFlowRegistrationLocked(slot, fmt.Sprintf("mqtt|%s|%s|%d", topicFilter, contains, interval)) {
		return
	}
	callback := func(topic, payload string) {
		data, _ := json.Marshal(map[string]string{"topic": topic, "payload": payload})
		m.fireFlowEvent(missionID, nodeID, TriggerMQTTMessage, "mqtt", string(data), func(c *TriggerConfig) bool {
			current := c.MQTTMinIntervalSeconds
			if current <= 0 {
				current = c.MinIntervalSeconds
			}
			return c.MQTTTopic == topicFilter && c.MQTTPayloadContains == contains && current == interval
		})
	}
	if keyed, ok := m.mqttMgr.(KeyedMQTTManagerInterface); ok {
		keyed.RegisterMissionTriggerForKey(slot, topicFilter, contains, interval, callback)
		return
	}
	m.mqttMgr.RegisterMissionTrigger(topicFilter, contains, interval, callback)
}

// fireFlowEvent starts a run when a registered event still matches the current spec of an
// enabled flow mission. Registrations call it without holding m.mu.
func (m *MissionManagerV2) fireFlowEvent(missionID, nodeID string, trigger TriggerType, triggerType, data string, match func(*TriggerConfig) bool) {
	m.mu.Lock()
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) || !mission.Enabled {
		m.mu.Unlock()
		return
	}
	spec, found := flowSpec(mission, nodeID)
	cfg := spec.TriggerConfig
	if cfg == nil {
		cfg = &TriggerConfig{}
	}
	if !found || spec.TriggerType != trigger || (match != nil && !match(cfg)) || !m.shouldFireFlowSpecLocked(missionID, spec, time.Now()) {
		m.mu.Unlock()
		return
	}
	hooks := m.flowHooks
	m.mu.Unlock()
	m.startFlowRun(hooks, missionID, nodeID, triggerType, data)
}

// fireFlowSchedule handles a flow cron job. It reports false when missionID is not a flow
// mission, so the cron runner treats the job as a prompt mission job.
func (m *MissionManagerV2) fireFlowSchedule(missionID, nodeID string) bool {
	m.mu.Lock()
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		m.mu.Unlock()
		return false
	}
	spec, found := flowSpec(mission, nodeID)
	if !mission.Enabled || !found || spec.TriggerType != FlowTriggerSchedule {
		m.mu.Unlock()
		return true
	}
	hooks := m.flowHooks
	m.mu.Unlock()
	m.startFlowRun(hooks, missionID, nodeID, "cron", "")
	return true
}

func (m *MissionManagerV2) shouldFireFlowSpecLocked(missionID string, spec FlowTriggerSpec, now time.Time) bool {
	interval := triggerMinIntervalSeconds(spec.TriggerConfig)
	if interval <= 0 {
		return true
	}
	if m.lastTriggerFire == nil {
		m.lastTriggerFire = make(map[string]time.Time)
	}
	key := missionID + "|flow|" + spec.NodeID
	if last := m.lastTriggerFire[key]; !last.IsZero() && now.Sub(last) < time.Duration(interval)*time.Second {
		return false
	}
	m.lastTriggerFire[key] = now
	return true
}

func (m *MissionManagerV2) startFlowRun(hooks FlowHooks, missionID, nodeID, triggerType, data string) {
	if hooks == nil {
		slog.Warn("[MissionV2] Flow trigger fired but flows are not available", "mission_id", missionID, "node", nodeID)
		return
	}
	if err := hooks.StartFlowRun(missionID, nodeID, triggerType, data); err != nil {
		slog.Warn("[MissionV2] Flow trigger could not start a run", "mission_id", missionID, "node", nodeID,
			"trigger", triggerType, "error", err)
	}
}

// flowEvent carries the values that event trigger specs filter on.
type flowEvent struct {
	DeviceID, DeviceName string
	CallType             string
	EntityID, NewState   string
	Title                string
	SourceMissionID      string
	Result               string
}

// flowEventMatches mirrors the filters of the Notify* methods for one flow spec.
func flowEventMatches(spec FlowTriggerSpec, trigger TriggerType, ev flowEvent) bool {
	if spec.TriggerType != trigger {
		return false
	}
	cfg := spec.TriggerConfig
	if cfg == nil {
		cfg = &TriggerConfig{}
	}
	switch trigger {
	case TriggerDeviceConnected, TriggerDeviceDisconnected:
		return (cfg.DeviceID == "" || cfg.DeviceID == ev.DeviceID) && (cfg.DeviceName == "" || cfg.DeviceName == ev.DeviceName)
	case TriggerFritzBoxCall:
		return cfg.CallType == "" || cfg.CallType == ev.CallType
	case TriggerHomeAssistantState:
		return (cfg.HAEntityID == "" || cfg.HAEntityID == ev.EntityID) && (cfg.HAStateEquals == "" || cfg.HAStateEquals == ev.NewState)
	case TriggerPlannerAppointmentDue, TriggerPlannerTodoOverdue:
		return cfg.PlannerTitleContains == "" ||
			strings.Contains(strings.ToLower(ev.Title), strings.ToLower(cfg.PlannerTitleContains))
	case TriggerMissionCompleted:
		return cfg.SourceMissionID != "" && cfg.SourceMissionID == ev.SourceMissionID &&
			(!cfg.RequireSuccess || ev.Result == MissionResultSuccess)
	default: // system_startup, budget_warning, budget_exceeded: no filters
		return true
	}
}

// notifyFlowsLocked starts the runs of enabled flows whose specs match an event. Caller holds
// m.mu, so the runs start on a new goroutine (the hooks call back into the manager).
func (m *MissionManagerV2) notifyFlowsLocked(trigger TriggerType, ev flowEvent, data any) {
	if m.flowHooks == nil {
		return
	}
	type start struct{ missionID, nodeID string }
	var starts []start
	now := time.Now()
	for _, mission := range m.missions {
		if !isFlowMission(mission) || !mission.Enabled {
			continue
		}
		for _, spec := range mission.FlowTriggers {
			if flowEventMatches(spec, trigger, ev) && m.shouldFireFlowSpecLocked(mission.ID, spec, now) {
				starts = append(starts, start{mission.ID, spec.NodeID})
			}
		}
	}
	if len(starts) == 0 {
		return
	}
	raw := ""
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			raw = string(b)
		}
	}
	hooks := m.flowHooks
	go func() {
		for _, s := range starts {
			m.startFlowRun(hooks, s.missionID, s.nodeID, string(trigger), raw)
		}
	}()
}

// homeAssistantMonitoredEntities lists the entities that enabled prompt missions and flows watch.
func homeAssistantMonitoredEntities(missions []*MissionV2) map[string]bool {
	out := map[string]bool{}
	for _, mission := range missions {
		if !mission.Enabled {
			continue
		}
		if mission.ExecutionType == ExecutionTriggered && mission.TriggerType == TriggerHomeAssistantState &&
			mission.TriggerConfig != nil && mission.TriggerConfig.HAEntityID != "" {
			out[mission.TriggerConfig.HAEntityID] = true
		}
		if !isFlowMission(mission) {
			continue
		}
		for _, spec := range mission.FlowTriggers {
			if spec.TriggerType == TriggerHomeAssistantState && spec.TriggerConfig != nil && spec.TriggerConfig.HAEntityID != "" {
				out[spec.TriggerConfig.HAEntityID] = true
			}
		}
	}
	return out
}
