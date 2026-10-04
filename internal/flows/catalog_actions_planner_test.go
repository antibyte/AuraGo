package flows

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPlannerTitleIsValidated(t *testing.T) {
	long := strings.Repeat("ä", maxPlannerTitleRunes+1)
	for _, typ := range []string{TypeAppointmentAdd, TypeTodoAdd} {
		for _, c := range []struct {
			name string
			v    any
		}{
			{"nil", nil}, {"empty", ""}, {"blank", " \t "}, {"line feed", "a" + homeLF + "b"}, {"carriage return", "a" + homeCR + "b"}, {"tab", "a\tb"},
			{"nul", "a" + homeNUL}, {"invalid utf-8", "a" + homeBad}, {"too long", long}, {"list", []any{"t"}}, {"object", map[string]any{"title": "t"}},
		} {
			tools, _, err := homeRun(t, typ, map[string]any{"title": c.v})
			homeWantInvalid(t, typ+" "+c.name, tools, err, homeEchoOf(c.v))
		}
		atLimit := strings.Repeat("ä", maxPlannerTitleRunes)
		for title, want := range map[string]string{"  Zahnarzt \n": "Zahnarzt", atLimit: atLimit, "Müll rausbringen": "Müll rausbringen"} {
			tools, res, err := homeRun(t, typ, map[string]any{"title": title})
			if err != nil || tools.last(t).Args["title"] != want || res.Output["title"] != want {
				t.Errorf("%s title %.20q: %v", typ, title, err)
			}
		}
		// A number is text of its own form.
		if tools, _, err := homeRun(t, typ, map[string]any{"title": 7.0}); err != nil || tools.last(t).Args["title"] != "7" {
			t.Errorf("%s numeric title: %v", typ, err)
		}
	}
}

func TestPlannerDescription(t *testing.T) {
	for _, typ := range []string{TypeAppointmentAdd, TypeTodoAdd} {
		for name, v := range map[string]any{"nil": nil, "empty": "", "blank": " \n ", "empty list": []any{}, "empty object": map[string]any{}} {
			tools, _, err := homeRun(t, typ, map[string]any{"description": v})
			if _, has := tools.last(t).Args["description"]; err != nil || has {
				t.Errorf("%s %s: %v, sent %v", typ, name, err, tools.last(t).Args)
			}
		}
		for _, c := range []struct {
			name string
			v    any
			want string
		}{
			{"text", "  Bitte Versichertenkarte mitbringen\n", "Bitte Versichertenkarte mitbringen"}, {"multi-line", "a\nb", "a\nb"}, {"number", 5.0, "5"},
			{"list", []any{"a", 1.0}, `["a",1]`}, {"object", map[string]any{"a": "<b>"}, `{"a":"<b>"}`}, {"broken text is repaired", "a" + homeBad + "b", "a" + replacementRune + "b"},
			{"at the limit", strings.Repeat("d", maxPlannerDescriptionBytes), strings.Repeat("d", maxPlannerDescriptionBytes)},
		} {
			tools, _, err := homeRun(t, typ, map[string]any{"description": c.v})
			if got := tools.last(t).Args["description"]; err != nil || got != c.want {
				t.Errorf("%s %s: %.40q %v", typ, c.name, got, err)
			}
		}
		cyclic := map[string]any{}
		cyclic["self"] = cyclic
		for name, v := range map[string]any{"too long": strings.Repeat("d", maxPlannerDescriptionBytes+1), "cyclic": cyclic, "NaN in a list": []any{math.NaN()}, "func": func() {}} {
			tools, _, err := homeRun(t, typ, map[string]any{"description": v})
			homeWantInvalid(t, typ+" "+name, tools, err, homeEchoOf(v))
		}
	}
}

func TestAppointmentDateAndReminder(t *testing.T) {
	// Times without a zone are read in the run's location (UTC in these tests).
	for in, want := range map[string]string{
		"2026-10-05 09:00": "2026-10-05T09:00:00Z", "2026-10-05T09:00": "2026-10-05T09:00:00Z", "2026-10-05": "2026-10-05T00:00:00Z",
		"2026-10-05T09:00:00+02:00": "2026-10-05T09:00:00+02:00", " 2026-10-05T07:00:00Z ": "2026-10-05T07:00:00Z", "2026-10-05 09:00:30": "2026-10-05T09:00:30Z",
	} {
		tools, res, err := homeRun(t, TypeAppointmentAdd, map[string]any{"date_time": in})
		if err != nil || tools.last(t).Args["date_time"] != want || res.Output["date_time"] != want {
			t.Errorf("date_time %q: %v %v", in, err, tools.last(t).Args)
		}
	}
	for _, c := range []struct {
		name string
		v    any
	}{
		{"nil", nil}, {"empty", ""}, {"blank", " "}, {"words", "bald"}, {"day only without year", "05.10."}, {"month 13", "2026-13-01"}, {"number", 1789000000.0},
		{"bool", true}, {"list", []any{"2026-10-05"}}, {"object", map[string]any{}}, {"long", strings.Repeat("2", maxPlannerTimeBytes+1)}, {"invalid utf-8", "2026-10-05" + homeBad},
		{"nul", "2026-10-05" + homeNUL}, {"huge", strings.Repeat("x", 1<<20)},
	} {
		tools, _, err := homeRun(t, TypeAppointmentAdd, map[string]any{"date_time": c.v})
		homeWantInvalid(t, "date_time "+c.name, tools, err, homeEchoOf(c.v))
	}

	// remind_minutes
	at := "2026-10-05 09:00"
	for _, c := range []struct {
		v    any
		want string // notification_at, "" for none
	}{
		{nil, ""}, {"", ""}, {" ", ""}, {0.0, ""}, {"0", ""}, {30.0, "2026-10-05T08:30:00Z"}, {"30", "2026-10-05T08:30:00Z"}, {int64(60), "2026-10-05T08:00:00Z"},
		{1.5, "2026-10-05T08:58:30Z"}, {1440.0, "2026-10-04T09:00:00Z"}, {float64(maxRemindMinutes), "2025-10-04T09:00:00Z"},
	} {
		tools, _, err := homeRun(t, TypeAppointmentAdd, map[string]any{"date_time": at, "remind_minutes": c.v})
		got, has := tools.last(t).Args["notification_at"]
		if err != nil || (c.want == "") == has || has && got != c.want {
			t.Errorf("remind_minutes %#v: %v, notification_at %v", c.v, err, got)
		}
	}
	for _, v := range []any{-1.0, -0.001, float64(maxRemindMinutes) + 1, 1e300, math.NaN(), math.Inf(1), "abc", "1h", true, []any{30.0}, map[string]any{}} {
		tools, _, err := homeRun(t, TypeAppointmentAdd, map[string]any{"remind_minutes": v})
		homeWantInvalid(t, fmt.Sprintf("remind_minutes %v", v), tools, err, homeEchoOf(v))
	}
	// A reminder that would fall before the year 0 is refused, not sent as a broken date.
	tools, _, err := homeRun(t, TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 00:10", "remind_minutes": 60.0})
	homeWantInvalid(t, "reminder before year 0", tools, err, "")
	if tools, _, err := homeRun(t, TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 01:00", "remind_minutes": 60.0}); err != nil || tools.last(t).Args["notification_at"] != "0000-01-01T00:00:00Z" {
		t.Errorf("a reminder at the start of year 0: %v", err)
	}
	// Only title, date_time, description and notification_at go to the tool: the node offers no agent wake-up.
	tools, _, err = homeRun(t, TypeAppointmentAdd, map[string]any{"description": "d", "remind_minutes": 5.0, "wake_agent": true, "agent_instruction": "do things"})
	want := []string{"date_time", "description", "notification_at", "operation", "title"}
	var keys []string
	for k := range tools.last(t).Args {
		keys = append(keys, k)
	}
	if err != nil || len(keys) != len(want) {
		t.Errorf("tool arguments %v: %v", keys, err)
	}
	for _, k := range want {
		if _, ok := tools.last(t).Args[k]; !ok {
			t.Errorf("argument %s missing", k)
		}
	}
}

func TestTodoPriorityAndDueDate(t *testing.T) {
	for _, c := range []struct {
		v    any
		want string
	}{{nil, "medium"}, {"", "medium"}, {" ", "medium"}, {"low", "low"}, {"HIGH", "high"}, {" Medium ", "medium"}} {
		tools, _, err := homeRun(t, TypeTodoAdd, map[string]any{"priority": c.v})
		if err != nil || tools.last(t).Args["priority"] != c.want {
			t.Errorf("priority %#v: %v %v", c.v, err, tools.last(t).Args["priority"])
		}
	}
	for _, v := range []any{"urgent", "critical", "1", 2.0, true, []any{"low"}, map[string]any{}, "{{x.p}}"} {
		tools, _, err := homeRun(t, TypeTodoAdd, map[string]any{"priority": v})
		homeWantInvalid(t, fmt.Sprintf("priority %v", v), tools, err, homeEchoOf(v))
	}

	for _, v := range []any{nil, "", " "} {
		tools, _, err := homeRun(t, TypeTodoAdd, map[string]any{"due_date": v})
		if _, has := tools.last(t).Args["due_date"]; err != nil || has {
			t.Errorf("due_date %#v: %v %v", v, err, tools.last(t).Args)
		}
	}
	for in, want := range map[string]string{"2026-10-06": "2026-10-06T00:00:00Z", "2026-10-06 18:30": "2026-10-06T18:30:00Z", "2026-10-06T18:30:00-05:00": "2026-10-06T18:30:00-05:00"} {
		tools, _, err := homeRun(t, TypeTodoAdd, map[string]any{"due_date": in})
		if err != nil || tools.last(t).Args["due_date"] != want {
			t.Errorf("due_date %q: %v %v", in, err, tools.last(t).Args["due_date"])
		}
	}
	for _, v := range []any{"morgen", "2026-02-30", 5.0, true, []any{}, map[string]any{}, strings.Repeat("9", 100)} {
		tools, _, err := homeRun(t, TypeTodoAdd, map[string]any{"due_date": v})
		homeWantInvalid(t, fmt.Sprintf("due_date %.20v", v), tools, err, homeEchoOf(v))
	}
}

// The planner tools answer with an id; a refusal (planner disabled, no database) is an
// error answer or plain text, and none of it is an entry that was created.
func TestPlannerAnswers(t *testing.T) {
	for _, typ := range []string{TypeAppointmentAdd, TypeTodoAdd} {
		for name, reply := range map[string]ToolResponse{
			"disabled":         {Output: `Tool Output: {"status":"error","message":"Planner is disabled. Enable tools.planner.enabled in config."}`, Status: "success"},
			"database":         {Output: `Tool Output: {"status":"error","message":"Planner database not available."}`, Status: "success"},
			"invalid date":     {Output: `Tool Output: {"status":"error","message":"invalid date_time format"}`, Status: "success"},
			"plain text":       {Output: "[PERMISSION DENIED] planner is off", Status: "success"},
			"empty":            {Output: "", Status: "success"},
			"no status":        {Output: `Tool Output: {"id":"x","message":"created"}`, Status: "success"},
			"flagged as error": {Output: `Tool Output: {"status":"success","id":"x"}`, Status: "success", IsError: true},
			"huge refusal":     {Output: "[PERMISSION DENIED] " + strings.Repeat("no ", 5000), Status: "success"},
		} {
			tools, res, err := homeRunWith(t, typ, nil, func(ToolRequest) (ToolResponse, error) { return reply, nil })
			ne := asNodeError(err)
			if ne == nil || ne.Code != "FLOW_TOOL_ERROR" || res.Output != nil || tools.count() != 1 || len(ne.Message) > maxEchoMessageBytes {
				t.Errorf("%s %s: %v, output %v", typ, name, err, res.Output)
			}
		}
		for status, code := range map[string]string{"denied": "FLOW_TOOL_DENIED", "policy_denied": "FLOW_TOOL_DENIED", "needs_setup": "FLOW_NODE_UNAVAILABLE"} {
			_, _, err := homeRunWith(t, typ, nil, func(ToolRequest) (ToolResponse, error) {
				return ToolResponse{Output: "planner is off", Status: status}, nil
			})
			if asNodeError(err) == nil || asNodeError(err).Code != code {
				t.Errorf("%s %s: %v, want %s", typ, status, err, code)
			}
		}
		// The id is the tool's text; a success that carries none still created the entry,
		// so it is not failed (a retry would create it again).
		for answer, want := range map[string]any{`{"status":"success","id":"abc"}`: "abc", `{"status":"success"}`: "", `{"status":"success","id":5}`: "", `{"status":"success","id":null}`: ""} {
			_, res, err := homeRunWith(t, typ, nil, homeReply(answer))
			if err != nil || res.Output["id"] != want {
				t.Errorf("%s answer %s: %v %v", typ, answer, res.Output, err)
			}
		}
	}
}

func TestPlannerValidate(t *testing.T) {
	reg := homeRegistry(t)
	check := func(typ string, params map[string]any) []string {
		var got []string
		node := &Node{ID: testNodeID(1), Key: "n", Type: typ, Params: params}
		for _, is := range lookupDef(t, reg, typ).Validate(node, ValidateContext{Mode: ModePublish}) {
			if is.Code != IssueParamInvalid || is.Severity != SeverityError || len(is.Message) > maxEchoMessageBytes || !utf8.ValidString(is.Message) || strings.ContainsAny(is.Message, homeLF+homeCR) {
				t.Errorf("%s: bad issue %+v", typ, is)
			}
			got = append(got, is.Param)
		}
		return got
	}
	for _, c := range []struct {
		name   string
		typ    string
		params map[string]any
		want   []string
	}{
		{"appointment fine", TypeAppointmentAdd, map[string]any{"title": "t", "date_time": "2026-10-05 09:00", "description": "d", "remind_minutes": 15.0}, nil},
		{"appointment templates", TypeAppointmentAdd, map[string]any{"title": "{{x.t}}", "date_time": "{{x.d}}", "description": "{{x.e}}", "remind_minutes": "{{x.r}}"}, nil},
		{"appointment bare", TypeAppointmentAdd, map[string]any{}, nil},
		{"appointment bad", TypeAppointmentAdd, map[string]any{"title": "a" + homeLF, "date_time": "bald", "description": strings.Repeat("d", maxPlannerDescriptionBytes+1), "remind_minutes": -5.0}, []string{"date_time", "description", "remind_minutes"}},
		{"appointment line break in the title", TypeAppointmentAdd, map[string]any{"title": "a" + homeLF + "b"}, []string{"title"}},
		{"appointment wrong types", TypeAppointmentAdd, map[string]any{"title": []any{"t"}, "date_time": []any{"2026-10-05"}, "remind_minutes": "soon"}, []string{"title", "date_time", "remind_minutes"}},
		{"todo fine", TypeTodoAdd, map[string]any{"title": "t", "description": "d", "priority": "high", "due_date": "2026-10-06"}, nil},
		{"todo templates", TypeTodoAdd, map[string]any{"title": "{{x.t}}", "priority": "{{x.p}}", "due_date": "{{x.d}}"}, nil},
		{"todo bad", TypeTodoAdd, map[string]any{"title": strings.Repeat("t", maxPlannerTitleRunes+1), "priority": "urgent", "due_date": "morgen"}, []string{"title", "priority", "due_date"}},
		{"todo empty values are the required check's", TypeTodoAdd, map[string]any{"title": "", "due_date": "", "description": ""}, nil},
	} {
		if got := check(c.typ, c.params); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: issues on %v, want %v", c.name, got, c.want)
		}
	}
}

// The planner nodes have no outward effect and no wake-up of the agent.
func TestPlannerNodesDeclareWhatTheyAre(t *testing.T) {
	reg := homeRegistry(t)
	for _, typ := range []string{TypeAppointmentAdd, TypeTodoAdd} {
		def := lookupDef(t, reg, typ)
		if len(def.EffectsOf(&Node{Params: map[string]any{}})) != 0 || len(def.Effects) != 0 || def.UntrustedOutput || def.PrimaryInput != "title" {
			t.Errorf("%s: effects %v, untrusted %v, primary %q", typ, def.Effects, def.UntrustedOutput, def.PrimaryInput)
		}
		for _, p := range def.Params {
			if p.Name == "wake_agent" || p.Name == "agent_instruction" {
				t.Errorf("%s offers %s", typ, p.Name)
			}
		}
		if raw, err := json.Marshal(def.Params); err != nil || strings.Contains(string(raw), "independent") {
			t.Errorf("%s: %s %v", typ, raw, err)
		}
	}
}
