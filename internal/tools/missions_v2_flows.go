package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
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

// flowCronSource is the cron job source of flow schedules. Only the runner that Start
// registers for it runs them; the cron manager never hands them to its agent fallback.
const flowCronSource = "flow"

// flowCronPrompt is the task prompt of flow schedule jobs (the runner ignores it).
const flowCronPrompt = "EasyDrag flow trigger"

// flowNodeIDPattern mirrors flows.ValidNodeID (internal/flows/model.go): "n_" and eight
// base32 characters. It keeps "__" and other separators out of cron job ids and slots.
var flowNodeIDPattern = regexp.MustCompile(`^n_[a-z2-7]{8}$`)

// flowEventQueueSize bounds the Notify* flow runs that wait for the event dispatcher.
const flowEventQueueSize = 256

// flowMaxEventPayloadBytes bounds the untrusted webhook, email and MQTT payloads that reach a
// flow run. Queued runs keep their trigger data in memory, and the flow engine drops trigger
// data over 5 MiB (it holds raw text plus parsed payload, about twice the body).
const flowMaxEventPayloadBytes = 1 << 20

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

// ErrFlowsUnavailable refuses a run of a flow mission while no flow service is wired in
// (flows.enabled is off, or the flow store could not be opened). The mission API answers 503.
var ErrFlowsUnavailable = errors.New("flows are not available")

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
// Every spec needs a distinct node id in the flow format. The new state is saved even when a
// trigger registration fails; the returned error then reports the registration problem.
func (m *MissionManagerV2) SyncFlowMission(missionID, name string, specs []FlowTriggerSpec) error {
	if err := requireMissionMutationPermission(); err != nil {
		return err
	}
	if err := validateFlowTriggerSpecs(specs); err != nil {
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

// SetFlowMissionEnabled switches the Mission Control triggers of a flow on or off. The new
// state is saved even when a trigger registration fails; the returned error then reports
// the registration problem.
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
		return ErrMissionLocked
	}
	m.unregisterFlowTriggersLocked(mission)
	delete(m.missions, missionID)
	delete(m.flowActive, missionID)
	return m.save()
}

// ErrFlowMissionNotFound is returned (wrapped, with the mission id; the text stays "flow
// mission not found: <id>") by SyncFlowMission and SetFlowMissionEnabled when Mission
// Control holds no flow mission of that id. The flow API tells the user to re-create the
// flow, since a publish never recreates a missing mission.
var ErrFlowMissionNotFound = errors.New("flow mission not found")

func (m *MissionManagerV2) flowMissionLocked(missionID string) (*MissionV2, error) {
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) {
		return nil, fmt.Errorf("%w: %s", ErrFlowMissionNotFound, missionID)
	}
	return mission, nil
}

// validateFlowTriggerSpecs requires a distinct node id in the flow format for every spec.
func validateFlowTriggerSpecs(specs []FlowTriggerSpec) error {
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		if !flowNodeIDPattern.MatchString(spec.NodeID) {
			return fmt.Errorf("invalid flow trigger node id %q", cutAtRuneBoundary(spec.NodeID, 40))
		}
		if seen[spec.NodeID] {
			return fmt.Errorf("duplicate flow trigger node id %q", spec.NodeID)
		}
		seen[spec.NodeID] = true
	}
	return nil
}

// syncFlowTriggersLocked re-registers the triggers of a flow mission; a disabled mission
// keeps none. Caller holds m.mu.
func (m *MissionManagerV2) syncFlowTriggersLocked(mission *MissionV2) error {
	m.unregisterFlowTriggersLocked(mission)
	return m.ensureFlowTriggersLocked(mission, false)
}

// ensureFlowTriggersLocked registers the triggers of an enabled flow mission that are not
// registered yet. Current cron jobs and keyed MQTT registrations stay untouched, so the
// manager setters neither rewrite cron jobs nor reset MQTT rate limits. Caller holds m.mu.
//
// restore is true where the triggers of an already published flow are set up again (Start
// and the manager setters through setupTriggersLocked, a failed delete): a schedule is then
// restored like a persisted job that the cron manager loads, without the scheduler
// mutation permission (CronManager.RestoreRuntimeJob). Publishing and switching a flow on
// (restore false) add the schedule with that permission (AddRuntimeJob), as the persisted
// add did before flow schedules became runtime-only.
func (m *MissionManagerV2) ensureFlowTriggersLocked(mission *MissionV2, restore bool) error {
	if !mission.Enabled {
		return nil
	}
	var errs []error
	for _, spec := range mission.FlowTriggers {
		switch spec.TriggerType {
		case FlowTriggerSchedule:
			if !m.flowCronJobCurrentLocked(mission.ID, spec) {
				errs = append(errs, m.addFlowCronLocked(mission.ID, spec, restore))
			}
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

// flowCronJobCurrentLocked reports whether the flow cron job of spec exists with its schedule.
func (m *MissionManagerV2) flowCronJobCurrentLocked(missionID string, spec FlowTriggerSpec) bool {
	if m.cron == nil {
		return false
	}
	jobID := flowCronJobID(missionID, spec.NodeID)
	for _, job := range m.cron.GetJobs() {
		if job.ID == jobID {
			return job.Source == flowCronSource && job.CronExpr == spec.Schedule
		}
	}
	return false
}

// pruneFlowCronJobsLocked removes flow cron jobs that no enabled schedule spec owns any more,
// for example after a crash or while the scheduler refused removals. Start runs it before
// the triggers are set up. Flow schedules are runtime-only cron jobs (addFlowCronLocked);
// it first takes flow jobs that an earlier build persisted out of the crontab store, so an
// older AuraGo never loads them. Caller holds m.mu.
func (m *MissionManagerV2) pruneFlowCronJobsLocked() {
	if m.cron == nil {
		return
	}
	if moved, err := m.cron.MakeRuntimeOnly(flowCronSource); err != nil {
		slog.Warn("[MissionV2] Failed to remove persisted flow cron jobs from the cron store", "jobs", moved, "error", err)
	} else if moved > 0 {
		slog.Info("[MissionV2] Removed persisted flow cron jobs from the cron store", "jobs", moved)
	}
	for _, job := range m.cron.GetJobs() {
		if job.Source != flowCronSource {
			continue
		}
		missionID, nodeID, ok := splitFlowCronJobID(job.ID)
		if ok {
			mission := m.missions[missionID]
			if isFlowMission(mission) && mission.Enabled {
				if spec, found := flowSpec(mission, nodeID); found && spec.TriggerType == FlowTriggerSchedule && spec.Schedule == job.CronExpr {
					continue
				}
			}
		}
		_, _ = m.cron.ManageSchedule("remove", job.ID, "", "", "")
	}
}

// IsFlowJob reports whether the job is a flow's schedule job. EasyDrag owns those.
func (j CronJob) IsFlowJob() bool { return j.Source == flowCronSource }

// OwnsFlowCronJob reports whether jobID ("mission_<mission>__<node>") names a schedule job of an
// existing flow mission. It is true while the job itself is absent, so nobody can plant a job
// under a flow's id before the flow registers it. The agent's cron tools use it to leave
// flow schedules to EasyDrag.
func (m *MissionManagerV2) OwnsFlowCronJob(jobID string) bool {
	missionID, _, ok := splitFlowCronJobID(jobID)
	if !ok {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return isFlowMission(m.missions[missionID])
}

// IsFlowCronJob reports whether an existing cron job under jobID is a flow's schedule job
// (the flow source). Editing, toggling and deleting such a job is EasyDrag's business.
// crons may be nil.
func IsFlowCronJob(crons *CronManager, jobID string) bool {
	if crons == nil || jobID == "" {
		return false
	}
	for _, job := range crons.GetJobs() {
		if job.ID == jobID {
			return job.IsFlowJob()
		}
	}
	return false
}

// FlowOwnsCronJob reports whether EasyDrag owns the cron job id for any change, an add
// included: a flow mission claims the id (OwnsFlowCronJob, also while the job is absent,
// so none can be planted under it) or an existing job under it is a flow job
// (IsFlowCronJob). missions and crons may be nil.
func FlowOwnsCronJob(missions *MissionManagerV2, crons *CronManager, jobID string) bool {
	if missions != nil && missions.OwnsFlowCronJob(jobID) {
		return true
	}
	return IsFlowCronJob(crons, jobID)
}

// runFlowCronJob is the cron runner of flowCronSource jobs.
func (m *MissionManagerV2) runFlowCronJob(jobID, _ string) {
	missionID, nodeID, ok := splitFlowCronJobID(jobID)
	if !ok || !m.fireFlowSchedule(missionID, nodeID) {
		slog.Debug("[MissionV2] Flow cron job has no flow mission", "job_id", jobID)
	}
}

// unregisterFlowTriggersLocked removes the cron jobs, the webhook registrations and the keyed
// MQTT registrations of a flow mission. Email and plain MQTT registrations cannot be removed:
// they stay in flowRegistered (so re-registering never duplicates them) and their callbacks
// re-check the spec. Only flowCronSource jobs are removed, so a prompt mission whose id is
// "<flow>__<x>" keeps its own cron job.
func (m *MissionManagerV2) unregisterFlowTriggersLocked(mission *MissionV2) {
	if mission == nil {
		return
	}
	if m.cron != nil {
		prefix := "mission_" + mission.ID + flowCronSeparator
		for _, job := range m.cron.GetJobs() {
			if job.Source == flowCronSource && strings.HasPrefix(job.ID, prefix) {
				_, _ = m.cron.ManageSchedule("remove", job.ID, "", "", "")
			}
		}
	}
	m.unregisterFlowWebhooksLocked(mission)
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

// unregisterFlowWebhooksLocked removes the keyed webhook registrations (flowSlot keys) of a
// flow mission. Stop calls it too, as it does for prompt missions' webhooks.
func (m *MissionManagerV2) unregisterFlowWebhooksLocked(mission *MissionV2) {
	if mission == nil {
		return
	}
	prefix := mission.ID + "|flow|"
	suffix := "|" + string(TriggerWebhook)
	for slot := range m.registeredTriggers {
		if strings.HasPrefix(slot, prefix) && strings.HasSuffix(slot, suffix) {
			if m.webhookMgr != nil {
				m.webhookMgr.UnregisterMissionTrigger(slot)
			}
			delete(m.registeredTriggers, slot)
		}
	}
}

// addFlowCronLocked registers the schedule of spec as a runtime-only cron job; restore picks
// the path (see ensureFlowTriggersLocked).
func (m *MissionManagerV2) addFlowCronLocked(missionID string, spec FlowTriggerSpec, restore bool) error {
	if strings.TrimSpace(spec.Schedule) == "" {
		return nil
	}
	if m.cron == nil {
		return fmt.Errorf("cron manager is not configured")
	}
	// The cron store keys jobs by id alone and an add replaces any job with that id, so a
	// job of another source (a prompt mission "<flow>__<node>") is never overwritten.
	jobID := flowCronJobID(missionID, spec.NodeID)
	for _, job := range m.cron.GetJobs() {
		if job.ID == jobID && job.Source != flowCronSource {
			return fmt.Errorf("register flow schedule %s: cron job %s belongs to another scheduler entry", spec.NodeID, jobID)
		}
	}
	// A runtime-only job: the crontab store never holds it, so an older AuraGo (which hands
	// every job it loads to its agent) never runs it; Start registers it again from
	// FlowTriggers (setupTriggersLocked).
	if restore {
		if err := m.cron.RestoreRuntimeJob(jobID, spec.Schedule, flowCronPrompt, flowCronSource); err != nil {
			return fmt.Errorf("restore flow schedule %s: %w", spec.NodeID, err)
		}
		return nil
	}
	out, err := m.cron.AddRuntimeJob(jobID, spec.Schedule, flowCronPrompt, flowCronSource)
	if err != nil {
		return fmt.Errorf("register flow schedule %s: %w", spec.NodeID, err)
	}
	var answer struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(out), &answer) != nil {
		return fmt.Errorf("register flow schedule %s: %s", spec.NodeID, out)
	}
	if answer.Status == "error" {
		return fmt.Errorf("register flow schedule %s: %s", spec.NodeID, answer.Message)
	}
	return nil
}

// markFlowRegistrationLocked tracks the current key of a keyed webhook or MQTT registration,
// which unregisterFlowTriggersLocked removes and every sync makes again.
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

// markPermanentFlowRegistrationLocked reports whether an email or plain MQTT
// registration still has to be made. Those stay for the process lifetime and their callbacks
// re-check the current spec, so a slot is registered once per key: after a change and a
// change back, the old callback serves again instead of a second one firing twice.
func (m *MissionManagerV2) markPermanentFlowRegistrationLocked(slot, key string) bool {
	if m.flowRegistered == nil {
		m.flowRegistered = make(map[string]bool)
	}
	id := slot + "\x00" + key
	if m.flowRegistered[id] {
		return false
	}
	m.flowRegistered[id] = true
	return true
}

func (m *MissionManagerV2) registerFlowWebhookLocked(missionID string, spec FlowTriggerSpec) {
	if m.webhookMgr == nil || spec.TriggerConfig == nil || spec.TriggerConfig.WebhookID == "" {
		return
	}
	nodeID, webhookID := spec.NodeID, spec.TriggerConfig.WebhookID
	// Keyed by the slot: a new registration replaces the slot's old one, and
	// unregisterFlowTriggersLocked and Stop remove it.
	slot := flowSlot(missionID, nodeID, TriggerWebhook)
	if !m.markFlowRegistrationLocked(slot, "webhook|"+webhookID) {
		return
	}
	match := func(c *TriggerConfig) bool {
		return c.WebhookID == webhookID
	}
	m.webhookMgr.RegisterMissionTriggerForKey(slot, webhookID, func(payload []byte) {
		if len(payload) > flowMaxEventPayloadBytes {
			m.dropOversizedFlowEvent(missionID, nodeID, TriggerWebhook, len(payload), match)
			return
		}
		m.fireFlowEvent(missionID, nodeID, TriggerWebhook, "webhook", string(payload), match)
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
	// %q keeps filters that contain "|" apart.
	if !m.markPermanentFlowRegistrationLocked(flowSlot(missionID, nodeID, TriggerEmailReceived), fmt.Sprintf("email|%q|%q|%q", folder, subject, from)) {
		return
	}
	m.emailWatcher.RegisterMissionTrigger(folder, subject, from, func(subj, sender, body string) {
		data, _ := json.Marshal(flowEmailEventData(subj, sender, body))
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
	keyed, isKeyed := m.mqttMgr.(KeyedMQTTManagerInterface)
	mark := m.markPermanentFlowRegistrationLocked
	if isKeyed {
		mark = m.markFlowRegistrationLocked
	}
	if !mark(slot, fmt.Sprintf("mqtt|%q|%q|%d", topicFilter, contains, interval)) {
		return
	}
	match := func(c *TriggerConfig) bool {
		current := c.MQTTMinIntervalSeconds
		if current <= 0 {
			current = c.MinIntervalSeconds
		}
		return c.MQTTTopic == topicFilter && c.MQTTPayloadContains == contains && current == interval
	}
	callback := func(topic, payload string) {
		if len(payload) > flowMaxEventPayloadBytes {
			m.dropOversizedFlowEvent(missionID, nodeID, TriggerMQTTMessage, len(payload), match)
			return
		}
		data, _ := json.Marshal(map[string]string{"topic": topic, "payload": payload})
		m.fireFlowEvent(missionID, nodeID, TriggerMQTTMessage, "mqtt", string(data), match)
	}
	if isKeyed {
		keyed.RegisterMissionTriggerForKey(slot, topicFilter, contains, interval, callback)
		return
	}
	m.mqttMgr.RegisterMissionTrigger(topicFilter, contains, interval, callback)
}

// flowEmailEventData builds the trigger data of a received email. A body over
// flowMaxEventPayloadBytes is cut at a rune boundary and marked "truncated".
func flowEmailEventData(subject, from, body string) map[string]any {
	data := map[string]any{"subject": subject, "from": from, "body": body}
	if len(body) > flowMaxEventPayloadBytes {
		data["body"] = cutAtRuneBoundary(body, flowMaxEventPayloadBytes)
		data["truncated"] = true
	}
	return data
}

// cutAtRuneBoundary returns the longest prefix of s with at most limit bytes that does not
// split a UTF-8 sequence. It steps back at most utf8.UTFMax-1 bytes, so invalid input is
// cut at limit-3 or later instead of being scanned.
func cutAtRuneBoundary(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for i := 1; i < utf8.UTFMax && cut > 0 && !utf8.RuneStart(s[cut]); i++ {
		cut--
	}
	return s[:cut]
}

// CutAtRuneBoundary is cutAtRuneBoundary for other packages (the flow bridge in
// internal/server).
func CutAtRuneBoundary(s string, limit int) string { return cutAtRuneBoundary(s, limit) }

// flowEventSpecLocked returns the spec of an enabled flow mission's node while it still has
// the trigger type and matches. Caller holds m.mu.
func (m *MissionManagerV2) flowEventSpecLocked(missionID, nodeID string, trigger TriggerType, match func(*TriggerConfig) bool) (FlowTriggerSpec, bool) {
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) || !mission.Enabled {
		return FlowTriggerSpec{}, false
	}
	spec, found := flowSpec(mission, nodeID)
	if !found || spec.TriggerType != trigger {
		return FlowTriggerSpec{}, false
	}
	cfg := spec.TriggerConfig
	if cfg == nil {
		cfg = &TriggerConfig{}
	}
	if match != nil && !match(cfg) {
		return FlowTriggerSpec{}, false
	}
	return spec, true
}

// fireFlowEvent starts a run when a registered event still matches the current spec of an
// enabled flow mission. Registrations call it without holding m.mu. After Stop it starts
// nothing.
func (m *MissionManagerV2) fireFlowEvent(missionID, nodeID string, trigger TriggerType, triggerType, data string, match func(*TriggerConfig) bool) {
	if m.ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	spec, ok := m.flowEventSpecLocked(missionID, nodeID, trigger, match)
	if !ok || !m.shouldFireFlowSpecLocked(missionID, spec, time.Now()) {
		m.mu.Unlock()
		return
	}
	hooks := m.flowHooks
	m.mu.Unlock()
	m.startFlowRun(hooks, missionID, nodeID, triggerType, data)
}

// dropOversizedFlowEvent drops an event payload over flowMaxEventPayloadBytes without
// starting a run. It warns only while the registration still targets an enabled flow node,
// so stale registrations stay quiet, and it never logs the payload.
func (m *MissionManagerV2) dropOversizedFlowEvent(missionID, nodeID string, trigger TriggerType, size int, match func(*TriggerConfig) bool) {
	m.mu.RLock()
	_, current := m.flowEventSpecLocked(missionID, nodeID, trigger, match)
	m.mu.RUnlock()
	if !current {
		return
	}
	slog.Warn("[MissionV2] Flow trigger payload too large; no run started", "mission_id", missionID, "node", nodeID,
		"trigger", string(trigger), "size_bytes", size, "limit_bytes", flowMaxEventPayloadBytes)
}

// fireFlowSchedule handles a flow cron job. It reports false when missionID is not a flow
// mission. After Stop it starts nothing (and reports true: the job is not stale).
func (m *MissionManagerV2) fireFlowSchedule(missionID, nodeID string) bool {
	if m.ctx.Err() != nil {
		return true
	}
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

// startFlowRun starts a flow run through hooks. Callers do not hold m.mu.
func (m *MissionManagerV2) startFlowRun(hooks FlowHooks, missionID, nodeID, triggerType, data string) {
	if hooks == nil {
		m.mu.Lock()
		changed := m.noteFlowsUnavailableLocked(missionID)
		if changed {
			m.saveFlowNotesLocked()
		}
		m.mu.Unlock()
		logFlowsUnavailable(changed, "mission_id", missionID, "node", nodeID)
		return
	}
	if err := hooks.StartFlowRun(missionID, nodeID, triggerType, data); err != nil {
		slog.Warn("[MissionV2] Flow trigger could not start a run", "mission_id", missionID, "node", nodeID,
			"trigger", triggerType, "error", err)
	}
}

// flowsUnavailableOutput is the LastOutput of a flow mission whose trigger fired while no
// flow service is wired in: flows.enabled is off, or the flow store could not be opened.
const flowsUnavailableOutput = "EasyDrag flows are not available (switched off, or their store could not be opened); the flow did not run"

// noteFlowsUnavailableLocked shows on a flow mission that a trigger fired while flows are
// not available: LastResult "error" with flowsUnavailableOutput. Nothing else changes (no
// run is counted and no dependent fires). It reports whether the mission changed, that is
// whether it did not show that already; it does not save, so a caller noting several
// missions saves once (saveFlowNotesLocked) and a trigger that keeps firing does not
// rewrite the missions file. Caller holds m.mu for writing.
func (m *MissionManagerV2) noteFlowsUnavailableLocked(missionID string) bool {
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) ||
		(mission.LastResult == MissionResultError && mission.LastOutput == flowsUnavailableOutput) {
		return false
	}
	mission.LastResult, mission.LastOutput = MissionResultError, flowsUnavailableOutput
	return true
}

// saveFlowNotesLocked persists the notes of noteFlowsUnavailableLocked. Caller holds m.mu.
func (m *MissionManagerV2) saveFlowNotesLocked() {
	if err := m.save(); err != nil {
		slog.Warn("[MissionV2] Failed to persist the flow mission state", "error", err)
	}
}

// logFlowsUnavailable logs a flow trigger refused for lack of flow hooks: at Warn when it
// changed a mission (the first refusal shows up), at Debug when it repeats.
func logFlowsUnavailable(changed bool, args ...any) {
	const msg = "[MissionV2] Flow trigger fired but flows are not available"
	if changed {
		slog.Warn(msg, args...)
		return
	}
	slog.Debug(msg, args...)
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

// flowRunRequest is one flow run that a Notify* event asked for.
type flowRunRequest struct {
	hooks                                FlowHooks
	missionID, nodeID, triggerType, data string
	// ownerCtx is the Desktop owner of the finished run whose mission_completed event asked
	// for this run (QueueOwnedMission), nil otherwise. A revoked owner starts no run.
	ownerCtx context.Context
	// releaseOwner gives back the owner reference the request holds (nil without an owner).
	releaseOwner context.CancelFunc
}

// release gives back the request's owner reference, if it holds one.
func (r flowRunRequest) release() {
	if r.releaseOwner != nil {
		r.releaseOwner()
	}
}

// notifyFlowsLocked starts the runs of enabled flows whose specs match an event. Caller holds
// m.mu, so the runs go to the event dispatcher (the hooks call back into the manager). The
// dispatcher keeps event order; when its queue is full, the runs of the event are dropped.
// Without flow hooks no run starts and every matching flow mission shows why
// (noteFlowsUnavailableLocked).
func (m *MissionManagerV2) notifyFlowsLocked(trigger TriggerType, ev flowEvent, data any) {
	m.notifyFlowsForOwnerLocked(trigger, ev, data, QueueItem{})
}

// notifyFlowsForOwnerLocked is notifyFlowsLocked for the completion of a run that may hold a
// Desktop owner (owner.RequiresOwner): each queued run retains the owner until the dispatcher
// has started it, and the dispatcher starts none whose owner was revoked meanwhile. Without a
// way to retain the owner no run is queued, as for prompt dependents. Caller holds m.mu.
func (m *MissionManagerV2) notifyFlowsForOwnerLocked(trigger TriggerType, ev flowEvent, data any, owner QueueItem) {
	if owner.RequiresOwner && owner.retainOwner == nil {
		return
	}
	if m.ctx.Err() != nil {
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
	if m.flowHooks == nil {
		changed := false
		for _, s := range starts {
			changed = m.noteFlowsUnavailableLocked(s.missionID) || changed
		}
		if changed {
			m.saveFlowNotesLocked() // once per event, however many flows it refused
		}
		logFlowsUnavailable(changed, "trigger", string(trigger), "runs", len(starts))
		return
	}
	raw := ""
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			raw = string(b)
		}
	}
	if m.flowEvents == nil {
		events := make(chan flowRunRequest, flowEventQueueSize)
		if !m.runAsync(func() { m.dispatchFlowEvents(events) }) {
			return // Stop has begun; no run starts after it.
		}
		m.flowEvents = events
	}
	dropped := 0
	for _, s := range starts {
		req := flowRunRequest{hooks: m.flowHooks, missionID: s.missionID, nodeID: s.nodeID, triggerType: string(trigger), data: raw}
		if owner.RequiresOwner {
			req.ownerCtx, req.releaseOwner = owner.ownerContext, owner.retainOwner()
		}
		select {
		case m.flowEvents <- req:
		default:
			req.release()
			dropped++
		}
	}
	if dropped > 0 {
		slog.Warn("[MissionV2] Flow event queue is full; flow runs dropped", "trigger", string(trigger),
			"dropped", dropped, "capacity", flowEventQueueSize)
	}
}

// dispatchFlowEvents starts the flow runs of Notify* events one at a time, in event order.
// notifyFlowsLocked starts it with the first matching event; it ends when Stop cancels the
// manager context, and starts no queued run after that. A run whose Desktop owner was revoked
// while it waited is skipped (logged at Debug). Every request gives back its owner reference.
func (m *MissionManagerV2) dispatchFlowEvents(events <-chan flowRunRequest) {
	defer drainFlowRunRequests(events)
	for {
		select {
		case <-m.ctx.Done():
			return
		case req := <-events:
			// select picks at random when both are ready.
			if m.ctx.Err() != nil {
				req.release()
				return
			}
			if req.ownerCtx != nil && req.ownerCtx.Err() != nil {
				slog.Debug("[MissionV2] Flow run skipped: the Desktop owner of the finished mission was revoked",
					"mission_id", req.missionID, "node", req.nodeID)
				req.release()
				continue
			}
			m.startFlowRun(req.hooks, req.missionID, req.nodeID, req.triggerType, req.data)
			req.release()
		}
	}
}

// drainFlowRunRequests gives back the owner references of the runs still queued when the
// dispatcher ends; none of them starts.
func drainFlowRunRequests(events <-chan flowRunRequest) {
	for {
		select {
		case req := <-events:
			req.release()
		default:
			return
		}
	}
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
