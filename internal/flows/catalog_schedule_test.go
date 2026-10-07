package flows

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/robfig/cron/v3"
)

// testCronParser is AuraGo's parser configuration, written out again on purpose: the
// tests must notice when the production cronParser drifts from it.
var testCronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// starBit is robfig/cron's marker on a field that was written as * (or ?).
const starBit = uint64(1) << 63

func bitsOf(values ...int) uint64 {
	var bits uint64
	for _, v := range values {
		bits |= 1 << uint(v)
	}
	return bits
}

func rangeBits(from, to, step int) uint64 {
	var bits uint64
	for v := from; v <= to; v += step {
		bits |= 1 << uint(v)
	}
	return bits
}

// parseCron parses expr with the reference parser and returns its fields without the
// star marker.
func parseCron(t *testing.T, expr string) cron.SpecSchedule {
	t.Helper()
	sched, err := testCronParser.Parse(expr)
	if err != nil {
		t.Fatalf("the parser rejects %q: %v", expr, err)
	}
	spec, ok := sched.(*cron.SpecSchedule)
	if !ok {
		t.Fatalf("%q is a %T, not a SpecSchedule", expr, sched)
	}
	out := *spec
	out.Second &^= starBit
	out.Minute &^= starBit
	out.Hour &^= starBit
	out.Dom &^= starBit
	out.Month &^= starBit
	out.Dow &^= starBit
	return out
}

func TestScheduleToCronEdgeValues(t *testing.T) {
	// Exactly maxCronBytes long: the cap counts the text as written, blanks included.
	padded := "0" + strings.Repeat(" ", maxCronBytes-8) + "7 * * *"
	if len(padded) != maxCronBytes {
		t.Fatalf("padded expression has %d bytes", len(padded))
	}
	cases := []struct {
		name    string
		p       map[string]any
		want    string
		bad     bool
		missing bool
	}{
		{"numeric text minutes", map[string]any{"mode": "interval_minutes", "minutes": "5"}, "*/5 * * * *", false, false},
		{"fraction", map[string]any{"mode": "interval_minutes", "minutes": 2.5}, "", true, false},
		{"zero", map[string]any{"mode": "interval_hours", "hours": 0.0}, "", true, false},
		{"huge", map[string]any{"mode": "interval_minutes", "minutes": 1e300}, "", true, false},
		{"negative huge", map[string]any{"mode": "monthly", "time": "07:00", "day": -1e300}, "", true, false},
		{"just over int64", map[string]any{"mode": "monthly", "time": "07:00", "day": 9.3e18}, "", true, false},
		{"bool", map[string]any{"mode": "interval_hours", "hours": true}, "", true, false},
		{"day 31", map[string]any{"mode": "monthly", "time": "07:00", "day": 31.0}, "0 7 31 * *", false, false},
		{"day 32", map[string]any{"mode": "monthly", "time": "07:00", "day": 32.0}, "", true, false},
		{"weekdays in calendar order", map[string]any{"mode": "weekly", "weekdays": []any{"sun", "SAT", "Mon"}}, "0 7 * * 1,6,0", false, false},
		{"weekdays repeated", map[string]any{"mode": "weekly", "weekdays": []any{"mon", "mon"}}, "0 7 * * 1", false, false},
		{"weekdays empty list", map[string]any{"mode": "weekly", "weekdays": []any{}}, "", true, true},
		{"weekdays blank text", map[string]any{"mode": "weekly", "weekdays": " "}, "", true, true},
		{"weekdays text", map[string]any{"mode": "weekly", "weekdays": "mon"}, "", true, false},
		{"weekdays number", map[string]any{"mode": "weekly", "weekdays": 3.0}, "", true, false},
		{"weekdays with a null entry", map[string]any{"mode": "weekly", "weekdays": []any{nil}}, "", true, false},
		{"cron six fields", map[string]any{"mode": "cron", "cron": "0 0 7 * * 1-5"}, "0 0 7 * * 1-5", false, false},
		{"cron blank", map[string]any{"mode": "cron", "cron": "  "}, "", true, true},
		{"cron seven fields", map[string]any{"mode": "cron", "cron": "0 0 7 * * 1-5 2026"}, "", true, false},
		{"cron at the length limit", map[string]any{"mode": "cron", "cron": padded}, "0 7 * * *", false, false},
		{"cron over the length limit", map[string]any{"mode": "cron", "cron": padded[:1] + " " + padded[1:]}, "", true, false},
		{"cron far over the length limit", map[string]any{"mode": "cron", "cron": strings.Repeat("*", maxCronBytes) + " * * * *"}, "", true, false},
		{"time without leading zero", map[string]any{"mode": "daily", "time": "7:00"}, "", true, false},
		{"time 24:00", map[string]any{"mode": "daily", "time": "24:00"}, "", true, false},
		{"nil params", nil, "0 7 * * *", false, false},
	}
	for _, tc := range cases {
		got, err := ScheduleToCron(tc.p)
		switch {
		case tc.bad && err == nil:
			t.Errorf("%s: ScheduleToCron = %q, want an error", tc.name, got)
		case tc.bad && errors.Is(err, errParamMissing) != tc.missing:
			t.Errorf("%s: err = %v, missing-value error = %v, want %v", tc.name, err, errors.Is(err, errParamMissing), tc.missing)
		case !tc.bad && (err != nil || got != tc.want):
			t.Errorf("%s: ScheduleToCron = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

// Cron mode hands the expression to the parser AuraGo schedules with, so what the
// editor accepts is what Mission Control and the cron manager accept.
func TestScheduleToCronValidatesCron(t *testing.T) {
	berlinOK := true
	if _, err := time.LoadLocation("Europe/Berlin"); err != nil {
		berlinOK = false
	}
	valid := []string{
		"0 7 * * 1-5", "*/5 * * * *", "0 0 7 * * 1-5", "00 0 7 * * *", "0 0 1 1 *", "0 7 * * SUN", "0 7 * JAN MON-FRI",
		"59 23 31 12 6", "0 7 * * 0", "TZ=UTC 0 7 * * *", "CRON_TZ=UTC 0 0 7 * * *",
	}
	if berlinOK {
		valid = append(valid, "CRON_TZ=Europe/Berlin 0 7 * * *", "TZ=Europe/Berlin 0 0 7 * * 1-5")
	}
	for _, expr := range valid {
		got, err := ScheduleToCron(map[string]any{"mode": "cron", "cron": " " + expr + "  "})
		if err != nil || got != expr {
			t.Errorf("%q: ScheduleToCron = %q, %v; want it accepted unchanged", expr, got, err)
		}
	}

	invalid := []string{
		"99 99 * * *", "a b c d e", "0 7 * * 7", "60 * * * *", "* 24 * * *", "0 7 32 * *", "0 7 0 * *", "0 7 * 13 *", "0 7 * * 8",
		"*/0 * * * *", "5-1 * * * *", "0 7 * * MONDAY", "@daily", "@every 1s", "@every 1s a b c", "@hourly 1 2 3 4",
		"CRON_TZ=Mars/Olympus 0 7 * * *", "TZ=../../etc/passwd 0 7 * * *", "CRON_TZ=UTC 0 7 *",
		"CRON_TZ=UTC 0 0 0 7 * * *", "0 7 * * * * *",
	}
	for _, expr := range invalid {
		got, err := ScheduleToCron(map[string]any{"mode": "cron", "cron": expr})
		if err == nil {
			t.Errorf("%q: ScheduleToCron = %q, want an error", expr, got)
			continue
		}
		if errors.Is(err, errParamMissing) || len(err.Error()) > maxEchoMessageBytes || !utf8.ValidString(err.Error()) {
			t.Errorf("%q: error is a missing-value error, too long or not valid UTF-8: %q", expr, err.Error())
		}
	}

	// The message is the parser's, quoted and cut to 40 runes.
	_, err := ScheduleToCron(map[string]any{"mode": "cron", "cron": "99 99 * * *"})
	if err == nil || err.Error() != `a cron expression is not valid: "end of range (99) above maximum (59): 99"` {
		t.Errorf("message = %v", err)
	}
	_, err = ScheduleToCron(map[string]any{"mode": "cron", "cron": strings.Repeat("é", 90) + " b c d e"})
	if err == nil || !strings.HasPrefix(err.Error(), "a cron expression is not valid: \"") || !strings.HasSuffix(err.Error(), `…"`) {
		t.Errorf("a long parser message must be cut: %v", err)
	}
	// Hostile text cannot break the message into lines.
	_, err = ScheduleToCron(map[string]any{"mode": "cron", "cron": "0 7 * * \nFAKE\r\x00\xff x"})
	if err == nil || strings.ContainsAny(err.Error(), "\n\r\x00") || !utf8.ValidString(err.Error()) {
		t.Errorf("hostile expression: %q", err)
	}
}

// A schedule below one minute is for the cron manager, not for flows: the editor and
// Mission Control think in minutes.
func TestScheduleToCronRejectsSubMinuteSchedules(t *testing.T) {
	for _, expr := range []string{"30 0 7 * * *", "* * * * * *", "*/10 * * * * *", "0-30 * * * * *", "0,30 * * * * *", "1 * * * * *", "59 59 23 31 12 *"} {
		_, err := ScheduleToCron(map[string]any{"mode": "cron", "cron": expr})
		if err == nil || !strings.Contains(err.Error(), "at most once a minute") || errors.Is(err, errParamMissing) {
			t.Errorf("%q: err = %v, want a once-a-minute error", expr, err)
		}
	}
	for _, expr := range []string{"0 * * * * *", "00 * * * * *", "0-0 * * * * *", "* * * * *", "0 7 * * *"} {
		if got, err := ScheduleToCron(map[string]any{"mode": "cron", "cron": expr}); err != nil || got != expr {
			t.Errorf("%q: ScheduleToCron = %q, %v; want it accepted", expr, got, err)
		}
	}
}

// The parser slices a TZ= prefix without checking its bounds. validateCronExpr is
// called with at least five fields, so it cannot happen through ScheduleToCron, but a
// panic in a third-party parser must never reach the validator.
func TestValidateCronExprSurvivesParserPanics(t *testing.T) {
	for _, expr := range []string{"TZ=UTC", "CRON_TZ=UTC", "TZ=", "CRON_TZ=", "TZ", "", " ", "@", "@every", "TZ=\x00\xff"} {
		var err error
		if r := catchPanic(func() { err = validateCronExpr(expr) }); r != nil {
			t.Errorf("%q: validateCronExpr panicked: %v", expr, r)
		} else if err == nil {
			t.Errorf("%q: validateCronExpr accepted it", expr)
		}
	}
}

func TestScheduleToCronIntervalSteps(t *testing.T) {
	const wantMinutes = "minutes must divide 60 (1, 2, 3, 4, 5, 6, 10, 12, 15, 20, 30)"
	const wantHours = "hours must divide 24 (1, 2, 3, 4, 6, 8, 12)"
	for n := 1; n <= 59; n++ {
		got, err := ScheduleToCron(map[string]any{"mode": "interval_minutes", "minutes": float64(n)})
		if 60%n == 0 {
			if err != nil || got != "*/"+strconv.Itoa(n)+" * * * *" {
				t.Errorf("minutes %d = %q, %v", n, got, err)
			}
		} else if err == nil || err.Error() != wantMinutes {
			t.Errorf("minutes %d = %q, %v; want %q", n, got, err, wantMinutes)
		}
	}
	for n := 1; n <= 23; n++ {
		got, err := ScheduleToCron(map[string]any{"mode": "interval_hours", "hours": float64(n)})
		if 24%n == 0 {
			if err != nil || got != "0 */"+strconv.Itoa(n)+" * * *" {
				t.Errorf("hours %d = %q, %v", n, got, err)
			}
		} else if err == nil || err.Error() != wantHours {
			t.Errorf("hours %d = %q, %v; want %q", n, got, err, wantHours)
		}
	}
	for _, v := range []any{0.0, 60.0, 90.0, -5.0, 2.5, "x", true, 1e300, []any{5.0}, map[string]any{"a": 1.0}} {
		for mode, key := range map[string]string{"interval_minutes": "minutes", "interval_hours": "hours"} {
			if got, err := ScheduleToCron(map[string]any{"mode": mode, key: v}); err == nil || errors.Is(err, errParamMissing) {
				t.Errorf("%s %v = %q, %v; want an invalid-value error", mode, v, got, err)
			}
		}
	}
	// Hours 24 and minutes 60 are one period, which is what daily and hourly are for.
	if _, err := ScheduleToCron(map[string]any{"mode": "interval_hours", "hours": 24.0}); err == nil {
		t.Error("24 hours must be rejected")
	}
	// Missing and blank values mean the defaults: every 15 minutes, every hour.
	for _, v := range []any{nil, "", "  "} {
		if got, err := ScheduleToCron(map[string]any{"mode": "interval_minutes", "minutes": v}); err != nil || got != "*/15 * * * *" {
			t.Errorf("minutes %#v = %q, %v", v, got, err)
		}
		if got, err := ScheduleToCron(map[string]any{"mode": "interval_hours", "hours": v}); err != nil || got != "0 */1 * * *" {
			t.Errorf("hours %#v = %q, %v", v, got, err)
		}
	}
}

// Every expression the non-cron modes can produce must parse with AuraGo's parser and
// mean what the editor says: the checks read the parsed schedule's field bits.
func TestScheduleToCronOutputParses(t *testing.T) {
	convert := func(p map[string]any) string {
		t.Helper()
		expr, err := ScheduleToCron(p)
		if err != nil {
			t.Fatalf("ScheduleToCron(%v): %v", p, err)
		}
		return expr
	}
	allHours := rangeBits(0, 23, 1)
	allDays := rangeBits(1, 31, 1)
	allMonths := rangeBits(1, 12, 1)
	allDows := rangeBits(0, 6, 1)

	t.Run("minutes", func(t *testing.T) {
		for n := 1; n <= 59; n++ {
			p := map[string]any{"mode": "interval_minutes", "minutes": float64(n)}
			if 60%n != 0 {
				if _, err := ScheduleToCron(p); err == nil {
					t.Errorf("%d minutes is not an even step but was accepted", n)
				}
				continue
			}
			s := parseCron(t, convert(p))
			if s.Second != bitsOf(0) || s.Minute != rangeBits(0, 59, n) || s.Hour != allHours || s.Dom != allDays || s.Month != allMonths || s.Dow != allDows {
				t.Errorf("every %d minutes parsed as %+v", n, s)
			}
		}
	})

	t.Run("hours", func(t *testing.T) {
		for n := 1; n <= 23; n++ {
			p := map[string]any{"mode": "interval_hours", "hours": float64(n)}
			if 24%n != 0 {
				if _, err := ScheduleToCron(p); err == nil {
					t.Errorf("%d hours is not an even step but was accepted", n)
				}
				continue
			}
			s := parseCron(t, convert(p))
			if s.Second != bitsOf(0) || s.Minute != bitsOf(0) || s.Hour != rangeBits(0, 23, n) || s.Dom != allDays || s.Month != allMonths || s.Dow != allDows {
				t.Errorf("every %d hours parsed as %+v", n, s)
			}
		}
	})

	t.Run("days of the month", func(t *testing.T) {
		for day := 1; day <= 31; day++ {
			s := parseCron(t, convert(map[string]any{"mode": "monthly", "time": "07:00", "day": float64(day)}))
			if s.Second != bitsOf(0) || s.Minute != bitsOf(0) || s.Hour != bitsOf(7) || s.Dom != bitsOf(day) || s.Month != allMonths || s.Dow != allDows {
				t.Errorf("day %d parsed as %+v", day, s)
			}
		}
	})

	t.Run("every set of weekdays", func(t *testing.T) {
		for set := 1; set < 1<<len(weekdayNumbers); set++ {
			var names []any
			var nums []int
			// Last to first, so the order of the list is not what makes the result right.
			for i := len(weekdayNumbers) - 1; i >= 0; i-- {
				if set&(1<<i) != 0 {
					names = append(names, weekdayNumbers[i].key)
					nums = append(nums, weekdayNumbers[i].num)
				}
			}
			s := parseCron(t, convert(map[string]any{"mode": "weekly", "time": "08:15", "weekdays": names}))
			if s.Second != bitsOf(0) || s.Minute != bitsOf(15) || s.Hour != bitsOf(8) || s.Dom != allDays || s.Month != allMonths || s.Dow != bitsOf(nums...) {
				t.Errorf("weekdays %v parsed as %+v, want day-of-week bits %b", names, s, bitsOf(nums...))
			}
		}
	})

	t.Run("every clock time", func(t *testing.T) {
		count := 0
		for hour := 0; hour < 24; hour++ {
			for minute := 0; minute < 60; minute++ {
				clock := strconv.Itoa(hour/10) + strconv.Itoa(hour%10) + ":" + strconv.Itoa(minute/10) + strconv.Itoa(minute%10)
				daily := parseCron(t, convert(map[string]any{"mode": "daily", "time": clock}))
				if daily.Second != bitsOf(0) || daily.Minute != bitsOf(minute) || daily.Hour != bitsOf(hour) || daily.Dom != allDays || daily.Month != allMonths || daily.Dow != allDows {
					t.Fatalf("daily %s parsed as %+v", clock, daily)
				}
				weekdays := parseCron(t, convert(map[string]any{"mode": "weekdays", "time": clock}))
				if weekdays.Minute != bitsOf(minute) || weekdays.Hour != bitsOf(hour) || weekdays.Dow != rangeBits(1, 5, 1) {
					t.Fatalf("weekdays %s parsed as %+v", clock, weekdays)
				}
				count++
			}
		}
		if count != 1440 {
			t.Fatalf("checked %d clock times, want 1440", count)
		}
	})

	t.Run("defaults", func(t *testing.T) {
		if s := parseCron(t, convert(nil)); s.Minute != bitsOf(0) || s.Hour != bitsOf(7) {
			t.Errorf("the default schedule parsed as %+v", s)
		}
	})
}
