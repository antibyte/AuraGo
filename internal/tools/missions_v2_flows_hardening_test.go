package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// c07DeadlockGuard bounds waits on hook calls that re-enter the manager. Every guarded step
// finishes in well under 100 ms, so 5 s leaves 50x headroom.
const c07DeadlockGuard = 5 * time.Second

const c07PayloadMarker = "c07-untrusted-payload-marker"

// c07Webhooks is a webhook manager fake whose fields sit behind a mutex.
type c07Webhooks struct {
	mu        sync.Mutex
	callbacks map[string][]func([]byte)
}

func (f *c07Webhooks) RegisterMissionTrigger(webhookID string, callback func(payload []byte)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.callbacks == nil {
		f.callbacks = make(map[string][]func([]byte))
	}
	f.callbacks[webhookID] = append(f.callbacks[webhookID], callback)
}

func (f *c07Webhooks) count(webhookID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.callbacks[webhookID])
}

func (f *c07Webhooks) fire(webhookID string, payload []byte) {
	f.mu.Lock()
	callbacks := append(([]func([]byte))(nil), f.callbacks[webhookID]...)
	f.mu.Unlock()
	for _, callback := range callbacks {
		callback(payload)
	}
}

// c07Emails is an email watcher fake whose fields sit behind a mutex.
type c07Emails struct {
	mu        sync.Mutex
	callbacks []func(subject, from, body string)
}

func (f *c07Emails) RegisterMissionTrigger(_, _, _ string, callback func(subject, from, body string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callbacks = append(f.callbacks, callback)
}

func (f *c07Emails) registered() []func(subject, from, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(([]func(subject, from, body string))(nil), f.callbacks...)
}

// c07PlainMQTT implements only MQTTManagerInterface, so its registrations cannot be removed.
type c07PlainMQTT struct {
	mu        sync.Mutex
	callbacks []func(topic, payload string)
}

func (f *c07PlainMQTT) RegisterMissionTrigger(_, _ string, _ int, callback func(topic, payload string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callbacks = append(f.callbacks, callback)
}

func (f *c07PlainMQTT) registered() []func(topic, payload string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(([]func(topic, payload string))(nil), f.callbacks...)
}

// c07KeyedMQTT replaces and removes registrations by key, like the real MQTT bridge.
type c07KeyedMQTT struct {
	mu    sync.Mutex
	byKey map[string]func(topic, payload string)
	plain int
}

func (f *c07KeyedMQTT) RegisterMissionTrigger(_, _ string, _ int, _ func(topic, payload string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.plain++
}

func (f *c07KeyedMQTT) RegisterMissionTriggerForKey(key, _, _ string, _ int, callback func(topic, payload string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byKey == nil {
		f.byKey = make(map[string]func(topic, payload string))
	}
	f.byKey[key] = callback
}

func (f *c07KeyedMQTT) UnregisterMissionTrigger(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byKey, key)
}

func (f *c07KeyedMQTT) registered() []func(topic, payload string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]func(topic, payload string), 0, len(f.byKey))
	for _, callback := range f.byKey {
		out = append(out, callback)
	}
	return out
}

// c07ReentrantHooks calls back into the manager from StartFlowRun, with a read lock (Get,
// List) and a write lock (SetFlowMissionEnabled), before it reports the run on runs.
type c07ReentrantHooks struct {
	mm   *MissionManagerV2
	runs chan string

	mu   sync.Mutex
	errs []error
}

func (h *c07ReentrantHooks) StartFlowRun(missionID, _, triggerType, _ string) error {
	if _, ok := h.mm.Get(missionID); !ok {
		h.addErr(fmt.Errorf("Get(%s) found no mission", missionID))
	}
	if len(h.mm.List()) == 0 {
		h.addErr(fmt.Errorf("List() returned no missions"))
	}
	if err := h.mm.SetFlowMissionEnabled(missionID, true); err != nil {
		h.addErr(fmt.Errorf("SetFlowMissionEnabled: %w", err))
	}
	h.runs <- triggerType
	return nil
}

func (h *c07ReentrantHooks) FlowMissionDeleted(string) {}

func (h *c07ReentrantHooks) FlowEnabledChanged(string, bool) {}

func (h *c07ReentrantHooks) NextFlowRun(string) (time.Time, bool) { return time.Time{}, false }

func (h *c07ReentrantHooks) addErr(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errs = append(h.errs, err)
}

func (h *c07ReentrantHooks) failures() []error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]error(nil), h.errs...)
}

func (h *c07ReentrantHooks) waitRun(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-h.runs:
		if got != want {
			t.Fatalf("hook run for trigger %q, want %q", got, want)
		}
	case <-time.After(c07DeadlockGuard):
		t.Fatalf("no %s run within %s: the hook could not re-enter the manager", want, c07DeadlockGuard)
	}
}

// c07Within runs fn on its own goroutine and fails when it does not return within the guard.
func c07Within(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(c07DeadlockGuard):
		t.Fatalf("%s did not return within %s: deadlock", what, c07DeadlockGuard)
	}
}

// c07LogBuffer collects slog output behind a mutex.
type c07LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *c07LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *c07LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// c07CaptureWarnings routes the default slog logger at Warn level into a buffer for the test.
func c07CaptureWarnings(t *testing.T) *c07LogBuffer {
	t.Helper()
	logs := &c07LogBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return logs
}

const c07DropMessage = "Flow trigger payload too large"

func c07ExpectDropWarning(t *testing.T, logs *c07LogBuffer, missionID, nodeID string, size int) {
	t.Helper()
	out := logs.String()
	for _, want := range []string{c07DropMessage, "mission_id=" + missionID, "node=" + nodeID, fmt.Sprintf("size_bytes=%d", size)} {
		if !strings.Contains(out, want) {
			t.Fatalf("drop warning lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, c07PayloadMarker) {
		t.Fatal("the drop warning must never contain the payload")
	}
}

// c07Payload returns n bytes that start with c07PayloadMarker.
func c07Payload(n int) []byte {
	out := bytes.Repeat([]byte("x"), n)
	copy(out, c07PayloadMarker)
	return out
}

func c07MissionCronRunner(t *testing.T, cronMgr *CronManager) func(jobID, prompt string) {
	t.Helper()
	cronMgr.mu.Lock()
	runner := cronMgr.runners["mission"]
	cronMgr.mu.Unlock()
	if runner == nil {
		t.Fatal("the mission manager registered no cron runner")
	}
	return runner
}

func c07FlowCronJobs(cronMgr *CronManager, missionID string) int {
	n := 0
	for _, job := range cronMgr.GetJobs() {
		if strings.HasPrefix(job.ID, "mission_"+missionID+flowCronSeparator) {
			n++
		}
	}
	return n
}

// Extra 1: a cron job id is only routed to a flow when it splits into an existing flow
// mission and does not name an existing mission as a whole.
func TestFlowCronJobsNeverCapturePromptMissionJobs(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	prompts := make(chan string, 8)
	mm.SetCallback(func(_ string, missionID string) {
		prompts <- missionID
		mm.OnMissionComplete(missionID, MissionResultSuccess, "ok")
	})
	flowID := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"})
	// One prompt id with "__" whose head is no mission, one whose head is the flow mission.
	promptIDs := []string{"mission_a__b", flowID + flowCronSeparator + "n_bbbbbbbb"}
	for _, id := range promptIDs {
		if err := mm.Create(&MissionV2{ID: id, Name: id, Prompt: "run", ExecutionType: ExecutionManual}); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
	}
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mm.Stop)
	run := c07MissionCronRunner(t, cronMgr)

	for _, id := range promptIDs {
		run("mission_"+id, "run")
		select {
		case got := <-prompts:
			if got != id {
				t.Fatalf("cron job of %s ran prompt mission %s", id, got)
			}
		case <-time.After(c07DeadlockGuard):
			t.Fatalf("cron job of prompt mission %s did not run it", id)
		}
		hooks.expectNoStart(t)
	}

	run(flowCronJobID(flowID, "n_aaaaaaaa"), "EasyDrag flow trigger")
	if c := hooks.waitStart(t); c != (flowStartCall{flowID, "n_aaaaaaaa", "cron", ""}) {
		t.Fatalf("flow cron start = %+v", c)
	}
	select {
	case id := <-prompts:
		t.Fatalf("the flow cron job also ran prompt mission %s", id)
	case <-time.After(100 * time.Millisecond):
	}
	if queue, _ := mm.GetQueue(); len(queue.List()) != 0 {
		t.Fatalf("flow cron jobs must not use the agent queue: %+v", queue.List())
	}
}

// Extra 2: webhook payloads over the cap start no run and warn without the payload.
func TestFlowWebhookPayloadLimit(t *testing.T) {
	logs := c07CaptureWarnings(t)
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	webhooks := &c07Webhooks{}
	mm.SetWebhookManager(webhooks)
	spec := FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1", MinIntervalSeconds: 60}}
	id := publishTestFlow(t, mm, spec)

	webhooks.fire("hook-1", c07Payload(flowMaxEventPayloadBytes+1))
	hooks.expectNoStart(t)
	c07ExpectDropWarning(t, logs, id, "n_aaaaaaaa", flowMaxEventPayloadBytes+1)

	// The dropped event used no min-interval slot: a payload at the cap still starts a run.
	atCap := c07Payload(flowMaxEventPayloadBytes)
	webhooks.fire("hook-1", atCap)
	if c := hooks.waitStart(t); c.missionID != id || c.data != string(atCap) {
		t.Fatalf("start at the cap = mission %s, %d bytes", c.missionID, len(c.data))
	}

	// A registration that no longer targets the node drops quietly.
	moved := spec
	moved.TriggerConfig = &TriggerConfig{WebhookID: "hook-2"}
	if err := mm.SyncFlowMission(id, "Morgenbericht", []FlowTriggerSpec{moved}); err != nil {
		t.Fatal(err)
	}
	before := strings.Count(logs.String(), c07DropMessage)
	webhooks.fire("hook-1", c07Payload(flowMaxEventPayloadBytes+1))
	hooks.expectNoStart(t)
	if after := strings.Count(logs.String(), c07DropMessage); after != before {
		t.Fatalf("a stale registration warned about a dropped payload (%d -> %d)", before, after)
	}
}

// Extra 2: MQTT payloads over the cap start no run, keyed and plain registrations alike.
func TestFlowMQTTPayloadLimit(t *testing.T) {
	for _, keyed := range []bool{true, false} {
		t.Run(fmt.Sprintf("keyed=%v", keyed), func(t *testing.T) {
			logs := c07CaptureWarnings(t)
			mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
			hooks := newFakeFlowHooks()
			mm.SetFlowHooks(hooks)
			var registered func() []func(topic, payload string)
			if keyed {
				mqtt := &c07KeyedMQTT{}
				mm.SetMQTTManager(mqtt)
				registered = mqtt.registered
			} else {
				mqtt := &c07PlainMQTT{}
				mm.SetMQTTManager(mqtt)
				registered = mqtt.registered
			}
			id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerMQTTMessage, TriggerConfig: &TriggerConfig{MQTTTopic: "home/door"}})
			callbacks := registered()
			if len(callbacks) != 1 {
				t.Fatalf("%d MQTT registrations, want 1", len(callbacks))
			}

			callbacks[0]("home/door", string(c07Payload(flowMaxEventPayloadBytes+1)))
			hooks.expectNoStart(t)
			c07ExpectDropWarning(t, logs, id, "n_aaaaaaaa", flowMaxEventPayloadBytes+1)

			atCap := string(c07Payload(flowMaxEventPayloadBytes))
			callbacks[0]("home/door", atCap)
			c := hooks.waitStart(t)
			var data struct{ Topic, Payload string }
			if err := json.Unmarshal([]byte(c.data), &data); err != nil {
				t.Fatalf("trigger data: %v", err)
			}
			if c.triggerType != "mqtt" || data.Topic != "home/door" || data.Payload != atCap {
				t.Fatalf("start at the cap = %s, topic %q, %d payload bytes", c.triggerType, data.Topic, len(data.Payload))
			}
		})
	}
}

// Extra 2: email bodies over the cap are cut at a rune boundary and marked "truncated".
func TestFlowEmailBodyLimit(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	email := &c07Emails{}
	mm.SetEmailWatcher(email)
	publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerEmailReceived, TriggerConfig: &TriggerConfig{EmailFolder: "INBOX"}})
	callbacks := email.registered()
	if len(callbacks) != 1 {
		t.Fatalf("%d email registrations, want 1", len(callbacks))
	}
	limit := flowMaxEventPayloadBytes
	cases := []struct {
		name, body, wantBody string
		truncated            bool
	}{
		{"at the cap", strings.Repeat("a", limit), strings.Repeat("a", limit), false},
		{"one byte over", strings.Repeat("a", limit+1), strings.Repeat("a", limit), true},
		{"rune across the cap", strings.Repeat("a", limit-1) + "é", strings.Repeat("a", limit-1), true},
	}
	for _, tc := range cases {
		callbacks[0]("Rechnung Oktober", "shop@example.com", tc.body)
		c := hooks.waitStart(t)
		var data struct {
			Subject, From, Body string
			Truncated           bool
		}
		if err := json.Unmarshal([]byte(c.data), &data); err != nil {
			t.Fatalf("%s: trigger data: %v", tc.name, err)
		}
		if data.Subject != "Rechnung Oktober" || data.From != "shop@example.com" || data.Body != tc.wantBody ||
			data.Truncated != tc.truncated || !utf8.ValidString(data.Body) {
			t.Fatalf("%s: subject %q from %q body %d bytes truncated %v", tc.name, data.Subject, data.From, len(data.Body), data.Truncated)
		}
		if strings.Contains(c.data, `"truncated"`) != tc.truncated {
			t.Fatalf("%s: the truncated key must appear only for cut bodies", tc.name)
		}
	}
}

func TestCutAtRuneBoundary(t *testing.T) {
	cases := []struct {
		s     string
		limit int
		want  string
	}{
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"ab€", 3, "ab"},
		{"a😀", 4, "a"},
		{"😀b", 2, ""},
		{"\x80\x80\x80\x80\x80", 4, "\x80"},
	}
	for _, tc := range cases {
		if got := cutAtRuneBoundary(tc.s, tc.limit); got != tc.want {
			t.Errorf("cutAtRuneBoundary(%q, %d) = %q, want %q", tc.s, tc.limit, got, tc.want)
		}
	}
}

// Extra 3: a deleted flow mission keeps no cron jobs or keyed MQTT registrations, and its
// remaining webhook and email callbacks start nothing.
func TestDeletedFlowMissionLeavesNoLiveTriggers(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	webhooks, email, mqtt := &c07Webhooks{}, &c07Emails{}, &c07KeyedMQTT{}
	mm.SetWebhookManager(webhooks)
	mm.SetEmailWatcher(email)
	mm.SetMQTTManager(mqtt)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}},
		FlowTriggerSpec{NodeID: "n_cccccccc", TriggerType: TriggerEmailReceived, TriggerConfig: &TriggerConfig{EmailFolder: "INBOX"}},
		FlowTriggerSpec{NodeID: "n_dddddddd", TriggerType: TriggerMQTTMessage, TriggerConfig: &TriggerConfig{MQTTTopic: "home/door"}},
	)
	emailCallbacks, mqttCallbacks := email.registered(), mqtt.registered()
	if c07FlowCronJobs(cronMgr, id) != 1 || webhooks.count("hook-1") != 1 || len(emailCallbacks) != 1 || len(mqttCallbacks) != 1 {
		t.Fatalf("setup: cron %d webhook %d email %d mqtt %d", c07FlowCronJobs(cronMgr, id), webhooks.count("hook-1"), len(emailCallbacks), len(mqttCallbacks))
	}
	if err := mm.Create(&MissionV2{ID: "mission_prompt", Name: "Prompt", Prompt: "run", ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	if err := mm.DeleteFlowMission("mission_prompt"); err == nil {
		t.Fatal("DeleteFlowMission must refuse prompt missions")
	}
	if _, ok := mm.Get("mission_prompt"); !ok {
		t.Fatal("DeleteFlowMission removed a prompt mission")
	}

	if err := mm.DeleteFlowMission(id); err != nil {
		t.Fatalf("DeleteFlowMission: %v", err)
	}
	if _, ok := mm.Get(id); ok {
		t.Fatal("the flow mission is still there")
	}
	if n := c07FlowCronJobs(cronMgr, id); n != 0 {
		t.Fatalf("%d cron jobs left after delete", n)
	}
	if n := len(mqtt.registered()); n != 0 {
		t.Fatalf("%d keyed MQTT registrations left after delete", n)
	}
	webhooks.fire("hook-1", []byte(`{}`))
	for _, callback := range emailCallbacks {
		callback("Rechnung", "shop@example.com", "Hallo")
	}
	for _, callback := range mqttCallbacks { // an in-flight delivery of the removed registration
		callback("home/door", "{}")
	}
	if mm.fireFlowCronJob(flowCronJobID(id, "n_aaaaaaaa")) {
		t.Fatal("a stale cron job of a deleted flow must fall back to the prompt path")
	}
	hooks.expectNoStart(t)
	if err := mm.DeleteFlowMission(id); err != nil {
		t.Fatalf("deleting a missing flow mission: %v", err)
	}
}

// Extra 3: re-syncs, a change and a change back, and disable/enable never register a
// webhook, email or plain MQTT trigger twice, so one event starts exactly one run.
func TestFlowResyncNeverDoublesRegistrations(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	webhooks, email, mqtt := &c07Webhooks{}, &c07Emails{}, &c07PlainMQTT{}
	mm.SetWebhookManager(webhooks)
	mm.SetEmailWatcher(email)
	mm.SetMQTTManager(mqtt)
	specs := func(hook, folder, topic string) []FlowTriggerSpec {
		return []FlowTriggerSpec{
			{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: hook}},
			{NodeID: "n_bbbbbbbb", TriggerType: TriggerEmailReceived, TriggerConfig: &TriggerConfig{EmailFolder: folder}},
			{NodeID: "n_cccccccc", TriggerType: TriggerMQTTMessage, TriggerConfig: &TriggerConfig{MQTTTopic: topic}},
		}
	}
	id := publishTestFlow(t, mm, specs("hook-1", "INBOX", "home/a")...)
	for _, s := range [][]FlowTriggerSpec{specs("hook-1", "INBOX", "home/a"), specs("hook-2", "Archiv", "home/b"), specs("hook-1", "INBOX", "home/a")} {
		if err := mm.SyncFlowMission(id, "Morgenbericht", s); err != nil {
			t.Fatal(err)
		}
	}
	for _, enabled := range []bool{false, true} {
		if err := mm.SetFlowMissionEnabled(id, enabled); err != nil {
			t.Fatal(err)
		}
	}
	if webhooks.count("hook-1") != 1 || webhooks.count("hook-2") != 1 {
		t.Fatalf("webhook registrations: hook-1 %d, hook-2 %d", webhooks.count("hook-1"), webhooks.count("hook-2"))
	}
	emailCallbacks, mqttCallbacks := email.registered(), mqtt.registered()
	if len(emailCallbacks) != 2 || len(mqttCallbacks) != 2 {
		t.Fatalf("email %d, plain MQTT %d registrations; want one per distinct filter (2)", len(emailCallbacks), len(mqttCallbacks))
	}

	webhooks.fire("hook-1", []byte(`{}`))
	if c := hooks.waitStart(t); c.nodeID != "n_aaaaaaaa" {
		t.Fatalf("webhook start = %+v", c)
	}
	webhooks.fire("hook-2", []byte(`{}`))
	hooks.expectNoStart(t)
	for _, callback := range emailCallbacks {
		callback("Rechnung", "shop@example.com", "Hallo")
	}
	if c := hooks.waitStart(t); c.nodeID != "n_bbbbbbbb" {
		t.Fatalf("email start = %+v", c)
	}
	for _, callback := range mqttCallbacks {
		callback("home/a", "{}")
	}
	if c := hooks.waitStart(t); c.nodeID != "n_cccccccc" {
		t.Fatalf("MQTT start = %+v", c)
	}
	hooks.expectNoStart(t)
}

// Extra 3: filters that contain the "|" separator never share a registration key, so a
// changed filter always gets a registration of its own.
func TestFlowFiltersWithSeparatorsRegisterSeparately(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	email, mqtt := &c07Emails{}, &c07PlainMQTT{}
	mm.SetEmailWatcher(email)
	mm.SetMQTTManager(mqtt)
	specs := func(folder, subject, topic, contains string) []FlowTriggerSpec {
		return []FlowTriggerSpec{
			{NodeID: "n_aaaaaaaa", TriggerType: TriggerEmailReceived, TriggerConfig: &TriggerConfig{EmailFolder: folder, EmailSubjectContains: subject}},
			{NodeID: "n_bbbbbbbb", TriggerType: TriggerMQTTMessage, TriggerConfig: &TriggerConfig{MQTTTopic: topic, MQTTPayloadContains: contains}},
		}
	}
	id := publishTestFlow(t, mm, specs("a|b", "c", "x|y", "z")...)
	if err := mm.SyncFlowMission(id, "Morgenbericht", specs("a", "b|c", "x", "y|z")); err != nil {
		t.Fatal(err)
	}
	emailCallbacks, mqttCallbacks := email.registered(), mqtt.registered()
	if len(emailCallbacks) != 2 || len(mqttCallbacks) != 2 {
		t.Fatalf("email %d, plain MQTT %d registrations; the changed filters need their own (2 each)", len(emailCallbacks), len(mqttCallbacks))
	}
	emailCallbacks[1]("b|c", "shop@example.com", "Hallo")
	if c := hooks.waitStart(t); c.nodeID != "n_aaaaaaaa" {
		t.Fatalf("email start = %+v", c)
	}
	mqttCallbacks[1]("x", "y|z")
	if c := hooks.waitStart(t); c.nodeID != "n_bbbbbbbb" {
		t.Fatalf("MQTT start = %+v", c)
	}
}

// Extra 4: FlowHooks are never called under m.mu, so a hook may call back into the manager
// on every trigger path, including the startup trigger fired by Start.
func TestFlowHooksMayCallBackIntoTheManager(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	hooks := &c07ReentrantHooks{mm: mm, runs: make(chan string, 16)}
	mm.SetFlowHooks(hooks)
	webhooks, email, mqtt := &c07Webhooks{}, &c07Emails{}, &c07KeyedMQTT{}
	mm.SetWebhookManager(webhooks)
	mm.SetEmailWatcher(email)
	mm.SetMQTTManager(mqtt)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}},
		FlowTriggerSpec{NodeID: "n_cccccccc", TriggerType: TriggerEmailReceived, TriggerConfig: &TriggerConfig{EmailFolder: "INBOX"}},
		FlowTriggerSpec{NodeID: "n_dddddddd", TriggerType: TriggerMQTTMessage, TriggerConfig: &TriggerConfig{MQTTTopic: "home/door"}},
		FlowTriggerSpec{NodeID: "n_eeeeeeee", TriggerType: TriggerDeviceConnected, TriggerConfig: &TriggerConfig{}},
		FlowTriggerSpec{NodeID: "n_ffffffff", TriggerType: TriggerSystemStartup, TriggerConfig: &TriggerConfig{}},
	)
	emailCallbacks, mqttCallbacks := email.registered(), mqtt.registered()
	if len(emailCallbacks) != 1 || len(mqttCallbacks) != 1 {
		t.Fatalf("setup: email %d, MQTT %d registrations", len(emailCallbacks), len(mqttCallbacks))
	}
	steps := []struct {
		name, trigger string
		fire          func()
	}{
		{"webhook", "webhook", func() { webhooks.fire("hook-1", []byte(`{}`)) }},
		{"email", "email", func() { emailCallbacks[0]("Rechnung", "shop@example.com", "Hallo") }},
		{"mqtt", "mqtt", func() { mqttCallbacks[0]("home/door", "{}") }},
		{"cron", "cron", func() { mm.fireFlowCronJob(flowCronJobID(id, "n_aaaaaaaa")) }},
		{"device event", "device_connected", func() { mm.NotifyDeviceEvent("device_connected", "dev-1", "Laptop") }},
		{"startup event", "system_startup", mm.NotifySystemStartup},
	}
	for _, step := range steps {
		c07Within(t, step.name, step.fire)
		hooks.waitRun(t, step.trigger)
	}
	if errs := hooks.failures(); len(errs) != 0 {
		t.Fatalf("re-entrant calls failed: %v", errs)
	}

	restarted := NewMissionManagerV2(dir, cronMgr)
	restartHooks := &c07ReentrantHooks{mm: restarted, runs: make(chan string, 16)}
	restarted.SetFlowHooks(restartHooks)
	c07Within(t, "Start", func() {
		if err := restarted.Start(); err != nil {
			t.Errorf("Start: %v", err)
		}
	})
	t.Cleanup(restarted.Stop)
	restartHooks.waitRun(t, "system_startup")
	if errs := restartHooks.failures(); len(errs) != 0 {
		t.Fatalf("re-entrant calls after Start failed: %v", errs)
	}
}

// c07LegacyMissionsFile is a missions file in the format before flow missions existed. Its
// values are already normalised, so loading and saving it must reproduce every object.
const c07LegacyMissionsFile = `[
  {
    "id": "mission_legacy_scheduled",
    "name": "Morning report",
    "prompt": "Summarise the overnight news.",
    "execution_type": "scheduled",
    "schedule": "0 7 * * 1-5",
    "trigger_type": "",
    "priority": "high",
    "enabled": true,
    "status": "idle",
    "last_run": "2026-09-30T07:00:00Z",
    "last_result": "success",
    "last_output": "Report sent.",
    "run_count": 3,
    "created_at": "2026-01-02T03:04:05Z",
    "locked": false,
    "cheatsheet_ids": [
      "cs_news"
    ],
    "runner_type": "local",
    "preparation_status": "prepared",
    "last_prepared_at": "2026-09-29T20:00:00+02:00",
    "auto_prepare": true
  },
  {
    "id": "mission_legacy_webhook",
    "name": "Door bell",
    "prompt": "Tell me who rang.",
    "execution_type": "triggered",
    "schedule": "",
    "trigger_type": "webhook",
    "trigger_config": {
      "min_interval_seconds": 30,
      "webhook_id": "wh_door",
      "webhook_slug": "door"
    },
    "priority": "medium",
    "enabled": true,
    "status": "idle",
    "last_run": "0001-01-01T00:00:00Z",
    "last_result": "",
    "last_output": "",
    "run_count": 0,
    "created_at": "2026-02-03T04:05:06.123456789Z",
    "locked": true,
    "runner_type": "local"
  },
  {
    "id": "mission_legacy_remote",
    "name": "Nest backup",
    "prompt": "Back up the nest.",
    "execution_type": "manual",
    "schedule": "",
    "trigger_type": "",
    "priority": "low",
    "enabled": false,
    "status": "idle",
    "last_run": "0001-01-01T00:00:00Z",
    "last_result": "error",
    "last_output": "egg offline",
    "run_count": 1,
    "created_at": "2026-03-04T05:06:07Z",
    "locked": false,
    "waiting_for_id": "mission_legacy_scheduled",
    "runner_type": "remote",
    "remote_nest_id": "nest_1",
    "remote_nest_name": "Cellar",
    "remote_egg_id": "egg_1",
    "remote_egg_name": "Backup egg",
    "remote_sync_status": "synced",
    "remote_revision": "rev_7"
  },
  {
    "id": "mission_legacy_synced",
    "name": "Synced mission",
    "prompt": "Check the disk.",
    "execution_type": "triggered",
    "schedule": "",
    "trigger_type": "mission_completed",
    "trigger_config": {
      "source_mission_id": "mission_legacy_scheduled",
      "source_mission_name": "Morning report",
      "require_success": true
    },
    "priority": "medium",
    "enabled": true,
    "status": "waiting",
    "last_run": "0001-01-01T00:00:00Z",
    "last_result": "",
    "last_output": "",
    "run_count": 0,
    "created_at": "2026-04-05T06:07:08Z",
    "locked": false,
    "runner_type": "local",
    "synced_from_master": true
  }
]`

// c07MissionObjects maps each mission object of a missions file to its exact bytes.
func c07MissionObjects(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var objects []json.RawMessage
	if err := json.Unmarshal(data, &objects); err != nil {
		t.Fatalf("missions file: %v", err)
	}
	out := make(map[string]string, len(objects))
	for _, object := range objects {
		var head struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(object, &head); err != nil {
			t.Fatalf("mission object: %v", err)
		}
		out[head.ID] = string(object)
	}
	return out
}

// Extra 5: a missions file without the flow fields loads and saves byte-identically.
func TestMissionsFileWithoutFlowFieldsSavesUnchanged(t *testing.T) {
	dir := tempSystemTaskDir(t)
	file := filepath.Join(dir, "missions_v2.json")
	if err := os.WriteFile(file, []byte(c07LegacyMissionsFile), 0o644); err != nil {
		t.Fatal(err)
	}
	mm := NewMissionManagerV2(dir, nil)
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mm.Stop)
	for _, m := range mm.List() {
		if m.FlowID != "" || m.FlowTriggers != nil || m.FlowPublished || isFlowMission(m) {
			t.Fatalf("prompt mission %s gained flow fields: %+v", m.ID, m)
		}
	}
	mm.mu.Lock()
	err := mm.save()
	mm.mu.Unlock()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	saved, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(saved, []byte(`"flow_`)) {
		t.Fatalf("prompt missions saved flow fields:\n%s", saved)
	}
	want, got := c07MissionObjects(t, []byte(c07LegacyMissionsFile)), c07MissionObjects(t, saved)
	if len(got) != len(want) {
		t.Fatalf("saved %d missions, want %d", len(got), len(want))
	}
	for id, object := range want {
		if got[id] != object {
			t.Errorf("mission %s changed on load and save:\nwant %s\ngot  %s", id, object, got[id])
		}
	}
}

// Extra 5: a flow mission keeps its flow fields across a restart.
func TestFlowMissionFieldsSurviveARestart(t *testing.T) {
	dir := tempSystemTaskDir(t)
	mm := NewMissionManagerV2(dir, nil)
	specs := []FlowTriggerSpec{
		{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}},
		{NodeID: "n_bbbbbbbb", TriggerType: FlowTriggerManual},
	}
	id := publishTestFlow(t, mm, specs...)
	raw, err := os.ReadFile(filepath.Join(dir, "missions_v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"execution_type": "flow"`, `"flow_id": "flow_aaaaaaaaaa"`, `"flow_triggers": [`, `"flow_published": true`} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Fatalf("missions file lacks %s:\n%s", want, raw)
		}
	}

	restarted := NewMissionManagerV2(dir, nil)
	if err := restarted.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(restarted.Stop)
	m, ok := restarted.Get(id)
	if !ok || m.ExecutionType != ExecutionFlow || m.FlowID != "flow_aaaaaaaaaa" || !m.FlowPublished || !m.Enabled ||
		!reflect.DeepEqual(m.FlowTriggers, specs) {
		t.Fatalf("after restart: %+v", m)
	}
}

// c07LegacyHomeAssistantEntities is the poller's entity loop before flow missions existed.
func c07LegacyHomeAssistantEntities(missions []*MissionV2) map[string]bool {
	monitoredEntities := make(map[string]bool)
	for _, mission := range missions {
		if mission.ExecutionType == ExecutionTriggered && mission.TriggerType == TriggerHomeAssistantState && mission.Enabled {
			if mission.TriggerConfig != nil && mission.TriggerConfig.HAEntityID != "" {
				monitoredEntities[mission.TriggerConfig.HAEntityID] = true
			}
		}
	}
	return monitoredEntities
}

// Extra 6: prompt missions are monitored exactly as before; flows only while enabled.
func TestHomeAssistantMonitoredEntitiesKeepsPromptBehaviour(t *testing.T) {
	ha := func(entity string) *TriggerConfig { return &TriggerConfig{HAEntityID: entity} }
	prompt := []*MissionV2{
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("light.kitchen")},
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("light.kitchen")},
		{Enabled: false, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("light.off")},
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerWebhook, TriggerConfig: ha("light.webhook")},
		{Enabled: true, ExecutionType: ExecutionManual, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("light.manual")},
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState},
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("")},
		{Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("sensor.remote"),
			RunnerType: MissionRunnerRemote, RemoteNestID: "n", RemoteEggID: "e"},
	}
	if got, want := homeAssistantMonitoredEntities(prompt), c07LegacyHomeAssistantEntities(prompt); !reflect.DeepEqual(got, want) {
		t.Fatalf("prompt entities = %v, before = %v", got, want)
	}
	// A flow mission's top-level trigger fields are not its triggers.
	topLevel := []*MissionV2{{Enabled: true, ExecutionType: ExecutionFlow, TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("light.top")}}
	if got := homeAssistantMonitoredEntities(topLevel); len(got) != 0 {
		t.Fatalf("flow top-level entities = %v", got)
	}

	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mm.SetFlowHooks(newFakeFlowHooks())
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerHomeAssistantState, TriggerConfig: ha("binary_sensor.door")},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerHomeAssistantState},
	)
	if got := homeAssistantMonitoredEntities(mm.List()); !reflect.DeepEqual(got, map[string]bool{"binary_sensor.door": true}) {
		t.Fatalf("enabled flow entities = %v", got)
	}
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if got := homeAssistantMonitoredEntities(mm.List()); len(got) != 0 {
		t.Fatalf("disabled flow entities = %v", got)
	}
}
