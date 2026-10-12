package persondetect

import (
	"fmt"
	"testing"
	"time"
	_ "time/tzdata"
)

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

func mustBerlin(t *testing.T) *time.Location {
	t.Helper()
	return mustLocation(t, "Europe/Berlin")
}

func utcTime(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
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
	if armed, next := EffectiveArmed(now, ArmMode("bogus"), nil); armed || !next.IsZero() {
		t.Fatalf("unknown mode = %v %v", armed, next)
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

// The server maps config weekdays ("mon".."sun") and "HH:MM" clocks to
// Window.Days (time.Weekday, Sunday = 0) and Window.From/To (minutes since
// midnight, 0..1439, end exclusive); the mapping itself is pinned by the server's
// mapping test. This pins what a Window means for those values.
func TestWindowWeekdayAndMinuteConventions(t *testing.T) {
	sunday := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for day := time.Sunday; day <= time.Saturday; day++ {
		at := sunday.AddDate(0, 0, int(day))
		window := Window{Days: []time.Weekday{day}, From: 11*60 + 30, To: 12*60 + 30}
		if !InWindow(at, window) {
			t.Errorf("%v 12:00 must be inside a %v window", day, day)
		}
		if InWindow(at.AddDate(0, 0, -1), window) || InWindow(at.AddDate(0, 0, 1), window) {
			t.Errorf("the neighbours of %v must be outside a %v-only window", day, day)
		}
	}

	monday := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	for _, minute := range []int{0, 1, 8*60 + 30, 12 * 60, 23*60 + 59} {
		start := monday.Add(time.Duration(minute) * time.Minute)
		// From is the first armed minute; To is exclusive, so From+1 ends a one-minute window.
		window := Window{Days: []time.Weekday{time.Monday}, From: minute, To: (minute + 1) % (24 * 60)}
		if !InWindow(start, window) {
			t.Errorf("minute %d must be the first minute of a window starting there", minute)
		}
		if InWindow(start.Add(-time.Minute), window) {
			t.Errorf("minute %d - 1 must be outside the window", minute)
		}
		if InWindow(start.Add(time.Minute), window) {
			t.Errorf("minute %d + 1 must be outside a one-minute window", minute)
		}
	}
}

// armFlip is one change of the armed state.
type armFlip struct {
	at    time.Time
	armed bool
}

func (f armFlip) String() string {
	return fmt.Sprintf("armed=%v@%s", f.armed, f.at.UTC().Format("2006-01-02T15:04:05Z"))
}

// armFlips evaluates ScheduleArmed every 10 s, like the service's arming loop,
// and returns the state at from and every change up to and including to.
func armFlips(from, to time.Time, windows []Window) (bool, []armFlip) {
	state := ScheduleArmed(from, windows)
	initial := state
	var flips []armFlip
	for at := from; !at.After(to); at = at.Add(10 * time.Second) {
		if now := ScheduleArmed(at, windows); now != state {
			flips = append(flips, armFlip{at: at, armed: now})
			state = now
		}
	}
	return initial, flips
}

type armingCase struct {
	name        string
	windows     []Window
	from        time.Time
	span        time.Duration
	wantInitial bool
	wantFlips   []armFlip
	// wantNext is the next change EffectiveArmed reports at from.
	wantNext time.Time
}

func runArmingCases(t *testing.T, cases []armingCase) {
	t.Helper()
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			initial, flips := armFlips(c.from, c.from.Add(c.span), c.windows)
			if initial != c.wantInitial {
				t.Errorf("armed at %v = %v, want %v", c.from, initial, c.wantInitial)
			}
			if len(flips) != len(c.wantFlips) {
				t.Fatalf("flips = %v, want %v", flips, c.wantFlips)
			}
			for i, want := range c.wantFlips {
				if !flips[i].at.Equal(want.at) || flips[i].armed != want.armed {
					t.Errorf("flip %d = %v armed=%v, want %v armed=%v", i, flips[i].at.UTC(), flips[i].armed, want.at.UTC(), want.armed)
				}
			}
			armed, next := EffectiveArmed(c.from, ArmSchedule, c.windows)
			if armed != c.wantInitial || !next.Equal(c.wantNext) {
				t.Errorf("EffectiveArmed(%v) = %v %v, want %v %v", c.from, armed, next.UTC(), c.wantInitial, c.wantNext.UTC())
			}
		})
	}
}

// A repeated autumn hour switches once, at its first occurrence; a skipped spring
// time switches at the jump. Evaluating the schedule every 10 s must therefore
// log exactly one change per boundary, and next_change must name that instant.
func TestArmingBoundariesInsideBerlinTransitionHours(t *testing.T) {
	berlin := mustBerlin(t)
	satNight := []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 2*60 + 30}}
	sunMorning := []Window{{Days: []time.Weekday{time.Sunday}, From: 2*60 + 30, To: 5 * 60}}
	fallFrom := time.Date(2026, 10, 25, 1, 30, 0, 0, berlin) // 01:30 CEST, before the repeated 02:00-03:00
	springFrom := time.Date(2026, 3, 29, 1, 30, 0, 0, berlin)
	runArmingCases(t, []armingCase{
		{
			name: "fall back: window ending in the repeated hour disarms once",
			// 02:30 CEST (first occurrence) = 00:30Z. The repeated 02:00-02:30 CET must not re-arm.
			windows: satNight, from: fallFrom, span: 5 * time.Hour,
			wantInitial: true, wantFlips: []armFlip{{utcTime(2026, 10, 25, 0, 30), false}},
			wantNext: utcTime(2026, 10, 25, 0, 30),
		},
		{
			name: "fall back: window starting in the repeated hour arms once",
			// Arms at 02:30 CEST = 00:30Z and ends 05:00 CET = 04:00Z; the second 02:30 changes nothing.
			windows: sunMorning, from: fallFrom, span: 5 * time.Hour,
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 10, 25, 0, 30), true}, {utcTime(2026, 10, 25, 4, 0), false}},
			wantNext: utcTime(2026, 10, 25, 0, 30),
		},
		{
			name:    "fall back: windows meeting inside the repeated hour stay armed",
			windows: append(append([]Window{}, satNight...), sunMorning...), from: fallFrom, span: 5 * time.Hour,
			wantInitial: true, wantFlips: []armFlip{{utcTime(2026, 10, 25, 4, 0), false}},
			wantNext: utcTime(2026, 10, 25, 4, 0),
		},
		{
			name: "spring forward: window starting in the skipped hour arms at the jump",
			// 02:30 does not exist; the jump is 03:00 CEST = 01:00Z. The window ends 03:10 CEST = 01:10Z.
			windows: []Window{{Days: []time.Weekday{time.Sunday}, From: 2*60 + 30, To: 3*60 + 10}}, from: springFrom, span: 4 * time.Hour,
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 3, 29, 1, 0), true}, {utcTime(2026, 3, 29, 1, 10), false}},
			wantNext: utcTime(2026, 3, 29, 1, 0),
		},
		{
			name:    "spring forward: window ending in the skipped hour disarms at the jump",
			windows: satNight, from: springFrom, span: 4 * time.Hour,
			wantInitial: true, wantFlips: []armFlip{{utcTime(2026, 3, 29, 1, 0), false}},
			wantNext: utcTime(2026, 3, 29, 1, 0),
		},
		{
			name: "spring forward: a window inside the skipped hour never arms",
			// The next occurrence is the following Sunday 02:10 CEST = 00:10Z, still within 8 days.
			windows: []Window{{Days: []time.Weekday{time.Sunday}, From: 2*60 + 10, To: 2*60 + 50}}, from: springFrom, span: 4 * time.Hour,
			wantInitial: false, wantFlips: nil,
			wantNext: utcTime(2026, 4, 5, 0, 10),
		},
	})

	// While the repeated hour is running, next_change keeps naming the same instants.
	for _, c := range []struct {
		name      string
		windows   []Window
		at        time.Time
		wantArmed bool
		wantNext  time.Time
	}{
		{"ending window, after the first 02:30", satNight, utcTime(2026, 10, 25, 0, 35), false, time.Date(2026, 10, 31, 22, 0, 0, 0, berlin)},
		{"ending window, in the repeated 02:05 CET", satNight, utcTime(2026, 10, 25, 1, 5), false, time.Date(2026, 10, 31, 22, 0, 0, 0, berlin)},
		{"starting window, in the repeated 02:05 CET", sunMorning, utcTime(2026, 10, 25, 1, 5), true, utcTime(2026, 10, 25, 4, 0)},
		{"starting window, at the second 02:30 CET", sunMorning, utcTime(2026, 10, 25, 1, 30), true, utcTime(2026, 10, 25, 4, 0)},
	} {
		armed, next := EffectiveArmed(c.at.In(berlin), ArmSchedule, c.windows)
		if armed != c.wantArmed || !next.Equal(c.wantNext) {
			t.Errorf("%s: EffectiveArmed(%v) = %v %v, want %v %v", c.name, c.at, armed, next.UTC(), c.wantArmed, c.wantNext.UTC())
		}
	}
}

// Santiago changes clocks at midnight: the autumn change repeats 23:00-24:00
// (Saturday evening) and the spring change skips 00:00-01:00 (Sunday).
func TestArmingMidnightTransitionsSantiago(t *testing.T) {
	santiago := mustLocation(t, "America/Santiago")
	springFrom := time.Date(2026, 9, 5, 22, 0, 0, 0, santiago) // 02:00Z; the jump is 2026-09-06 04:00Z
	fallFrom := time.Date(2026, 4, 4, 21, 0, 0, 0, santiago)   // 00:00Z; the change is 2026-04-05 03:00Z
	runArmingCases(t, []armingCase{
		{
			name:    "spring: window starting at the skipped midnight arms at the jump",
			windows: []Window{{Days: []time.Weekday{time.Sunday}, From: 0, To: 6 * 60}}, from: springFrom, span: 8 * time.Hour,
			// 06:00 -03 = 09:00Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 9, 6, 4, 0), true}, {utcTime(2026, 9, 6, 9, 0), false}},
			wantNext: utcTime(2026, 9, 6, 4, 0),
		},
		{
			name:    "spring: window ending at the skipped midnight disarms at the jump",
			windows: []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 0}}, from: springFrom, span: 8 * time.Hour,
			wantInitial: true, wantFlips: []armFlip{{utcTime(2026, 9, 6, 4, 0), false}},
			wantNext: utcTime(2026, 9, 6, 4, 0),
		},
		{
			name:    "spring: window ending 00:30 inside the skipped hour disarms at the jump",
			windows: []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 30}}, from: springFrom, span: 8 * time.Hour,
			wantInitial: true, wantFlips: []armFlip{{utcTime(2026, 9, 6, 4, 0), false}},
			wantNext: utcTime(2026, 9, 6, 4, 0),
		},
		{
			name:    "fall: window ending in the repeated 23:00-24:00 disarms once",
			windows: []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 23*60 + 30}}, from: fallFrom, span: 12 * time.Hour,
			// 22:00 -03 = 01:00Z; 23:30 -03 (first occurrence) = 02:30Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 4, 5, 1, 0), true}, {utcTime(2026, 4, 5, 2, 30), false}},
			wantNext: utcTime(2026, 4, 5, 1, 0),
		},
		{
			name:    "fall: window starting at midnight arms once the clock reaches it",
			windows: []Window{{Days: []time.Weekday{time.Sunday}, From: 0, To: 6 * 60}}, from: fallFrom, span: 12 * time.Hour,
			// Sunday 00:00 is first reached at 00:00 -04 = 04:00Z; 06:00 -04 = 10:00Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 4, 5, 4, 0), true}, {utcTime(2026, 4, 5, 10, 0), false}},
			wantNext: utcTime(2026, 4, 5, 4, 0),
		},
		{
			name:    "fall: consecutive full days stay armed across the change",
			windows: []Window{{Days: []time.Weekday{time.Saturday, time.Sunday}, From: 0, To: 0}}, from: fallFrom, span: 12 * time.Hour,
			wantInitial: true, wantFlips: nil,
			wantNext: utcTime(2026, 4, 6, 4, 0),
		},
	})
}

// Lord Howe shifts the clock by 30 minutes: spring skips 02:00-02:30, autumn
// repeats 01:30-02:00.
func TestArmingHalfHourTransitionsLordHowe(t *testing.T) {
	lordHowe := mustLocation(t, "Australia/Lord_Howe")
	springFrom := time.Date(2026, 10, 3, 21, 0, 0, 0, lordHowe) // 10:30Z; the jump is 15:30Z
	fallFrom := time.Date(2026, 4, 4, 21, 0, 0, 0, lordHowe)    // 10:00Z; the change is 15:00Z
	runArmingCases(t, []armingCase{
		{
			name:    "spring: window ending in the skipped half hour disarms at the jump",
			windows: []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 2*60 + 15}}, from: springFrom, span: 6 * time.Hour,
			// 22:00 +10:30 = 11:30Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 10, 3, 11, 30), true}, {utcTime(2026, 10, 3, 15, 30), false}},
			wantNext: utcTime(2026, 10, 3, 11, 30),
		},
		{
			name:    "spring: window starting in the skipped half hour arms at the jump",
			windows: []Window{{Days: []time.Weekday{time.Sunday}, From: 2*60 + 15, To: 5 * 60}}, from: springFrom, span: 10 * time.Hour,
			// 05:00 +11 = 18:00Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 10, 3, 15, 30), true}, {utcTime(2026, 10, 3, 18, 0), false}},
			wantNext: utcTime(2026, 10, 3, 15, 30),
		},
		{
			name:    "fall: window ending in the repeated half hour disarms once",
			windows: []Window{{Days: []time.Weekday{time.Saturday}, From: 22 * 60, To: 60 + 45}}, from: fallFrom, span: 8 * time.Hour,
			// 22:00 +11 = 11:00Z; 01:45 +11 (first occurrence) = 14:45Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 4, 4, 11, 0), true}, {utcTime(2026, 4, 4, 14, 45), false}},
			wantNext: utcTime(2026, 4, 4, 11, 0),
		},
		{
			name:    "fall: window starting in the repeated half hour arms once",
			windows: []Window{{Days: []time.Weekday{time.Sunday}, From: 60 + 45, To: 5 * 60}}, from: fallFrom, span: 10 * time.Hour,
			// 05:00 +10:30 = 18:30Z.
			wantInitial: false, wantFlips: []armFlip{{utcTime(2026, 4, 4, 14, 45), true}, {utcTime(2026, 4, 4, 18, 30), false}},
			wantNext: utcTime(2026, 4, 4, 14, 45),
		},
	})
}

func TestEffectiveArmedAdjacentWindowsAndWholeWeek(t *testing.T) {
	berlin := mustBerlin(t)
	// Adjacent windows leave no gap: nothing changes at the shared boundary.
	sameDay := []Window{
		{Days: []time.Weekday{time.Monday}, From: 20 * 60, To: 22 * 60},
		{Days: []time.Weekday{time.Monday}, From: 22 * 60, To: 23 * 60},
	}
	if armed, next := EffectiveArmed(utcTime(2026, 10, 19, 21, 0), ArmSchedule, sameDay); !armed || !next.Equal(utcTime(2026, 10, 19, 23, 0)) {
		t.Errorf("adjacent windows = %v %v, want armed until 23:00", armed, next)
	}
	overMidnight := []Window{
		{Days: []time.Weekday{time.Monday}, From: 22 * 60, To: 0},
		{Days: []time.Weekday{time.Tuesday}, From: 0, To: 2 * 60},
	}
	if armed, next := EffectiveArmed(utcTime(2026, 10, 19, 23, 0), ArmSchedule, overMidnight); !armed || !next.Equal(utcTime(2026, 10, 20, 2, 0)) {
		t.Errorf("windows meeting at midnight = %v %v, want armed until 02:00", armed, next)
	}

	// A schedule that covers the whole week never changes, also across the DST changes.
	allDays := []time.Weekday{0, 1, 2, 3, 4, 5, 6}
	for _, c := range []struct {
		name string
		at   time.Time
		w    Window
	}{
		{"utc, 24 h windows", utcTime(2026, 10, 19, 12, 0), Window{Days: allDays, From: 0, To: 0}},
		{"utc, windows from 10:00 to 10:00", utcTime(2026, 10, 19, 12, 0), Window{Days: allDays, From: 10 * 60, To: 10 * 60}},
		{"berlin autumn week", time.Date(2026, 10, 24, 12, 0, 0, 0, berlin), Window{Days: allDays, From: 0, To: 0}},
		{"berlin spring week", time.Date(2026, 3, 28, 12, 0, 0, 0, berlin), Window{Days: allDays, From: 2*60 + 30, To: 2*60 + 30}},
	} {
		if armed, next := EffectiveArmed(c.at, ArmSchedule, []Window{c.w}); !armed || !next.IsZero() {
			t.Errorf("%s: whole week = %v %v, want armed without a change", c.name, armed, next)
		}
	}

	// A change a week away is still reported: the horizon covers a full weekly period.
	oneMinute := []Window{{Days: []time.Weekday{time.Monday}, From: 8 * 60, To: 8*60 + 1}}
	at := utcTime(2026, 10, 19, 8, 1)
	armed, next := EffectiveArmed(at, ArmSchedule, oneMinute)
	if armed || !next.Equal(utcTime(2026, 10, 26, 8, 0)) || next.Sub(at) != 7*24*time.Hour-time.Minute {
		t.Errorf("weekly one-minute window = %v %v (in %v), want disarmed until next monday 08:00", armed, next, next.Sub(at))
	}
	// The mirror image: armed all week except Monday 08:00-08:01.
	allButOneMinute := []Window{
		{Days: []time.Weekday{time.Monday}, From: 8*60 + 1, To: 0},
		{Days: []time.Weekday{2, 3, 4, 5, 6, 0}, From: 0, To: 0},
		{Days: []time.Weekday{time.Monday}, From: 0, To: 8 * 60},
	}
	armed, next = EffectiveArmed(at, ArmSchedule, allButOneMinute)
	if !armed || !next.Equal(utcTime(2026, 10, 26, 8, 0)) || next.Sub(at) != 7*24*time.Hour-time.Minute {
		t.Errorf("armed all week but one minute = %v %v (in %v), want armed until next monday 08:00", armed, next, next.Sub(at))
	}
}
