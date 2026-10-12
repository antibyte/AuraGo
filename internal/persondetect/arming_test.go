package persondetect

import (
	"fmt"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"
)

func mustBerlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("load Europe/Berlin: %v", err)
	}
	return loc
}

func TestEffectiveArmedModes(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	if armed, next := EffectiveArmed(now, ArmOn, nil); !armed || !next.IsZero() {
		t.Fatalf("on = %v %v", armed, next)
	}
	if armed, next := EffectiveArmed(now, ArmOff, []Window{{Days: []time.Weekday{time.Wednesday}, From: 0, To: 0}}); armed || !next.IsZero() {
		t.Fatalf("off = %v %v", armed, next)
	}
	if armed, next := EffectiveArmed(now, ArmSchedule, nil); armed || !next.IsZero() {
		t.Fatalf("schedule without windows = %v %v", armed, next)
	}
}

func TestEffectiveArmedWeekdays(t *testing.T) {
	office := []Window{{Days: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}, From: 8 * 60, To: 17 * 60}}
	wednesdayNoon := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	armed, next := EffectiveArmed(wednesdayNoon, ArmSchedule, office)
	if !armed || !next.Equal(time.Date(2026, 10, 14, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("wednesday noon = %v %v", armed, next)
	}
	fridayEvening := time.Date(2026, 10, 16, 18, 0, 0, 0, time.UTC)
	armed, next = EffectiveArmed(fridayEvening, ArmSchedule, office)
	if armed || !next.Equal(time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("friday evening = %v %v, want disarmed until monday 08:00", armed, next)
	}
	if armed, _ := EffectiveArmed(time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC), ArmSchedule, office); armed {
		t.Fatal("saturday must be disarmed")
	}
	if armed, _ := EffectiveArmed(time.Date(2026, 10, 14, 17, 0, 0, 0, time.UTC), ArmSchedule, office); armed {
		t.Fatal("the end minute is exclusive")
	}
}

func TestEffectiveArmedMidnightCrossingAndFullDay(t *testing.T) {
	night := []Window{{Days: []time.Weekday{time.Friday}, From: 22 * 60, To: 6 * 60}}
	fridayLate := time.Date(2026, 10, 16, 23, 0, 0, 0, time.UTC)
	armed, next := EffectiveArmed(fridayLate, ArmSchedule, night)
	if !armed || !next.Equal(time.Date(2026, 10, 17, 6, 0, 0, 0, time.UTC)) {
		t.Fatalf("friday 23:00 = %v %v", armed, next)
	}
	if !InWindow(time.Date(2026, 10, 17, 5, 59, 0, 0, time.UTC), night[0]) {
		t.Fatal("saturday 05:59 belongs to friday's window")
	}
	if InWindow(time.Date(2026, 10, 17, 23, 0, 0, 0, time.UTC), night[0]) {
		t.Fatal("saturday 23:00 is not in a friday-only window")
	}
	fullMonday := Window{Days: []time.Weekday{time.Monday}, From: 0, To: 0}
	if !InWindow(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), fullMonday) || !InWindow(time.Date(2026, 10, 19, 23, 59, 0, 0, time.UTC), fullMonday) {
		t.Fatal("To == From covers the whole day")
	}
	if InWindow(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), fullMonday) {
		t.Fatal("the full-day window ends at midnight")
	}
}

func TestEffectiveArmedOverlappingWindows(t *testing.T) {
	windows := []Window{
		{Days: []time.Weekday{time.Monday}, From: 8 * 60, To: 12 * 60},
		{Days: []time.Weekday{time.Monday}, From: 10 * 60, To: 14 * 60},
	}
	armed, next := EffectiveArmed(time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC), ArmSchedule, windows)
	if !armed || !next.Equal(time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("overlap = %v %v, want armed until 14:00", armed, next)
	}
}

func TestEffectiveArmedAcrossDSTChanges(t *testing.T) {
	berlin := mustBerlin(t)
	night := []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 6 * 60}}

	// Spring forward: Sunday 2026-03-29, 02:00 CET -> 03:00 CEST.
	spring := time.Date(2026, 3, 29, 1, 30, 0, 0, berlin)
	armed, next := EffectiveArmed(spring, ArmSchedule, night)
	if !armed || !next.Equal(time.Date(2026, 3, 29, 4, 0, 0, 0, time.UTC)) || next.Sub(spring) != 3*time.Hour+30*time.Minute {
		t.Fatalf("spring = %v %v (in %v)", armed, next.UTC(), next.Sub(spring))
	}

	// Fall back: Sunday 2026-10-25, 03:00 CEST -> 02:00 CET.
	fall := time.Date(2026, 10, 25, 1, 30, 0, 0, berlin)
	armed, next = EffectiveArmed(fall, ArmSchedule, night)
	if !armed || !next.Equal(time.Date(2026, 10, 25, 5, 0, 0, 0, time.UTC)) || next.Sub(fall) != 5*time.Hour+30*time.Minute {
		t.Fatalf("fall = %v %v (in %v)", armed, next.UTC(), next.Sub(fall))
	}

	// After the window the next change is the next saturday 22:00 local time.
	afterwards := time.Date(2026, 10, 25, 7, 0, 0, 0, berlin)
	armed, next = EffectiveArmed(afterwards, ArmSchedule, night)
	if armed || !next.Equal(time.Date(2026, 10, 31, 22, 0, 0, 0, berlin)) {
		t.Fatalf("after the window = %v %v", armed, next)
	}
}

// The config package spells weekdays "sun".."sat" (config.ParseGo2RTCDetectionDay)
// and clocks "HH:MM" from 00:00 to 23:59 (config.ParseGo2RTCDetectionClock).
// persondetect must not import config, so the server mapping converts them to
// Window.Days (time.Weekday, Sunday = 0) and Window.From/To (minutes since
// midnight, end exclusive). This pins the conventions that mapping relies on.
func TestWindowWeekdayAndMinuteConventions(t *testing.T) {
	spellings := []struct {
		name string
		day  time.Weekday
	}{
		{"sun", time.Sunday}, {"mon", time.Monday}, {"tue", time.Tuesday}, {"wed", time.Wednesday},
		{"thu", time.Thursday}, {"fri", time.Friday}, {"sat", time.Saturday},
	}
	sunday := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for i, spelling := range spellings {
		at := sunday.AddDate(0, 0, i)
		if got := strings.ToLower(at.Format("Mon")); got != spelling.name {
			t.Fatalf("2026-10-%d is %q, want %q", 18+i, got, spelling.name)
		}
		if int(spelling.day) != i {
			t.Fatalf("%s = time.Weekday(%d), want %d", spelling.name, spelling.day, i)
		}
		window := Window{Days: []time.Weekday{spelling.day}, From: 11*60 + 30, To: 12*60 + 30}
		if !InWindow(at, window) {
			t.Errorf("%s 12:00 must be inside a %s window", spelling.name, spelling.name)
		}
		if InWindow(at.AddDate(0, 0, -1), window) || InWindow(at.AddDate(0, 0, 1), window) {
			t.Errorf("the neighbours of %s must be outside a %s-only window", spelling.name, spelling.name)
		}
	}

	clocks := []struct {
		text    string
		minutes int
	}{
		{"00:00", 0}, {"00:01", 1}, {"08:30", 8*60 + 30}, {"12:00", 12 * 60}, {"23:59", 23*60 + 59},
	}
	monday := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	for _, clock := range clocks {
		if got := fmt.Sprintf("%02d:%02d", clock.minutes/60, clock.minutes%60); got != clock.text {
			t.Fatalf("minute %d formats as %q, want %q", clock.minutes, got, clock.text)
		}
		start := monday.Add(time.Duration(clock.minutes) * time.Minute)
		// From is the first armed minute; To is exclusive, so From+1 ends a one-minute window.
		window := Window{Days: []time.Weekday{time.Monday}, From: clock.minutes, To: (clock.minutes + 1) % (24 * 60)}
		if !InWindow(start, window) {
			t.Errorf("%s must be the first minute of a window starting there", clock.text)
		}
		if InWindow(start.Add(time.Minute), window) {
			t.Errorf("%s + 1 minute must be outside a one-minute window", clock.text)
		}
	}
}
