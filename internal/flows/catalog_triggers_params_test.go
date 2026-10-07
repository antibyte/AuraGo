package flows

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func bindOne(t *testing.T, typ string, params map[string]any) (TriggerBinding, error) {
	t.Helper()
	b := newFlow("One")
	b.node("trg", typ, params)
	got, err := BindTriggers(b.build(), triggerRegistry(t), time.UTC, triggerNow)
	if err != nil {
		return TriggerBinding{}, err
	}
	if len(got) != 1 {
		t.Fatalf("BindTriggers = %+v, want one binding", got)
	}
	return got[0], nil
}

func TestMinIntervalSeconds(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  any // nil: the key is absent
		bad   bool
	}{
		{"whole seconds", 30.0, 30.0, false},
		{"numeric text", "45", 45.0, false},
		{"fraction is rounded down", 2.9, 2.0, false},
		{"below one second is cut to zero", 0.5, 0.0, false},
		{"zero", 0.0, nil, false},
		{"negative", -5.0, nil, false},
		{"absent", nil, nil, false},
		{"blank text", "  ", nil, false},
		{"empty list", []any{}, nil, false},
		{"the limit", float64(maxMinIntervalSeconds), float64(maxMinIntervalSeconds), false},
		{"over the limit", float64(maxMinIntervalSeconds) + 1, nil, true},
		{"huge", 1e300, nil, true},
		{"not a number", "soon", nil, true},
		{"true", true, nil, true},
		{"false", false, nil, true},
		{"list", []any{1.0}, nil, true},
		{"object", map[string]any{"a": 1.0}, nil, true},
	}
	for _, tc := range cases {
		for _, typ := range []string{TypeTriggerWebhook, TypeTriggerMQTT} {
			// The key Mission Control reads: MQTT has its own, and only that one.
			key := "min_interval_seconds"
			if typ == TypeTriggerMQTT {
				key = "mqtt_min_interval_seconds"
			}
			params := map[string]any{"webhook": "wh_1", "topic": "a/b"}
			if tc.value != nil {
				params["min_interval_seconds"] = tc.value
			}
			got, err := bindOne(t, typ, params)
			if tc.bad {
				if err == nil || strings.Contains(err.Error(), "1e+300") || errors.Is(err, errParamMissing) {
					t.Errorf("%s/%s: err = %v, want a bounded invalid-value error", tc.name, typ, err)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s/%s: %v", tc.name, typ, err)
				continue
			}
			if v, present := got.Config[key]; tc.want == nil && present || tc.want != nil && v != tc.want {
				t.Errorf("%s/%s: %s = %v (present %v), want %v", tc.name, typ, key, v, present, tc.want)
			}
			if typ == TypeTriggerMQTT {
				if _, both := got.Config["min_interval_seconds"]; both {
					t.Errorf("%s/mqtt: the generic min_interval_seconds is set as well: %+v", tc.name, got.Config)
				}
			}
		}
	}
}

func TestEmailTriggerFolderDefault(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"absent", nil, "INBOX"},
		{"other params only", map[string]any{"subject_contains": "Rechnung"}, "INBOX"},
		{"null", map[string]any{"folder": nil}, "INBOX"},
		{"blank", map[string]any{"folder": "  "}, "INBOX"},
		{"set", map[string]any{"folder": "Archiv"}, "Archiv"},
		{"set with blanks", map[string]any{"folder": " Archiv "}, "Archiv"},
	}
	for _, tc := range cases {
		got, err := bindOne(t, TypeTriggerEmail, tc.params)
		if err != nil || got.MissionTrigger != "email_received" || got.Config["email_folder"] != tc.want {
			t.Errorf("%s: binding = %+v, %v; want email_folder %q", tc.name, got, err, tc.want)
		}
	}
	for _, spec := range lookupDef(t, triggerRegistry(t), TypeTriggerEmail).Params {
		if spec.Name == "folder" && spec.Default != "INBOX" {
			t.Errorf("the folder parameter defaults to %v, want INBOX", spec.Default)
		}
	}
}

// An unknown repeat value must not quietly turn a yearly reminder into a one-off.
func TestDateTimeRepeatIsValidated(t *testing.T) {
	reg := triggerRegistry(t)
	for _, repeat := range []any{nil, "", "  ", "none", RepeatYearly, " yearly "} {
		params := map[string]any{"at": "2027-01-01 10:00", "repeat": repeat}
		got, err := datetimeBinding(t, reg, time.UTC, triggerNow, params)
		wantRepeat := ""
		if strings.TrimSpace(Stringify(repeat)) == RepeatYearly {
			wantRepeat = RepeatYearly
		}
		if err != nil || got.Repeat != wantRepeat {
			t.Errorf("repeat %#v: binding = %+v, %v; want repeat %q", repeat, got, err, wantRepeat)
		}
	}
	for _, repeat := range []any{"monthly", "daily", "Yearly", "yearly,", true, 1.0, []any{"yearly"}} {
		params := map[string]any{"at": "2027-01-01 10:00", "repeat": repeat}
		_, err := datetimeBinding(t, reg, time.UTC, triggerNow, params)
		if err == nil || !strings.Contains(err.Error(), "repeat has an unknown value") || errors.Is(err, errParamMissing) {
			t.Errorf("repeat %#v: err = %v, want an unknown-value error", repeat, err)
		}
	}
	// The editor sees it as an invalid parameter, also while the date is still missing.
	b := newFlow("Repeat")
	missing := b.node("missing", TypeTriggerDateTime, map[string]any{"repeat": "monthly"})
	both := b.node("both", TypeTriggerDateTime, map[string]any{"at": "2027-01-01 10:00", "repeat": "monthly"})
	issues := Validate(b.build(), reg, ValidateContext{Mode: ModePublish, Now: triggerNow, Location: time.UTC})
	for _, id := range []string{missing, both} {
		if findIssue(issues, IssueParamInvalid, id) == nil {
			t.Errorf("node %s: no PARAM_INVALID issue in %+v", id, issues)
		}
	}
	if findIssue(issues, IssueParamRequired, missing) == nil {
		t.Errorf("the missing date is still reported as required: %+v", issues)
	}
}

// A zero now is the current time, as in Validate, so a caller that forgets it does not
// arm dates that are long gone.
func TestBindTriggersZeroNowMeansNow(t *testing.T) {
	reg := triggerRegistry(t)
	bind := func(params map[string]any) (TriggerBinding, error) {
		return datetimeBinding(t, reg, time.UTC, time.Time{}, params)
	}
	if _, err := bind(map[string]any{"at": "2020-03-01 09:00"}); err == nil || !strings.Contains(err.Error(), "in the past") {
		t.Errorf("a one-off date in the past with a zero now: err = %v, want a past-time error", err)
	}
	before := time.Now()
	got, err := bind(map[string]any{"at": "2020-03-01 09:00", "repeat": "yearly"})
	after := time.Now()
	if err != nil || !got.FireAt.After(before) || got.FireAt.After(after.AddDate(1, 0, 1)) || got.FireAt.Month() != time.March || got.FireAt.Day() != 1 {
		t.Errorf("a yearly date with a zero now armed %v, %v; want the next March 1 after now", got.FireAt, err)
	}
	if got, err := bind(map[string]any{"at": "2999-01-01 00:00"}); err != nil || !got.FireAt.Equal(utcTime(2999, 1, 1, 0, 0)) {
		t.Errorf("a date far ahead with a zero now = %+v, %v", got, err)
	}
	// Validate and BindTriggers agree on the same zero now.
	b := newFlow("Zero")
	id := b.node("past", TypeTriggerDateTime, map[string]any{"at": "2020-03-01 09:00"})
	if findIssue(Validate(b.build(), reg, ValidateContext{Mode: ModePublish, Location: time.UTC}), IssueParamInvalid, id) == nil {
		t.Error("Validate with a zero now must reject a date in the past too")
	}
}

// The sample is what the editor shows and what a test run feeds the flow: its fields
// must be the ones Mission Control sends for that event (NotifyBudgetEvent,
// NotifyDeviceEvent, NotifyPlannerAppointmentDue and NotifyPlannerTodoOverdue in
// internal/tools/missions_v2.go).
func TestTriggerSamplesFollowTheEvent(t *testing.T) {
	appointment := []string{"appointment_id", "date_time", "time", "title"}
	todo := []string{"due_date", "time", "title", "todo_id"}
	budget := []string{"event", "limit_usd", "percentage", "spent_usd", "time"}
	device := []string{"device_id", "device_name", "event", "time"}
	cases := []struct {
		name         string
		typ          string
		params       map[string]any
		keys         []string
		event        string // the sample's event field, when it has one
		missionEvent string // the trigger type the same node binds to
	}{
		{"planner default", TypeTriggerPlanner, nil, appointment, "", "planner_appointment_due"},
		{"planner appointment", TypeTriggerPlanner, map[string]any{"event": "appointment_due"}, appointment, "", "planner_appointment_due"},
		{"planner blank", TypeTriggerPlanner, map[string]any{"event": " "}, appointment, "", "planner_appointment_due"},
		{"planner todo", TypeTriggerPlanner, map[string]any{"event": "todo_overdue"}, todo, "", "planner_todo_overdue"},
		{"planner todo with blanks", TypeTriggerPlanner, map[string]any{"event": " todo_overdue "}, todo, "", "planner_todo_overdue"},
		{"budget default", TypeTriggerBudget, nil, budget, "budget_warning", "budget_warning"},
		{"budget warning", TypeTriggerBudget, map[string]any{"event": "warning"}, budget, "budget_warning", "budget_warning"},
		{"budget exceeded", TypeTriggerBudget, map[string]any{"event": "exceeded"}, budget, "budget_exceeded", "budget_exceeded"},
		{"device default", TypeTriggerDevice, nil, device, "device_connected", "device_connected"},
		{"device connected", TypeTriggerDevice, map[string]any{"event": "connected"}, device, "device_connected", "device_connected"},
		{"device disconnected", TypeTriggerDevice, map[string]any{"event": "disconnected"}, device, "device_disconnected", "device_disconnected"},
	}
	for _, tc := range cases {
		node := &Node{Type: tc.typ, Params: tc.params}
		sample := TriggerSample(node)
		if got := sortedKeys(sample); !reflect.DeepEqual(got, tc.keys) {
			t.Errorf("%s: sample fields = %v, want %v", tc.name, got, tc.keys)
		}
		if tc.event != "" && sample["event"] != tc.event {
			t.Errorf("%s: sample event = %v, want %q", tc.name, sample["event"], tc.event)
		}
		if got, err := bindOne(t, tc.typ, tc.params); err != nil || got.MissionTrigger != tc.missionEvent {
			t.Errorf("%s: binds to %q, %v; want %q", tc.name, got.MissionTrigger, err, tc.missionEvent)
		}
		if spent, ok := sample["spent_usd"].(float64); ok {
			if pct := sample["percentage"].(float64); math.Abs(spent/sample["limit_usd"].(float64)-pct) > 1e-9 {
				t.Errorf("%s: percentage %v is not spent/limit (Mission Control sends the ratio)", tc.name, pct)
			}
		}
		if _, err := json.Marshal(sample); err != nil {
			t.Errorf("%s: sample is not JSON encodable: %v", tc.name, err)
		}
		sample["injected"] = true
		delete(sample, "time")
		if got := TriggerSample(node); got["injected"] != nil || got["time"] == nil {
			t.Errorf("%s: a sample must be a fresh copy: %v", tc.name, got)
		}
	}
	// An unknown or odd event gives the default sample (validation reports the value).
	for _, event := range []any{"bogus", 1.0, true, []any{"todo_overdue"}, map[string]any{}, nil} {
		got := sortedKeys(TriggerSample(&Node{Type: TypeTriggerPlanner, Params: map[string]any{"event": event}}))
		if !reflect.DeepEqual(got, appointment) {
			t.Errorf("planner event %#v: sample fields = %v, want the default %v", event, got, appointment)
		}
	}
	if s := TriggerSample(&Node{Type: TypeTriggerPlanner, Params: map[string]any{"event": "todo_overdue"}}); s["title"] == "Zahnarzt" {
		t.Error("the todo sample must not be the appointment's")
	}
}
