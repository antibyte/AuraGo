package flows

import (
	"reflect"
	"testing"
	"time"
)

var triggerNow = time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)

func triggerRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterLogicNodes(reg); err != nil {
		t.Fatal(err)
	}
	if err := RegisterTriggerNodes(reg); err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestTriggerNodesRegistered(t *testing.T) {
	reg := triggerRegistry(t)
	types := []string{TypeTriggerManual, TypeTriggerSchedule, TypeTriggerDateTime, TypeTriggerWebhook, TypeTriggerEmail,
		TypeTriggerMQTT, TypeTriggerHAState, TypeTriggerDevice, TypeTriggerFritzBox, TypeTriggerPlanner,
		TypeTriggerStartup, TypeTriggerBudget, TypeTriggerMission}
	if len(types) != 13 {
		t.Fatalf("expected 13 trigger types, listed %d", len(types))
	}
	for _, typ := range types {
		def := lookupDef(t, reg, typ)
		if !def.Trigger || def.InputPorts() != nil || def.Category != "trigger" {
			t.Errorf("%s is not a proper trigger: %+v", typ, def)
		}
	}
	for typ, untrusted := range map[string]bool{TypeTriggerWebhook: true, TypeTriggerEmail: true, TypeTriggerMQTT: true, TypeTriggerHAState: false, TypeTriggerSchedule: false} {
		if lookupDef(t, reg, typ).UntrustedOutput != untrusted {
			t.Errorf("%s UntrustedOutput != %v", typ, untrusted)
		}
	}
}

func TestScheduleToCron(t *testing.T) {
	cases := []struct {
		p    map[string]any
		want string
		bad  bool
	}{
		{map[string]any{}, "0 7 * * *", false},
		{map[string]any{"mode": "interval_minutes", "minutes": 5.0}, "*/5 * * * *", false},
		{map[string]any{"mode": "interval_hours", "hours": 2.0}, "0 */2 * * *", false},
		{map[string]any{"mode": "daily", "time": "06:30"}, "30 6 * * *", false},
		{map[string]any{"mode": "weekdays", "time": "07:00"}, "0 7 * * 1-5", false},
		{map[string]any{"mode": "weekly", "time": "08:15", "weekdays": []any{"sun", "mon", "fri"}}, "15 8 * * 1,5,0", false},
		{map[string]any{"mode": "monthly", "time": "09:00", "day": 15.0}, "0 9 15 * *", false},
		{map[string]any{"mode": "cron", "cron": " 0  7 * *  1-5 "}, "0 7 * * 1-5", false},
		{map[string]any{"mode": "interval_minutes", "minutes": 90.0}, "", true},
		{map[string]any{"mode": "daily", "time": "7 Uhr"}, "", true},
		{map[string]any{"mode": "weekly", "time": "07:00"}, "", true},
		{map[string]any{"mode": "weekly", "time": "07:00", "weekdays": []any{"funday"}}, "", true},
		{map[string]any{"mode": "cron", "cron": "* *"}, "", true},
		{map[string]any{"mode": "hourly"}, "", true},
	}
	for _, tc := range cases {
		got, err := ScheduleToCron(tc.p)
		if tc.bad {
			if err == nil {
				t.Errorf("ScheduleToCron(%v) = %q, want an error", tc.p, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ScheduleToCron(%v) = %q, %v; want %q", tc.p, got, err, tc.want)
		}
	}
}

func TestBindTriggers(t *testing.T) {
	reg := triggerRegistry(t)
	b := newFlow("Triggers")
	manual := b.node("manual", TypeTriggerManual, map[string]any{"data": map[string]any{"x": 1.0}})
	sched := b.node("sched", TypeTriggerSchedule, map[string]any{"mode": "weekdays", "time": "07:30"})
	hook := b.node("hook", TypeTriggerWebhook, map[string]any{"webhook": "wh_1", "min_interval_seconds": 30.0})
	bday := b.node("bday", TypeTriggerDateTime, map[string]any{"at": "2026-03-01 09:00", "repeat": "yearly"})
	off := b.node("off", TypeTriggerStartup, nil)
	b.node("echo", TypeSet, nil)
	f := b.build()
	f.NodeByID(off).Settings.Disabled = true
	got, err := BindTriggers(f, reg, time.UTC, triggerNow)
	if err != nil || len(got) != 4 {
		t.Fatalf("BindTriggers = %+v, %v", got, err)
	}
	if !reflect.DeepEqual(got[0], TriggerBinding{NodeID: manual, Kind: BindingManual}) {
		t.Errorf("manual = %+v", got[0])
	}
	if !reflect.DeepEqual(got[1], TriggerBinding{NodeID: sched, Kind: BindingCron, Schedule: "30 7 * * 1-5"}) {
		t.Errorf("schedule = %+v", got[1])
	}
	wantHook := TriggerBinding{NodeID: hook, Kind: BindingMission, MissionTrigger: "webhook",
		Config: map[string]any{"webhook_id": "wh_1", "min_interval_seconds": 30.0}}
	if !reflect.DeepEqual(got[2], wantHook) {
		t.Errorf("webhook = %+v", got[2])
	}
	if got[3].NodeID != bday || got[3].Kind != BindingTimer || got[3].Repeat != RepeatYearly ||
		!got[3].FireAt.Equal(time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("datetime = %+v", got[3])
	}

	b = newFlow("Derived")
	b.node("dev", TypeTriggerDevice, map[string]any{"event": "disconnected", "device_id": "d1"})
	b.node("plan", TypeTriggerPlanner, map[string]any{"event": "todo_overdue"})
	b.node("bud", TypeTriggerBudget, nil)
	b.node("ms", TypeTriggerMission, map[string]any{"source": "mission_1"})
	b.node("mq", TypeTriggerMQTT, map[string]any{"topic": "a/#", "min_interval_seconds": 10.0})
	got, err = BindTriggers(b.build(), reg, time.UTC, triggerNow)
	if err != nil {
		t.Fatalf("derived: %v", err)
	}
	wantTypes := []string{"device_disconnected", "planner_todo_overdue", "budget_warning", "mission_completed", "mqtt_message"}
	for i, want := range wantTypes {
		if got[i].MissionTrigger != want {
			t.Errorf("binding %d mission trigger = %q, want %q", i, got[i].MissionTrigger, want)
		}
	}
	if got[3].Config["require_success"] != true || got[3].Config["source_mission_id"] != "mission_1" {
		t.Errorf("mission completed config = %+v", got[3].Config)
	}
	if got[4].Config["mqtt_min_interval_seconds"] != 10.0 || got[4].Config["mqtt_topic"] != "a/#" {
		t.Errorf("mqtt config = %+v", got[4].Config)
	}
}

func TestTriggerValidation(t *testing.T) {
	reg := triggerRegistry(t)
	vc := ValidateContext{Mode: ModePublish, Now: triggerNow, Location: time.UTC}
	b := newFlow("Validation")
	past := b.node("past", TypeTriggerDateTime, map[string]any{"at": "2026-01-01 08:00"})
	missing := b.node("missing", TypeTriggerDateTime, nil)
	cron := b.node("cron", TypeTriggerSchedule, map[string]any{"mode": "cron", "cron": "bad"})
	issues := Validate(b.build(), reg, vc)
	if findIssue(issues, IssueParamInvalid, past) == nil {
		t.Errorf("a one-off date in the past must be invalid: %+v", issues)
	}
	if findIssue(issues, IssueParamRequired, missing) == nil || findIssue(issues, IssueParamInvalid, missing) != nil {
		t.Errorf("a missing date is reported once as required: %+v", issues)
	}
	if findIssue(issues, IssueParamInvalid, cron) == nil {
		t.Errorf("a bad cron expression must be invalid: %+v", issues)
	}
}

func TestTriggerSampleAndNormalize(t *testing.T) {
	email := &Node{Type: TypeTriggerEmail}
	s := TriggerSample(email)
	s["subject"] = "changed"
	if TriggerSample(email)["subject"] != "Rechnung Oktober" {
		t.Fatal("TriggerSample must return a copy")
	}
	manual := &Node{Type: TypeTriggerManual, Params: map[string]any{"data": map[string]any{"a": 1.0}}}
	if !reflect.DeepEqual(TriggerSample(manual), map[string]any{"a": 1.0}) {
		t.Fatalf("manual sample = %#v", TriggerSample(manual))
	}
	if len(TriggerSample(&Node{Type: "nope"})) != 0 {
		t.Fatal("unknown triggers have an empty sample")
	}
	cases := []struct {
		kind, raw string
		want      map[string]any
	}{
		{"webhook", `{"a":1}`, map[string]any{"raw": `{"a":1}`, "payload": map[string]any{"a": 1.0}}},
		{"webhook", "plain", map[string]any{"raw": "plain", "payload": "plain"}},
		{"mqtt", `{"topic":"t","payload":"{\"x\":2}"}`, map[string]any{"topic": "t", "payload": `{"x":2}`, "json": map[string]any{"x": 2.0}}},
		{"email", `{"subject":"s"}`, map[string]any{"subject": "s"}},
		{"api", "not json", map[string]any{"raw": "not json"}},
		{"cron", "", map[string]any{}},
	}
	for _, tc := range cases {
		if got := NormalizeTriggerData(tc.kind, tc.raw); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("NormalizeTriggerData(%s, %q) = %#v, want %#v", tc.kind, tc.raw, got, tc.want)
		}
	}
}
