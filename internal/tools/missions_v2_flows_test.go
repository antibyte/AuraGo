package tools

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type flowStartCall struct {
	missionID, nodeID, triggerType, data string
}

type fakeFlowHooks struct {
	mu      sync.Mutex
	starts  chan flowStartCall
	deleted []string
	enabled []string
	next    time.Time
	err     error
}

func newFakeFlowHooks() *fakeFlowHooks {
	return &fakeFlowHooks{starts: make(chan flowStartCall, 32)}
}

func (h *fakeFlowHooks) StartFlowRun(missionID, nodeID, triggerType, data string) error {
	h.starts <- flowStartCall{missionID, nodeID, triggerType, data}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *fakeFlowHooks) FlowMissionDeleted(missionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deleted = append(h.deleted, missionID)
}

func (h *fakeFlowHooks) FlowEnabledChanged(missionID string, enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = append(h.enabled, fmt.Sprintf("%s=%v", missionID, enabled))
}

func (h *fakeFlowHooks) NextFlowRun(string) (time.Time, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.next, !h.next.IsZero()
}

func (h *fakeFlowHooks) setErr(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.err = err
}

func (h *fakeFlowHooks) setNext(t time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next = t
}

func (h *fakeFlowHooks) waitStart(t *testing.T) flowStartCall {
	t.Helper()
	select {
	case c := <-h.starts:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("no flow run was started")
	}
	return flowStartCall{}
}

func (h *fakeFlowHooks) expectNoStart(t *testing.T) {
	t.Helper()
	select {
	case c := <-h.starts:
		t.Fatalf("unexpected flow run %+v", c)
	case <-time.After(100 * time.Millisecond):
	}
}

// eventually polls cond for up to two seconds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func newFlowTestManager(t *testing.T) (*MissionManagerV2, *fakeFlowHooks, *fakeWebhookTriggerManager) {
	t.Helper()
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	t.Cleanup(mm.Stop) // ends the flow event dispatcher
	hooks := newFakeFlowHooks()
	webhooks := &fakeWebhookTriggerManager{}
	mm.SetFlowHooks(hooks)
	mm.SetWebhookManager(webhooks)
	return mm, hooks, webhooks
}

// publishTestFlow creates, publishes and enables a flow mission with the given specs.
func publishTestFlow(t *testing.T, mm *MissionManagerV2, specs ...FlowTriggerSpec) string {
	t.Helper()
	id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Morgenbericht")
	if err != nil {
		t.Fatalf("CreateFlowMission: %v", err)
	}
	if err := mm.SyncFlowMission(id, "Morgenbericht", specs); err != nil {
		t.Fatalf("SyncFlowMission: %v", err)
	}
	if err := mm.SetFlowMissionEnabled(id, true); err != nil {
		t.Fatalf("SetFlowMissionEnabled: %v", err)
	}
	return id
}

func TestCreateFlowMissionIsDisabledAndLocal(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatalf("CreateFlowMission: %v", err)
	}
	m, ok := mm.Get(id)
	if !ok || m.ExecutionType != ExecutionFlow || m.Enabled || m.FlowID != "flow_aaaaaaaaaa" ||
		m.Status != MissionStatusIdle || m.RunnerType != MissionRunnerLocal || m.FlowPublished {
		t.Fatalf("flow mission = %+v", m)
	}
	other, _ := mm.CreateFlowMission("flow_bbbbbbbbbb", "Zweiter")
	if other == id {
		t.Fatal("mission ids must be unique")
	}
	if _, err := mm.CreateFlowMission(" ", "x"); err == nil {
		t.Fatal("a flow id is required")
	}
	if err := mm.Create(&MissionV2{Name: "x", Prompt: "y", ExecutionType: ExecutionFlow}); !errors.Is(err, ErrFlowMissionManaged) {
		t.Fatalf("Create(flow) = %v", err)
	}
	if err := mm.SetFlowMissionEnabled("missing", true); err == nil {
		t.Fatal("unknown flow missions must fail")
	}
}

func TestFlowWebhookTriggerStartsRunWithoutQueue(t *testing.T) {
	mm, hooks, webhooks := newFlowTestManager(t)
	spec := FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}}
	id := publishTestFlow(t, mm, spec)

	webhooks.Fire("hook-1", []byte(`{"text":"Hallo"}`))
	if c := hooks.waitStart(t); c != (flowStartCall{id, "n_aaaaaaaa", "webhook", `{"text":"Hallo"}`}) {
		t.Fatalf("start = %+v", c)
	}
	if queue, _ := mm.GetQueue(); len(queue.List()) != 0 {
		t.Fatalf("flow runs must not use the agent queue: %+v", queue.List())
	}
	if err := mm.SyncFlowMission(id, "Morgenbericht", []FlowTriggerSpec{spec}); err != nil {
		t.Fatal(err)
	}
	if n := len(webhooks.callbacks["hook-1"]); n != 1 {
		t.Fatalf("republishing registered the webhook %d times", n)
	}
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.expectNoStart(t)

	if err := mm.SetFlowMissionEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	moved := spec
	moved.TriggerConfig = &TriggerConfig{WebhookID: "hook-2"}
	if err := mm.SyncFlowMission(id, "Morgenbericht", []FlowTriggerSpec{moved}); err != nil {
		t.Fatal(err)
	}
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.expectNoStart(t)
	webhooks.Fire("hook-2", []byte(`{}`))
	if c := hooks.waitStart(t); c.nodeID != "n_aaaaaaaa" {
		t.Fatalf("start after move = %+v", c)
	}
}

func TestFlowEventTriggersMatchFilters(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerDeviceConnected, TriggerConfig: &TriggerConfig{DeviceID: "dev-1"}},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerHomeAssistantState, TriggerConfig: &TriggerConfig{HAEntityID: "binary_sensor.door", HAStateEquals: "on"}},
		FlowTriggerSpec{NodeID: "n_cccccccc", TriggerType: TriggerPlannerAppointmentDue, TriggerConfig: &TriggerConfig{PlannerTitleContains: "zahnarzt"}},
		FlowTriggerSpec{NodeID: "n_dddddddd", TriggerType: TriggerBudgetWarning, TriggerConfig: &TriggerConfig{}},
		FlowTriggerSpec{NodeID: "n_eeeeeeee", TriggerType: TriggerFritzBoxCall, TriggerConfig: &TriggerConfig{CallType: "tam_message"}},
	)
	mm.NotifyDeviceEvent("device_connected", "dev-2", "Laptop")
	hooks.expectNoStart(t)
	mm.NotifyDeviceEvent("device_connected", "dev-1", "Laptop")
	if c := hooks.waitStart(t); c.missionID != id || c.nodeID != "n_aaaaaaaa" || c.triggerType != "device_connected" ||
		!strings.Contains(c.data, `"device_id":"dev-1"`) {
		t.Fatalf("device start = %+v", c)
	}
	mm.NotifyHomeAssistantEvent("binary_sensor.door", "off", "on")
	hooks.expectNoStart(t)
	mm.NotifyHomeAssistantEvent("binary_sensor.door", "on", "off")
	if c := hooks.waitStart(t); c.nodeID != "n_bbbbbbbb" || !strings.Contains(c.data, `"new_state":"on"`) {
		t.Fatalf("HA start = %+v", c)
	}
	mm.NotifyPlannerAppointmentDue("apt-1", "Zahnarzt Dr. Weber", "2026-10-05T09:00:00Z")
	if c := hooks.waitStart(t); c.nodeID != "n_cccccccc" || !strings.Contains(c.data, `"title":"Zahnarzt Dr. Weber"`) {
		t.Fatalf("planner start = %+v", c)
	}
	mm.NotifyBudgetEvent("budget_warning", 4.2, 5, 84)
	if c := hooks.waitStart(t); c.nodeID != "n_dddddddd" || c.triggerType != "budget_warning" {
		t.Fatalf("budget start = %+v", c)
	}
	mm.NotifyFritzBoxEvent("call", "Anruf")
	hooks.expectNoStart(t)
	mm.NotifyFritzBoxEvent("tam_message", "Neue Sprachnachricht")
	if c := hooks.waitStart(t); c.nodeID != "n_eeeeeeee" {
		t.Fatalf("fritzbox start = %+v", c)
	}
	if queue, _ := mm.GetQueue(); len(queue.List()) != 0 {
		t.Fatalf("flow events must not use the agent queue: %+v", queue.List())
	}
}

func TestFlowTriggerMinIntervalAndStartup(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerDeviceDisconnected, TriggerConfig: &TriggerConfig{MinIntervalSeconds: 60}},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerSystemStartup, TriggerConfig: &TriggerConfig{}},
	)
	mm.NotifyDeviceEvent("device_disconnected", "dev-1", "Laptop")
	hooks.waitStart(t)
	mm.NotifyDeviceEvent("device_disconnected", "dev-1", "Laptop")
	hooks.expectNoStart(t)
	mm.NotifySystemStartup()
	if c := hooks.waitStart(t); c.nodeID != "n_bbbbbbbb" || c.triggerType != "system_startup" {
		t.Fatalf("startup = %+v", c)
	}
}

func TestFlowMQTTAndEmailTriggers(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	mqtt := &fakeMQTTTriggerManager{}
	email := &fakeEmailTriggerWatcher{}
	mm.SetMQTTManager(mqtt)
	mm.SetEmailWatcher(email)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerMQTTMessage, TriggerConfig: &TriggerConfig{MQTTTopic: "home/door", MQTTMinIntervalSeconds: 5}},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerEmailReceived, TriggerConfig: &TriggerConfig{EmailFolder: "INBOX", EmailSubjectContains: "Rechnung"}},
	)
	if len(mqtt.keyedRegistrations) != 1 || mqtt.keyedRegistrations[0].key != flowSlot(id, "n_aaaaaaaa", TriggerMQTTMessage) ||
		mqtt.keyedRegistrations[0].minIntervalSeconds != 5 {
		t.Fatalf("mqtt registrations = %+v", mqtt.keyedRegistrations)
	}
	mqtt.keyedRegistrations[0].callback("home/door", `{"open":true}`)
	if c := hooks.waitStart(t); c.triggerType != "mqtt" || !strings.Contains(c.data, `"topic":"home/door"`) {
		t.Fatalf("mqtt start = %+v", c)
	}
	if len(email.registrations) != 1 || email.registrations[0].subjectContains != "Rechnung" {
		t.Fatalf("email registrations = %+v", email.registrations)
	}
	email.registrations[0].callback("Rechnung Oktober", "shop@example.com", "Hallo")
	if c := hooks.waitStart(t); c.triggerType != "email" || !strings.Contains(c.data, `"subject":"Rechnung Oktober"`) {
		t.Fatalf("email start = %+v", c)
	}
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if len(mqtt.unregisteredTriggers) == 0 || mqtt.unregisteredTriggers[0] != flowSlot(id, "n_aaaaaaaa", TriggerMQTTMessage) {
		t.Fatalf("disabling must unregister MQTT: %+v", mqtt.unregisteredTriggers)
	}
}

func TestFlowScheduleSpecsUseOneCronJobPerNode(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MissionsEnabled: true, SchedulerEnabled: true, AllowFilesystemWrite: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	defer cronMgr.Stop()
	mm := NewMissionManagerV2(dir, cronMgr)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * 1-5"},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: FlowTriggerSchedule, Schedule: "30 18 * * *"},
		FlowTriggerSpec{NodeID: "n_cccccccc", TriggerType: FlowTriggerManual},
	)
	for _, node := range []string{"n_aaaaaaaa", "n_bbbbbbbb"} {
		if !hasCronJob(cronMgr, flowCronJobID(id, node)) {
			t.Fatalf("missing cron job for %s", node)
		}
	}
	if hasCronJob(cronMgr, flowCronJobID(id, "n_cccccccc")) {
		t.Fatal("manual triggers need no cron job")
	}
	if !mm.fireFlowSchedule(id, "n_aaaaaaaa") {
		t.Fatal("a flow cron job must be handled")
	}
	if c := hooks.waitStart(t); c.nodeID != "n_aaaaaaaa" || c.triggerType != "cron" {
		t.Fatalf("cron start = %+v", c)
	}
	if mm.fireFlowSchedule("mission_unknown", "n_aaaaaaaa") {
		t.Fatal("unknown missions fall back to prompt missions")
	}
	if err := mm.SyncFlowMission(id, "Morgenbericht", []FlowTriggerSpec{{NodeID: "n_bbbbbbbb", TriggerType: FlowTriggerSchedule, Schedule: "30 18 * * *"}}); err != nil {
		t.Fatal(err)
	}
	if hasCronJob(cronMgr, flowCronJobID(id, "n_aaaaaaaa")) || !hasCronJob(cronMgr, flowCronJobID(id, "n_bbbbbbbb")) {
		t.Fatal("republishing must drop removed schedule nodes and keep the others")
	}
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if hasCronJob(cronMgr, flowCronJobID(id, "n_bbbbbbbb")) {
		t.Fatal("disabling must remove the cron jobs")
	}
}

func TestSplitFlowCronJobID(t *testing.T) {
	cases := map[string][3]string{
		"mission_mission_123__n_abcdefgh": {"mission_123", "n_abcdefgh", "true"},
		"mission_mission_123":             {"", "", "false"},
		"mission___n_abcdefgh":            {"", "", "false"},
		"mission_mission_1__":             {"", "", "false"},
		"other_mission_1__n_a":            {"", "", "false"},
	}
	for jobID, want := range cases {
		m, n, ok := splitFlowCronJobID(jobID)
		if m != want[0] || n != want[1] || fmt.Sprint(ok) != want[2] {
			t.Errorf("%s = %q %q %v", jobID, m, n, ok)
		}
	}
}

func TestStartKeepsFlowMissionsOutOfTheQueue(t *testing.T) {
	dir := tempSystemTaskDir(t)
	mm := NewMissionManagerV2(dir, nil)
	id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatal(err)
	}
	mm.mu.Lock()
	mm.missions[id].Status = MissionStatusRunning
	mm.missions[id].Enabled = true
	mm.missions[id].FlowPublished = true
	if err := mm.save(); err != nil {
		t.Fatal(err)
	}
	mm.mu.Unlock()

	restarted := NewMissionManagerV2(dir, nil)
	restarted.SetFlowHooks(newFakeFlowHooks())
	if err := restarted.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer restarted.Stop()
	m, _ := restarted.Get(id)
	queue, _ := restarted.GetQueue()
	if m.Status != MissionStatusIdle || len(queue.List()) != 0 {
		t.Fatalf("after restart: status %q queue %+v", m.Status, queue.List())
	}
}

func TestFlowTriggersAreDeepCopied(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}})
	got, _ := mm.Get(id)
	got.FlowTriggers[0].TriggerConfig.WebhookID = "changed"
	listed := mm.List()
	listed[0].FlowTriggers[0].NodeID = "changed"
	again, _ := mm.Get(id)
	if again.FlowTriggers[0].TriggerConfig.WebhookID != "hook-1" || again.FlowTriggers[0].NodeID != "n_aaaaaaaa" {
		t.Fatalf("copies leaked into the manager: %+v", again.FlowTriggers)
	}
}

func TestHomeAssistantMonitoredEntitiesIncludesFlows(t *testing.T) {
	missions := []*MissionV2{
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: &TriggerConfig{HAEntityID: "light.kitchen"}},
		{Enabled: true, ExecutionType: ExecutionFlow, FlowTriggers: []FlowTriggerSpec{
			{NodeID: "n_aaaaaaaa", TriggerType: TriggerHomeAssistantState, TriggerConfig: &TriggerConfig{HAEntityID: "binary_sensor.door"}},
			{NodeID: "n_bbbbbbbb", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "x"}},
		}},
		{Enabled: false, ExecutionType: ExecutionFlow, FlowTriggers: []FlowTriggerSpec{
			{NodeID: "n_cccccccc", TriggerType: TriggerHomeAssistantState, TriggerConfig: &TriggerConfig{HAEntityID: "sensor.off"}},
		}},
	}
	got := homeAssistantMonitoredEntities(missions)
	if len(got) != 2 || !got["light.kitchen"] || !got["binary_sensor.door"] {
		t.Fatalf("entities = %+v", got)
	}
}
