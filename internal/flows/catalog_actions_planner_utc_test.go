package flows

import (
	"fmt"
	"testing"
	"time"
)

// The planner stores a time string as it is and compares it as TEXT in SQL with
// time.Now().UTC().Format(RFC3339) (GetDueNotifications, AutoExpireAppointments). So the
// nodes send every time in UTC, in the form with "Z", whatever zone the run is in.

// plannerRunIn executes a planner node in loc and returns what the tool got and what the
// node returned.
func plannerRunIn(t *testing.T, typ string, over map[string]any, loc *time.Location) (args, output map[string]any) {
	t.Helper()
	tools := &fakeTools{respond: homeAnswers}
	res, err := execDef(lookupDef(t, homeRegistry(t), typ), homeParams(typ, over), &Services{Tools: tools, Location: loc})
	if err != nil {
		t.Fatalf("%s %v in %v: %v", typ, over, loc, err)
	}
	return tools.last(t).Args, res.Output
}

func TestPlannerSendsUTC(t *testing.T) {
	for _, c := range []struct {
		zone                            *time.Location
		datetime, notification, dueDate string // what the tool gets
		outputTime                      string // what the node returns
	}{
		{time.FixedZone("plus", 5*3600+1800), "2026-10-05T03:30:00Z", "2026-10-05T03:00:00Z", "2026-10-05T18:30:00Z", "2026-10-05T09:00:00+05:30"},
		{time.FixedZone("minus", -8*3600), "2026-10-05T17:00:00Z", "2026-10-05T16:30:00Z", "2026-10-06T08:00:00Z", "2026-10-05T09:00:00-08:00"},
		{time.UTC, "2026-10-05T09:00:00Z", "2026-10-05T08:30:00Z", "2026-10-06T00:00:00Z", "2026-10-05T09:00:00Z"},
	} {
		args, out := plannerRunIn(t, TypeAppointmentAdd, map[string]any{"date_time": "2026-10-05 09:00", "remind_minutes": 30.0}, c.zone)
		if args["date_time"] != c.datetime || args["notification_at"] != c.notification || out["date_time"] != c.outputTime {
			t.Errorf("%v appointment: args %v, output %v", c.zone, args, out["date_time"])
		}
		args, _ = plannerRunIn(t, TypeTodoAdd, map[string]any{"due_date": "2026-10-06"}, c.zone)
		if args["due_date"] != c.dueDate {
			t.Errorf("%v todo: args %v", c.zone, args)
		}
		// The three are in the form with "Z", and the same instants as the zone's.
		for _, key := range []string{"date_time", "notification_at", "due_date"} {
			s, has := args[key].(string)
			if has && (len(s) == 0 || s[len(s)-1] != 'Z') {
				t.Errorf("%v: %s = %q is not UTC", c.zone, key, s)
			}
		}
	}
	// A date written with its own offset is converted as well.
	args, out := plannerRunIn(t, TypeAppointmentAdd, map[string]any{"date_time": "2026-10-05T09:00:00+02:00", "remind_minutes": 90.0}, time.UTC)
	if args["date_time"] != "2026-10-05T07:00:00Z" || args["notification_at"] != "2026-10-05T05:30:00Z" || out["date_time"] != "2026-10-05T09:00:00+02:00" {
		t.Errorf("a date with an offset: %v, output %v", args, out["date_time"])
	}
}

// What the planner's SQL does with the strings: a reminder is due when
// notification_at <= now as text. With UTC strings that is the right instant; a local
// string ("...+02:00") would be compared by its local digits and fire two hours late.
func TestPlannerTimesCompareRightAsText(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	args, _ := plannerRunIn(t, TypeAppointmentAdd, map[string]any{"date_time": "2026-10-05 09:00", "remind_minutes": 30.0}, cest)
	reminder, _ := args["notification_at"].(string)
	if reminder != "2026-10-05T06:30:00Z" {
		t.Fatalf("notification_at = %q", reminder)
	}
	for _, c := range []struct {
		now string
		due bool
	}{{"2026-10-05T06:29:59Z", false}, {"2026-10-05T06:30:00Z", true}, {"2026-10-05T06:45:00Z", true}, {"2026-10-05T08:30:00Z", true}} {
		if got := reminder <= c.now; got != c.due {
			t.Errorf("at %s: text comparison says due=%v, want %v", c.now, got, c.due)
		}
	}
	// The form the first version sent, for the record: due 08:30 "UTC" instead of 06:30.
	if local := "2026-10-05T08:30:00+02:00"; local <= "2026-10-05T06:45:00Z" {
		t.Error("the local form compares right by accident; the test no longer shows the bug")
	}
}

// Berlin across the change to summer time (2026-03-29, 02:00 becomes 03:00) and the end
// of it (2026-10-25): the elapsed time of a reminder and the UTC form of dates and
// midnights are right on both sides.
func TestPlannerBerlinAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	for _, c := range []struct {
		date            string
		remind          float64
		wantTime        string
		wantReminder    string // "" for none
		wantOutputLocal string
	}{
		// 09:00 summer time, a day (24 elapsed hours, 25 on the local clock) before the change.
		{"2026-03-29 09:00", 1440, "2026-03-29T07:00:00Z", "2026-03-28T07:00:00Z", "2026-03-29T09:00:00+02:00"},
		// Winter time, the evening before; an hour's reminder.
		{"2026-03-28 23:30", 60, "2026-03-28T22:30:00Z", "2026-03-28T21:30:00Z", "2026-03-28T23:30:00+01:00"},
		// Summer time, 03:30; a two hour reminder reaches over the change.
		{"2026-03-29 03:30", 120, "2026-03-29T01:30:00Z", "2026-03-28T23:30:00Z", "2026-03-29T03:30:00+02:00"},
		// 02:30 does not exist on that day, it is read as 03:30 summer time.
		{"2026-03-29 02:30", 0, "2026-03-29T01:30:00Z", "", "2026-03-29T03:30:00+02:00"},
		// Winter time again, the day after the end of summer time.
		{"2026-10-26 09:00", 30, "2026-10-26T08:00:00Z", "2026-10-26T07:30:00Z", "2026-10-26T09:00:00+01:00"},
		// The end of summer time itself (03:00 becomes 02:00): 12:00 is winter time, a day of 25 hours.
		{"2026-10-25 12:00", 1440, "2026-10-25T11:00:00Z", "2026-10-24T11:00:00Z", "2026-10-25T12:00:00+01:00"},
	} {
		over := map[string]any{"date_time": c.date, "remind_minutes": c.remind}
		args, out := plannerRunIn(t, TypeAppointmentAdd, over, berlin)
		got, has := args["notification_at"].(string)
		if args["date_time"] != c.wantTime || got != c.wantReminder || has != (c.wantReminder != "") || out["date_time"] != c.wantOutputLocal {
			t.Errorf("%s remind %v: tool %v, output %v", c.date, c.remind, args, out["date_time"])
		}
	}
	for _, c := range []struct{ due, want string }{
		{"2026-03-28", "2026-03-27T23:00:00Z"}, // midnight, winter time
		{"2026-03-29", "2026-03-28T23:00:00Z"}, // midnight, before the change at 02:00
		{"2026-03-30", "2026-03-29T22:00:00Z"}, // midnight, summer time
		{"2026-03-29 03:30", "2026-03-29T01:30:00Z"},
		{"2026-10-24", "2026-10-23T22:00:00Z"},
		{"2026-10-25", "2026-10-24T22:00:00Z"}, // midnight, still summer time
		{"2026-10-26", "2026-10-25T23:00:00Z"}, // midnight, winter time
	} {
		args, _ := plannerRunIn(t, TypeTodoAdd, map[string]any{"due_date": c.due}, berlin)
		if args["due_date"] != c.want {
			t.Errorf("due_date %s: tool got %v, want %s", c.due, args["due_date"], c.want)
		}
	}
	// Whatever the zone, every time that reaches the tool is UTC.
	for _, date := range []string{"2026-03-29 02:30", "2026-03-29 03:00", "2026-10-25 02:30", "2026-10-25 03:00"} {
		args, _ := plannerRunIn(t, TypeAppointmentAdd, map[string]any{"date_time": date, "remind_minutes": 45.0}, berlin)
		for _, key := range []string{"date_time", "notification_at"} {
			if s, _ := args[key].(string); len(s) < 20 || s[len(s)-1] != 'Z' {
				t.Errorf("%s: %s = %q", date, key, fmt.Sprint(args[key]))
			}
		}
	}
}
