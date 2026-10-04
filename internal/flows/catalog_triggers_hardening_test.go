package flows

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// maxEchoMessageBytes bounds an error or issue message of a trigger hook. A quoted
// echo is at most 40 runes, each escaped to at most 10 bytes, plus the fixed text.
const maxEchoMessageBytes = 600

func utcTime(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func datetimeBinding(t *testing.T, reg *Registry, loc *time.Location, now time.Time, params map[string]any) (TriggerBinding, error) {
	t.Helper()
	b := newFlow("Datetime")
	b.node("when", TypeTriggerDateTime, params)
	got, err := BindTriggers(b.build(), reg, loc, now)
	if err != nil {
		return TriggerBinding{}, err
	}
	if len(got) != 1 {
		t.Fatalf("BindTriggers = %+v, want one binding", got)
	}
	return got[0], nil
}

// The set of triggers whose output an outsider can influence is part of the contract:
// the lint warns about untrusted data in a sensitive parameter only for these.
func TestTriggerUntrustedOutputTable(t *testing.T) {
	reg := newTestRegistry(t)
	if err := RegisterTriggerNodes(reg); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		TypeTriggerManual: false, TypeTriggerSchedule: false, TypeTriggerDateTime: false,
		TypeTriggerWebhook: true, TypeTriggerEmail: true, TypeTriggerMQTT: true,
		TypeTriggerHAState: false, TypeTriggerDevice: false, TypeTriggerFritzBox: true,
		TypeTriggerPlanner: true, TypeTriggerStartup: false, TypeTriggerBudget: false,
		TypeTriggerMission: true,
	}
	if len(want) != 13 {
		t.Fatalf("table lists %d trigger types, want 13", len(want))
	}
	for typ, untrusted := range want {
		if got := lookupDef(t, reg, typ).UntrustedOutput; got != untrusted {
			t.Errorf("%s UntrustedOutput = %v, want %v", typ, got, untrusted)
		}
		// The flag has to reach the lint: a trigger -> sink flow warns exactly for the untrusted ones.
		b := newFlow("Lint")
		hook := b.node("hook", typ, nil)
		sink := b.node("shell", "test.sink", map[string]any{"command": "{{hook.data.x}}"})
		b.edge(hook, PortOut, sink)
		issues := LintUntrustedData(b.build(), reg)
		if untrusted != (len(issues) == 1 && issues[0].Code == IssueUntrustedData) || len(issues) > 1 {
			t.Errorf("%s: lint issues = %+v, want a warning exactly when untrusted (%v)", typ, issues, untrusted)
		}
	}
}

func TestBindDateTimeYearlyKeepsFeb29(t *testing.T) {
	reg := triggerRegistry(t)
	cases := []struct {
		name string
		at   string
		now  time.Time
		want time.Time
	}{
		{"leap date in the past waits for the next leap year", "2024-02-29 09:00", triggerNow, utcTime(2028, 2, 29, 9, 0)},
		{"leap date in the future is kept", "2028-02-29 09:00", triggerNow, utcTime(2028, 2, 29, 9, 0)},
		{"leap day, one second before the time", "2028-02-29 09:00", utcTime(2028, 2, 29, 8, 59), utcTime(2028, 2, 29, 9, 0)},
		{"leap day, exactly at the time", "2024-02-29 09:00", utcTime(2028, 2, 29, 9, 0), utcTime(2032, 2, 29, 9, 0)},
		{"leap day, after the time", "2024-02-29 09:00", utcTime(2028, 2, 29, 9, 1), utcTime(2032, 2, 29, 9, 0)},
		{"leap day, day after", "2024-02-29 09:00", utcTime(2028, 3, 1, 0, 0), utcTime(2032, 2, 29, 9, 0)},
		{"2100 is no leap year", "2096-02-29 09:00", utcTime(2096, 3, 1, 0, 0), utcTime(2104, 2, 29, 9, 0)},
		{"other dates still move one year", "2026-03-01 09:00", triggerNow, utcTime(2027, 3, 1, 9, 0)},
		{"Feb 28 is not a leap date", "2024-02-28 09:00", triggerNow, utcTime(2027, 2, 28, 9, 0)},
		{"Mar 1 after a leap year", "2024-03-01 09:00", triggerNow, utcTime(2027, 3, 1, 9, 0)},
		{"Dec 31", "2025-12-31 23:59", triggerNow, utcTime(2026, 12, 31, 23, 59)},
		{"date in the future is kept", "2030-05-05 05:05", triggerNow, utcTime(2030, 5, 5, 5, 5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := datetimeBinding(t, reg, time.UTC, tc.now, map[string]any{"at": tc.at, "repeat": "yearly"})
			if err != nil || got.Kind != BindingTimer || got.Repeat != RepeatYearly || !got.FireAt.Equal(tc.want) {
				t.Fatalf("binding = %+v, %v; want a yearly timer at %v", got, err, tc.want)
			}
		})
	}
}

// Arming a binding and the timer service moving a fired timer forward must land on
// the same date, whichever of the two runs first.
func TestBindDateTimeYearlyAgreesWithTimerRearm(t *testing.T) {
	reg := triggerRegistry(t)
	params := map[string]any{"at": "2024-02-29 09:00", "repeat": "yearly"}
	armed, err := datetimeBinding(t, reg, time.UTC, triggerNow, params)
	if err != nil || !armed.FireAt.Equal(utcTime(2028, 2, 29, 9, 0)) {
		t.Fatalf("armed = %+v, %v", armed, err)
	}
	fireAt := armed.FireAt
	for _, year := range []int{2032, 2036, 2040} {
		// The timer service settles a fired timer with nextYearly(FireAt, now).
		moved := nextYearly(fireAt, fireAt.Add(5*time.Second))
		if !moved.Equal(utcTime(year, 2, 29, 9, 0)) {
			t.Fatalf("timer moved %v to %v, want Feb 29 %d", fireAt, moved, year)
		}
		// Republishing the flow right after the timer fired arms the same date.
		rearmed, err := datetimeBinding(t, reg, time.UTC, fireAt.Add(5*time.Second), params)
		if err != nil || !rearmed.FireAt.Equal(moved) {
			t.Fatalf("republished flow armed %v, %v; the timer moves to %v", rearmed.FireAt, err, moved)
		}
		fireAt = moved
	}
}

func TestBindDateTimeYearlyKeepsZone(t *testing.T) {
	reg := triggerRegistry(t)
	plus1 := time.FixedZone("UTC+1", 3600)
	got, err := datetimeBinding(t, reg, plus1, triggerNow, map[string]any{"at": "2024-02-29 09:00", "repeat": "yearly"})
	if err != nil || !got.FireAt.Equal(utcTime(2028, 2, 29, 8, 0)) || got.FireAt.Location() != plus1 {
		t.Fatalf("binding = %+v (%v), %v; want 2028-02-29 09:00 in UTC+1", got, got.FireAt.Location(), err)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	// A summer date keeps its local time of day across the year.
	got, err = datetimeBinding(t, reg, berlin, triggerNow, map[string]any{"at": "2026-07-01 09:00", "repeat": "yearly"})
	if err != nil || !got.FireAt.Equal(time.Date(2027, 7, 1, 9, 0, 0, 0, berlin)) {
		t.Fatalf("Berlin binding = %+v, %v", got, err)
	}
}

func TestBindDateTimeOneOff(t *testing.T) {
	reg := triggerRegistry(t)
	got, err := datetimeBinding(t, reg, time.UTC, triggerNow, map[string]any{"at": "2028-02-29 09:00"})
	if err != nil || got.Repeat != "" || !got.FireAt.Equal(utcTime(2028, 2, 29, 9, 0)) {
		t.Fatalf("one-off leap day = %+v, %v", got, err)
	}
	for _, params := range []map[string]any{
		{"at": "2024-02-29 09:00"},
		{"at": "2024-02-29 09:00", "repeat": "none"},
		{"at": triggerNow.Format("2006-01-02 15:04:05")}, // exactly now is not in the future
	} {
		if _, err := datetimeBinding(t, reg, time.UTC, triggerNow, params); err == nil || !strings.Contains(err.Error(), "in the past") {
			t.Errorf("params %v: err = %v, want a past-time error", params, err)
		}
	}
	for _, at := range []any{nil, "", "   "} {
		if _, err := datetimeBinding(t, reg, time.UTC, triggerNow, map[string]any{"at": at}); !errors.Is(err, errParamMissing) {
			t.Errorf("at %#v: err = %v, want a missing value", at, err)
		}
	}
}

// Error text that repeats user input is cut and quoted: a huge or hostile value
// cannot flood run records, break a log line or leave invalid UTF-8.
func TestTriggerErrorEchoesAreBounded(t *testing.T) {
	reg := triggerRegistry(t)
	huge := strings.Repeat("é€x", 40000)
	hostile := "07:00\nFAKE LOG LINE\r\x00\xff"
	type tc struct {
		name   string
		typ    string
		params map[string]any
	}
	var cases []tc
	for _, value := range []string{huge, hostile, huge + "\xff"} {
		cases = append(cases,
			tc{"daily time", TypeTriggerSchedule, map[string]any{"mode": "daily", "time": value}},
			tc{"weekly time", TypeTriggerSchedule, map[string]any{"mode": "weekly", "time": value, "weekdays": []any{"mon"}}},
			tc{"schedule mode", TypeTriggerSchedule, map[string]any{"mode": value}},
			tc{"weekday name", TypeTriggerSchedule, map[string]any{"mode": "weekly", "weekdays": []any{"mon", value}}},
			tc{"datetime at", TypeTriggerDateTime, map[string]any{"at": value}},
			tc{"datetime repeat", TypeTriggerDateTime, map[string]any{"at": "2027-01-01 10:00", "repeat": value}},
			tc{"device event", TypeTriggerDevice, map[string]any{"event": value}},
			tc{"call type", TypeTriggerFritzBox, map[string]any{"call_type": value}},
			tc{"planner event", TypeTriggerPlanner, map[string]any{"event": value}},
			tc{"budget event", TypeTriggerBudget, map[string]any{"event": value}},
		)
	}
	cases = append(cases,
		tc{"cron length", TypeTriggerSchedule, map[string]any{"mode": "cron", "cron": huge}},
		tc{"cron parser message", TypeTriggerSchedule, map[string]any{"mode": "cron", "cron": strings.Repeat("é", 90) + " b c d e"}},
		tc{"cron hostile", TypeTriggerSchedule, map[string]any{"mode": "cron", "cron": "0 7 * * \nFAKE\r\x00\xff x"}},
		tc{"interval step", TypeTriggerSchedule, map[string]any{"mode": "interval_minutes", "minutes": huge}},
		tc{"min interval", TypeTriggerWebhook, map[string]any{"webhook": "wh", "min_interval_seconds": huge}},
		tc{"weekdays text", TypeTriggerSchedule, map[string]any{"mode": "weekly", "weekdays": huge}},
		tc{"weekdays list of lists", TypeTriggerSchedule, map[string]any{"mode": "weekly", "weekdays": []any{[]any{huge}}}},
		tc{"manual data", TypeTriggerManual, map[string]any{"data": huge}},
	)
	for _, c := range cases {
		b := newFlow("Echo")
		node := b.node("trg", c.typ, c.params)
		f := b.build()
		_, err := BindTriggers(f, reg, time.UTC, triggerNow)
		if err == nil {
			t.Errorf("%s: no error for %.30q...", c.name, fmt.Sprint(c.params))
			continue
		}
		msg := err.Error()
		if len(msg) > maxEchoMessageBytes || !utf8.ValidString(msg) || strings.ContainsAny(msg, "\n\r\x00") {
			t.Errorf("%s: error text is not bounded, quoted and valid: %d bytes, %.200q", c.name, len(msg), msg)
		}
		issues := lookupDef(t, reg, c.typ).Validate(f.NodeByID(node), ValidateContext{Mode: ModePublish, Now: triggerNow, Location: time.UTC})
		if len(issues) != 1 || len(issues[0].Message) > maxEchoMessageBytes || !utf8.ValidString(issues[0].Message) {
			t.Errorf("%s: issues = %+v", c.name, issues)
		}
	}

	// The node key in the BindTriggers wrapper is cut as well.
	b := newFlow("Key")
	b.node("trg", TypeTriggerDevice, map[string]any{"event": "bogus"})
	f := b.build()
	f.Nodes[0].Key = huge
	_, err := BindTriggers(f, reg, time.UTC, triggerNow)
	if err == nil || len(err.Error()) > maxEchoMessageBytes+100 || !utf8.ValidString(err.Error()) {
		t.Errorf("long node key: error = %v", err)
	}
}

func oddParamValues() []struct {
	name string
	v    any
} {
	huge := strings.Repeat("x", 1<<20)
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	deep := any("leaf")
	for i := 0; i < 200; i++ {
		deep = []any{deep}
	}
	return []struct {
		name string
		v    any
	}{
		{"nil", nil}, {"true", true}, {"false", false},
		{"zero", 0.0}, {"minus one", -1.0}, {"fraction", 2.5}, {"1e300", 1e300}, {"-1e300", -1e300}, {"2^63", 9.3e18},
		{"NaN", math.NaN()}, {"+Inf", math.Inf(1)}, {"-Inf", math.Inf(-1)},
		{"int", 7}, {"int64", int64(1) << 62}, {"min int", math.MinInt}, {"float32", float32(1.5)},
		{"json.Number", json.Number("12")}, {"bad json.Number", json.Number("1e999")}, {"garbage json.Number", json.Number("x")},
		{"empty", ""}, {"blank", " \t "}, {"text", "x"}, {"clock", "07:00"}, {"mon", "mon"}, {"yearly", "yearly"},
		{"date", "2027-01-01 10:00"}, {"huge", huge}, {"huge invalid utf8", huge + "\xff"}, {"invalid utf8", "\xff\xfe"},
		{"empty list", []any{}}, {"nil list", []any(nil)}, {"null entry", []any{nil}},
		{"mixed list", []any{1.0, "mon", true, nil, map[string]any{}, []any{}}},
		{"huge entry", []any{huge}}, {"weekday list", []any{"mon", "TUE", "sun"}}, {"deep list", deep},
		{"string slice", []string{"mon"}}, {"empty map", map[string]any{}}, {"nil map", map[string]any(nil)},
		{"nested map", map[string]any{"a": []any{map[string]any{"b": nil}}}}, {"cyclic map", cyclic},
		{"struct", struct{}{}}, {"time", triggerNow}, {"func", func() {}}, {"chan", make(chan int)},
		{"node pointer", &Node{}}, {"nil pointer", (*int)(nil)},
	}
}

func oddParamNames(def *NodeDef) []string {
	names := []string{"min_interval_seconds", "at", "repeat", "time", "weekdays", "mode", "day", "cron", "minutes", "hours",
		"event", "call_type", "data", "webhook", "topic", "entity", "source", "require_success", "unknown_param"}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	for _, p := range def.Params {
		if !seen[p.Name] {
			names = append(names, p.Name)
			seen[p.Name] = true
		}
	}
	return names
}

func oddParamBases(typ string) []map[string]any {
	switch typ {
	case TypeTriggerSchedule:
		bases := []map[string]any{nil}
		for _, mode := range []string{"interval_minutes", "interval_hours", "daily", "weekdays", "weekly", "monthly", "cron", "hourly"} {
			bases = append(bases, map[string]any{"mode": mode})
		}
		return bases
	case TypeTriggerDateTime:
		return []map[string]any{nil, {"at": "2027-01-01 10:00"}, {"at": "2024-02-29 10:00", "repeat": "yearly"}}
	}
	return []map[string]any{nil}
}

// Validate and the bind functions run on raw nodes of unfinished drafts: whatever a
// parameter holds, they must neither panic nor echo it unbounded, and Validate must
// agree with the binding.
func TestTriggerHooksSurviveOddParams(t *testing.T) {
	reg := triggerRegistry(t)
	table := triggerTypes()
	if len(table) != 13 {
		t.Fatalf("%d trigger types, want 13", len(table))
	}
	vc := ValidateContext{Mode: ModePublish, Now: triggerNow, Location: time.UTC}
	values := oddParamValues()
	var failures []string
	fail := func(format string, args ...any) { failures = append(failures, fmt.Sprintf(format, args...)) }
	runs := 0

	check := func(def *NodeDef, tt triggerType, params map[string]any, label string) {
		runs++
		node := &Node{ID: testNodeID(1), Key: "trg", Type: def.Type, Params: params}
		flow := &Flow{Schema: SchemaVersion, ID: "flow_test", Kind: KindFlow, Name: "odd", Nodes: []Node{*node}}
		var (
			issues  []Issue
			binding TriggerBinding
			bindErr error
			listed  []TriggerBinding
			listErr error
			sample  map[string]any
		)
		for _, step := range []struct {
			name string
			fn   func()
		}{
			{"Validate", func() { issues = def.Validate(node, vc) }},
			{"bind", func() { binding, bindErr = tt.bind(node, time.UTC, triggerNow) }},
			{"BindTriggers", func() { listed, listErr = BindTriggers(flow, reg, time.UTC, triggerNow) }},
			{"TriggerSample", func() { sample = TriggerSample(node) }},
		} {
			if p := catchPanic(step.fn); p != nil {
				fail("%s: %s panicked: %v", label, step.name, p)
				return
			}
		}
		invalid := bindErr != nil && !errors.Is(bindErr, errParamMissing)
		if invalid != (len(issues) > 0) {
			fail("%s: Validate reported %+v but bind returned %v", label, issues, bindErr)
		}
		if (bindErr != nil) != (listErr != nil) || (bindErr == nil && len(listed) != 1) {
			fail("%s: BindTriggers = %+v, %v but bind returned %v", label, listed, listErr, bindErr)
		}
		for _, is := range issues {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || is.NodeID != node.ID ||
				len(is.Message) > maxEchoMessageBytes || !utf8.ValidString(is.Message) {
				fail("%s: bad issue %.200q", label, fmt.Sprintf("%+v", is))
			}
		}
		for _, err := range []error{bindErr, listErr} {
			if err != nil && (len(err.Error()) > maxEchoMessageBytes+100 || !utf8.ValidString(err.Error())) {
				fail("%s: error text of %d bytes or invalid UTF-8: %.100q", label, len(err.Error()), err.Error())
			}
		}
		if bindErr == nil {
			if _, err := json.Marshal(binding); err != nil {
				fail("%s: binding is not JSON encodable: %v", label, err)
			}
			if binding.NodeID != node.ID || binding.Kind == "" || len(binding.Schedule) > maxCronBytes {
				fail("%s: odd binding %+v", label, binding)
			}
		}
		if _, err := json.Marshal(sample); err != nil || sample == nil {
			fail("%s: sample = %v, %v", label, sample, err)
		}
	}

	for _, def := range reg.All() {
		tt, ok := table[def.Type]
		if !ok {
			continue // a logic node
		}
		names := oddParamNames(def)
		for _, base := range oddParamBases(def.Type) {
			for _, ov := range values {
				for _, name := range names {
					params := make(map[string]any, len(base)+1)
					for k, v := range base {
						params[k] = v
					}
					params[name] = ov.v
					check(def, tt, params, fmt.Sprintf("%s %v + %s=%s", def.Type, base, name, ov.name))
				}
				// Every parameter holding the odd value at once.
				all := make(map[string]any, len(names))
				for _, name := range names {
					all[name] = ov.v
				}
				check(def, tt, all, fmt.Sprintf("%s all=%s", def.Type, ov.name))
			}
		}
		check(def, tt, nil, def.Type+" nil params")
	}
	if runs < 5000 {
		t.Fatalf("only %d combinations ran", runs)
	}
	if len(failures) > 0 {
		if len(failures) > 15 {
			failures = append(failures[:15], fmt.Sprintf("... and %d more", len(failures)-15))
		}
		t.Fatalf("%d of %d combinations failed:\n%s", len(failures), runs, strings.Join(failures, "\n"))
	}
}

func TestBindTriggersNilInputs(t *testing.T) {
	reg := triggerRegistry(t)
	if got, err := BindTriggers(nil, reg, time.UTC, triggerNow); got != nil || err != nil {
		t.Errorf("nil flow = %+v, %v; want no bindings", got, err)
	}
	b := newFlow("Nil")
	b.node("manual", TypeTriggerManual, nil)
	if _, err := BindTriggers(b.build(), nil, time.UTC, triggerNow); err == nil {
		t.Error("a nil registry must be an error, not a panic")
	}
	got, err := BindTriggers(b.build(), reg, nil, time.Time{})
	if err != nil || len(got) != 1 {
		t.Errorf("nil location and zero time = %+v, %v", got, err)
	}
	if got := TriggerSample(nil); got == nil || len(got) != 0 {
		t.Errorf("TriggerSample(nil) = %#v, want an empty map", got)
	}
}

func TestTriggerSampleIsADeepCopy(t *testing.T) {
	data := map[string]any{"list": []any{map[string]any{"a": 1.0}}}
	n := &Node{Type: TypeTriggerManual, Params: map[string]any{"data": data}}
	s := TriggerSample(n)
	s["list"].([]any)[0].(map[string]any)["a"] = 2.0
	if data["list"].([]any)[0].(map[string]any)["a"] != 1.0 {
		t.Fatal("changing the sample changed the node's data")
	}
	for _, bad := range []any{"text", 1.0, []any{1.0}, math.NaN(), func() {}} {
		if got := TriggerSample(&Node{Type: TypeTriggerManual, Params: map[string]any{"data": bad}}); len(got) != 0 {
			t.Errorf("sample of data %#v = %#v, want an empty map", bad, got)
		}
	}
}

func TestTriggerBindingJSON(t *testing.T) {
	data, err := json.Marshal(TriggerBinding{NodeID: "n", Kind: BindingManual})
	if err != nil || string(data) != `{"node_id":"n","kind":"manual"}` {
		t.Fatalf("manual binding = %s, %v: a zero fire_at must be omitted", data, err)
	}
	data, err = json.Marshal(TriggerBinding{NodeID: "n", Kind: BindingTimer, FireAt: utcTime(2027, 3, 1, 9, 0), Repeat: RepeatYearly})
	if err != nil || string(data) != `{"node_id":"n","kind":"timer","fire_at":"2027-03-01T09:00:00Z","repeat":"yearly"}` {
		t.Fatalf("timer binding = %s, %v", data, err)
	}
}

func TestNormalizeTriggerDataLimits(t *testing.T) {
	t.Run("text at the cap is parsed", func(t *testing.T) {
		body := `"` + strings.Repeat("a", maxTriggerRawBytes-2) + `"`
		if len(body) != maxTriggerRawBytes {
			t.Fatalf("test body has %d bytes", len(body))
		}
		got := NormalizeTriggerData("webhook", body)
		if _, truncated := got["truncated"]; truncated || got["raw"] != body || got["payload"] != strings.Repeat("a", maxTriggerRawBytes-2) {
			t.Fatalf("a body of exactly the cap was not parsed: truncated=%v raw=%d bytes", truncated, len(got["raw"].(string)))
		}
	})

	t.Run("text over the cap is cut and not parsed", func(t *testing.T) {
		body := strings.Repeat("a", maxTriggerRawBytes+1)
		got := NormalizeTriggerData("webhook", body)
		payload, hasPayload := got["payload"]
		if len(got) != 3 || got["truncated"] != true || !hasPayload || payload != nil || got["raw"] != body[:triggerRawKeepBytes] {
			t.Fatalf("webhook = %d keys, truncated=%v, payload=%v/%v, raw %d bytes", len(got), got["truncated"], payload, hasPayload, len(got["raw"].(string)))
		}
		for _, kind := range []string{"email", "mqtt_message", "api", ""} {
			got := NormalizeTriggerData(kind, body)
			if len(got) != 2 || got["truncated"] != true || got["raw"] != body[:triggerRawKeepBytes] {
				t.Errorf("kind %q: %d keys, truncated=%v, raw %d bytes", kind, len(got), got["truncated"], len(got["raw"].(string)))
			}
		}
		// A body that would parse as an object is not parsed either.
		obj := `{"a":"` + strings.Repeat("b", maxTriggerRawBytes) + `"}`
		if got := NormalizeTriggerData("email", obj); got["truncated"] != true || got["a"] != nil {
			t.Errorf("an object over the cap was parsed: %d keys", len(got))
		}
		if _, err := json.Marshal(got); err != nil {
			t.Errorf("the result is not JSON encodable: %v", err)
		}
	})

	t.Run("the cut keeps whole characters", func(t *testing.T) {
		for _, char := range []string{"€", "é", "😀"} {
			body := strings.Repeat(char, maxTriggerRawBytes/len(char)+1)
			raw := NormalizeTriggerData("webhook", body)["raw"].(string)
			if !utf8.ValidString(raw) || len(raw) > triggerRawKeepBytes || len(raw) < triggerRawKeepBytes-len(char) ||
				!strings.HasPrefix(body, raw) || strings.ContainsRune(raw, utf8.RuneError) {
				t.Errorf("%q: raw has %d bytes, valid=%v", char, len(raw), utf8.ValidString(raw))
			}
		}
		raw := NormalizeTriggerData("webhook", "\xff"+strings.Repeat("a", maxTriggerRawBytes))["raw"].(string)
		if !utf8.ValidString(raw) || !strings.HasPrefix(raw, "�a") {
			t.Errorf("invalid bytes in the kept part: %.20q", raw)
		}
		if got := NormalizeTriggerData("webhook", strings.Repeat(" ", maxTriggerRawBytes+1)); got["raw"] != "" || got["truncated"] != true {
			t.Errorf("blank text over the cap = %#v", got)
		}
	})

	t.Run("invalid UTF-8 is replaced", func(t *testing.T) {
		got := NormalizeTriggerData("webhook", "a\xffb")
		if got["raw"] != "a�b" || got["payload"] != "a�b" {
			t.Errorf("webhook text = %#v", got)
		}
		got = NormalizeTriggerData("webhook", "{\"k\":\"v\xff\"}")
		if !utf8.ValidString(got["raw"].(string)) || got["payload"].(map[string]any)["k"] != "v�" {
			t.Errorf("webhook json = %#v", got)
		}
		got = NormalizeTriggerData("mqtt", "{\"topic\":\"t\xff\",\"payload\":\"{\\\"x\\\":1}\"}")
		if got["topic"] != "t�" || !reflect.DeepEqual(got["json"], map[string]any{"x": 1.0}) {
			t.Errorf("mqtt = %#v", got)
		}
		if got := NormalizeTriggerData("api", "not json \xff"); got["raw"] != "not json �" {
			t.Errorf("api text = %#v", got)
		}
	})

	t.Run("nesting beyond the parser limit falls back to text", func(t *testing.T) {
		body := strings.Repeat("[", 20000) + strings.Repeat("]", 20000)
		got := NormalizeTriggerData("webhook", body)
		if got["raw"] != body || got["payload"] != body {
			t.Errorf("deeply nested body: payload is %T", got["payload"])
		}
		if got := NormalizeTriggerData("email", body); got["raw"] != body || len(got) != 1 {
			t.Errorf("deeply nested object text = %#v", got)
		}
	})

	t.Run("JSON null and blank text", func(t *testing.T) {
		if got := NormalizeTriggerData("webhook", "null"); got["raw"] != "null" || got["payload"] != nil || len(got) != 2 {
			t.Errorf("null body = %#v", got)
		}
		if got := NormalizeTriggerData("email", "null"); !reflect.DeepEqual(got, map[string]any{"raw": "null"}) {
			t.Errorf("null object = %#v", got)
		}
		for _, kind := range []string{"webhook", "mqtt", "email", ""} {
			if got := NormalizeTriggerData(kind, " \n\t "); len(got) != 0 {
				t.Errorf("kind %q blank = %#v", kind, got)
			}
		}
	})
}
