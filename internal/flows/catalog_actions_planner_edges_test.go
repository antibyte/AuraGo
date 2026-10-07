package flows

import (
	"testing"
	"time"
)

// plannerTryIn executes a planner node in loc and returns the tool arguments (nil when no
// call was made) and the error.
func plannerTryIn(t *testing.T, typ string, over map[string]any, loc *time.Location) (map[string]any, error) {
	t.Helper()
	tools := &fakeTools{respond: homeAnswers}
	_, err := execDef(lookupDef(t, homeRegistry(t), typ), homeParams(typ, over), &Services{Tools: tools, Location: loc})
	if tools.count() == 0 {
		return nil, err
	}
	return tools.last(t).Args, err
}

// The tool gets every time in UTC and reads RFC 3339, which has four digit years. A date
// at the edge of the range, in a zone with an offset, can fall outside it once it is in
// UTC (the local year is fine): it is refused instead of being sent as "-0001-..." or
// "10000-...". The reminder is checked the same way.
func TestPlannerYearEdgesInOtherZones(t *testing.T) {
	plus1 := time.FixedZone("plus1", 3600)
	minus8 := time.FixedZone("minus8", -8*3600)

	type edge struct {
		name string
		typ  string
		over map[string]any
		loc  *time.Location
		key  string // the argument to check
		want string // "" when the node must refuse
	}
	for _, c := range []edge{
		// date_time
		{"year 0, east of UTC, in UTC year -1", TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 00:30"}, plus1, "date_time", ""},
		{"year 0, east of UTC, in UTC year 0", TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 01:00"}, plus1, "date_time", "0000-01-01T00:00:00Z"},
		{"year 9999, west of UTC, in UTC year 10000", TypeAppointmentAdd, map[string]any{"date_time": "9999-12-31 23:30"}, minus8, "date_time", ""},
		{"year 9999, west of UTC, in UTC year 9999", TypeAppointmentAdd, map[string]any{"date_time": "9999-12-31 15:59"}, minus8, "date_time", "9999-12-31T23:59:00Z"},
		{"year 9999, east of UTC", TypeAppointmentAdd, map[string]any{"date_time": "9999-12-31 23:30"}, plus1, "date_time", "9999-12-31T22:30:00Z"},
		{"an offset in the text, below the range", TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01T00:30:00+01:00"}, time.UTC, "date_time", ""},
		{"an offset in the text, above the range", TypeAppointmentAdd, map[string]any{"date_time": "9999-12-31T23:30:00-05:00"}, time.UTC, "date_time", ""},
		// due_date: a date alone is midnight in the zone
		{"due date year 0, east of UTC", TypeTodoAdd, map[string]any{"due_date": "0000-01-01"}, plus1, "due_date", ""},
		{"due date year 0, west of UTC", TypeTodoAdd, map[string]any{"due_date": "0000-01-01"}, minus8, "due_date", "0000-01-01T08:00:00Z"},
		{"due date year 9999, west of UTC", TypeTodoAdd, map[string]any{"due_date": "9999-12-31"}, minus8, "due_date", "9999-12-31T08:00:00Z"},
		{"due date with a time at year 9999, west of UTC", TypeTodoAdd, map[string]any{"due_date": "9999-12-31 23:30"}, minus8, "due_date", ""},
		{"due date year 9999, east of UTC", TypeTodoAdd, map[string]any{"due_date": "9999-12-31"}, plus1, "due_date", "9999-12-30T23:00:00Z"},
		// notification_at: 01:30 at +01:00 is 00:30 UTC; the local reminder time stays in year 0
		// for 45 minutes (00:45 local), while the UTC one is in year -1.
		{"reminder in UTC year -1 though the local year is 0", TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 01:30", "remind_minutes": 45.0}, plus1, "notification_at", ""},
		{"reminder at the start of UTC year 0", TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 01:30", "remind_minutes": 30.0}, plus1, "notification_at", "0000-01-01T00:00:00Z"},
		{"reminder in UTC year 0, west of UTC", TypeAppointmentAdd, map[string]any{"date_time": "0000-01-01 01:30", "remind_minutes": 90.0}, minus8, "notification_at", "0000-01-01T08:00:00Z"},
	} {
		args, err := plannerTryIn(t, c.typ, c.over, c.loc)
		if c.want == "" {
			ne := asNodeError(err)
			if ne == nil || ne.Code != "FLOW_PARAM_INVALID" || args != nil || len(ne.Message) > maxEchoMessageBytes {
				t.Errorf("%s: %v, tool args %v", c.name, err, args)
			}
			continue
		}
		if err != nil || args[c.key] != c.want {
			t.Errorf("%s: %v, %s = %v, want %s", c.name, err, c.key, args[c.key], c.want)
		}
	}
	// A reminder in the middle of year 0 reaches the tool in the form with "Z".
	args, err := plannerTryIn(t, TypeAppointmentAdd, map[string]any{"date_time": "0000-06-01 12:00", "remind_minutes": 60.0}, plus1)
	if err != nil || args["date_time"] != "0000-06-01T11:00:00Z" || args["notification_at"] != "0000-06-01T10:00:00Z" {
		t.Errorf("year 0 in the middle: %v %v", err, args)
	}

	// Validate says the same about a date written out.
	reg := homeRegistry(t)
	for _, c := range []struct {
		typ, param, value string
		loc               *time.Location
		bad               bool
	}{
		{TypeAppointmentAdd, "date_time", "9999-12-31 23:30", minus8, true},
		{TypeAppointmentAdd, "date_time", "9999-12-31 15:59", minus8, false},
		{TypeAppointmentAdd, "date_time", "0000-01-01T00:30:00+01:00", nil, true},
		{TypeTodoAdd, "due_date", "0000-01-01", plus1, true},
		{TypeTodoAdd, "due_date", "0000-01-01", minus8, false},
	} {
		node := &Node{ID: testNodeID(1), Key: "n", Type: c.typ, Params: homeParams(c.typ, map[string]any{c.param: c.value})}
		issues := lookupDef(t, reg, c.typ).Validate(node, ValidateContext{Mode: ModePublish, Location: c.loc})
		if got := len(issues) > 0 && issues[0].Param == c.param; got != c.bad {
			t.Errorf("Validate %s=%q in %v: issues %+v, want bad=%v", c.param, c.value, c.loc, issues, c.bad)
		}
	}
}
