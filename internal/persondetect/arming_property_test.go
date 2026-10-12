package persondetect

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

// wallKey turns the local calendar date and time of an instant into one number
// that grows with the wall clock: days since the epoch times 1440, plus the
// minutes since midnight.
func wallKey(t time.Time) int64 {
	year, month, day := t.Date()
	days := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / 86400
	return days*1440 + int64(t.Hour()*60+t.Minute())
}

// armingOracle decides the arming schedule on one-minute ticks without the
// boundary arithmetic of arming.go. A window occurrence runs from the first tick
// at which the local wall clock has reached its start (calendar day and minute)
// to the first tick at which it has reached its end. That is the whole DST rule
// in one sentence: the clock reaches a repeated time the first time it shows it,
// and it reaches a skipped time when it jumps past it.
type armingOracle struct {
	loc     *time.Location
	start   time.Time
	maxKey  []int64 // running maximum of the wall key per tick: non-decreasing, so searchable
	flipIdx []int   // ticks whose state differs from the previous tick (set by paint)
}

func newArmingOracle(loc *time.Location, start time.Time, ticks int) *armingOracle {
	o := &armingOracle{loc: loc, start: start.In(loc), maxKey: make([]int64, ticks)}
	var highest int64
	for i := range o.maxKey {
		if key := wallKey(o.tick(i)); i == 0 || key > highest {
			highest = key
		}
		o.maxKey[i] = highest
	}
	return o
}

func (o *armingOracle) tick(i int) time.Time {
	return o.start.Add(time.Duration(i) * time.Minute)
}

// reach is the first tick whose wall clock is at or past key, or the tick count.
func (o *armingOracle) reach(key int64) int {
	return sort.Search(len(o.maxKey), func(i int) bool { return o.maxKey[i] >= key })
}

// paint returns the armed state of every tick for the schedule and records the
// ticks at which it changes.
func (o *armingOracle) paint(windows []Window) []bool {
	armed := make([]bool, len(o.maxKey))
	firstDay := o.maxKey[0]/1440 - 1
	lastDay := o.maxKey[len(o.maxKey)-1]/1440 + 1
	for day := firstDay; day <= lastDay; day++ {
		weekday := time.Unix(day*86400, 0).UTC().Weekday()
		for _, w := range windows {
			if !windowHasDay(w, weekday) {
				continue
			}
			endDay := day
			if w.To <= w.From {
				endDay++
			}
			end := o.reach(endDay*1440 + int64(w.To))
			for i := o.reach(day*1440 + int64(w.From)); i < end; i++ {
				armed[i] = true
			}
		}
	}
	o.flipIdx = o.flipIdx[:0]
	for i := 1; i < len(armed); i++ {
		if armed[i] != armed[i-1] {
			o.flipIdx = append(o.flipIdx, i)
		}
	}
	return armed
}

// nextChange is the first tick after tick i at which the state differs, or the
// zero time.
func (o *armingOracle) nextChange(i int) time.Time {
	k := sort.SearchInts(o.flipIdx, i+1)
	if k == len(o.flipIdx) {
		return time.Time{}
	}
	return o.tick(o.flipIdx[k])
}

// randomSchedule returns one to four windows on random days with boundaries
// biased towards the small hours, where the transitions happen.
func randomSchedule(r *rand.Rand) []Window {
	minute := func() int {
		switch r.Intn(10) {
		case 0, 1, 2, 3, 4: // 00:00-03:45 in quarter hours
			return 15 * r.Intn(16)
		case 5:
			return 0
		default:
			return r.Intn(24 * 60)
		}
	}
	windows := make([]Window, 1+r.Intn(4))
	for i := range windows {
		var days []time.Weekday
		for day := time.Sunday; day <= time.Saturday; day++ {
			if r.Intn(2) == 0 {
				days = append(days, day)
			}
		}
		if len(days) == 0 {
			days = []time.Weekday{time.Weekday(r.Intn(7))}
		}
		windows[i] = Window{Days: days, From: minute(), To: minute()}
		if r.Intn(8) == 0 {
			windows[i].To = windows[i].From // a 24 h window
		}
	}
	return windows
}

// TestArmingMatchesBruteForce scans nine days minute by minute and compares
// ScheduleArmed with the oracle, then compares EffectiveArmed (armed state and
// next_change) at the minutes around every change plus a few random ones.
func TestArmingMatchesBruteForce(t *testing.T) {
	const (
		schedules   = 6
		scanDays    = 9
		leadDays    = 3
		oracleTicks = (leadDays + scanDays + armingHorizonDays + 2) * 24 * 60
	)
	scenarios := []struct {
		name  string
		zone  string
		year  int
		month time.Month
		day   int
	}{
		{"utc", "UTC", 2026, time.October, 20},
		{"berlin fall", "Europe/Berlin", 2026, time.October, 22},
		{"berlin spring", "Europe/Berlin", 2026, time.March, 26},
		{"santiago fall", "America/Santiago", 2026, time.April, 2},
		{"santiago spring", "America/Santiago", 2026, time.September, 3},
		{"lord howe fall", "Australia/Lord_Howe", 2026, time.April, 2},
		{"lord howe spring", "Australia/Lord_Howe", 2026, time.October, 1},
	}
	for index, scenario := range scenarios {
		scenario, seed := scenario, int64(index+1)
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			loc := mustLocation(t, scenario.zone)
			base := time.Date(scenario.year, scenario.month, scenario.day, 0, 0, 0, 0, loc)
			oracle := newArmingOracle(loc, base.Add(-leadDays*24*time.Hour), oracleTicks)
			first, last := leadDays*24*60, (leadDays+scanDays)*24*60
			r := rand.New(rand.NewSource(seed))
			flipsNearTransition := 0
			for n := 0; n < schedules; n++ {
				windows := randomSchedule(r)
				armed := oracle.paint(windows)
				describe := func(i int) string {
					return fmt.Sprintf("%s (%s) with %v", oracle.tick(i).Format("Mon 2006-01-02 15:04 MST"), oracle.tick(i).UTC().Format("15:04Z"), windows)
				}

				for i := first; i < last; i++ {
					if got := ScheduleArmed(oracle.tick(i), windows); got != armed[i] {
						t.Fatalf("ScheduleArmed at %s = %v, want %v", describe(i), got, armed[i])
					}
				}

				probes := map[int]bool{first: true}
				for k := 0; k < 6; k++ {
					probes[first+r.Intn(last-first)] = true
				}
				for _, flip := range oracle.flipIdx {
					if flip < first || flip >= last {
						continue
					}
					probes[flip-1], probes[flip], probes[flip+1] = true, true, true
					_, transition := oracle.tick(flip).Add(-3 * time.Hour).ZoneBounds()
					if !transition.IsZero() && transition.Sub(oracle.tick(flip)).Abs() <= 3*time.Hour {
						flipsNearTransition++
					}
				}
				ordered := make([]int, 0, len(probes))
				for i := range probes {
					if i >= first && i < last {
						ordered = append(ordered, i)
					}
				}
				sort.Ints(ordered)
				for _, i := range ordered {
					now := oracle.tick(i)
					want := oracle.nextChange(i)
					if !want.IsZero() && !want.Before(now.AddDate(0, 0, armingHorizonDays)) {
						want = time.Time{} // beyond the horizon
					}
					gotArmed, gotNext := EffectiveArmed(now, ArmSchedule, windows)
					if gotArmed != armed[i] || !gotNext.Equal(want) {
						t.Fatalf("EffectiveArmed at %s = %v %s, want %v %s", describe(i), gotArmed, fmtInstant(gotNext), armed[i], fmtInstant(want))
					}
				}
			}
			if scenario.zone != "UTC" && flipsNearTransition == 0 {
				t.Error("the generated schedules never change near a clock change; adjust the seed so the test covers the transition hours")
			}
		})
	}
}

func fmtInstant(t time.Time) string {
	if t.IsZero() {
		return "none"
	}
	return t.UTC().Format("2006-01-02 15:04Z")
}
