package persondetect

import (
	"sort"
	"time"
)

// armingHorizonDays bounds the search for the next transition.
const armingHorizonDays = 8

func minutesOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

func windowHasDay(w Window, day time.Weekday) bool {
	for _, d := range w.Days {
		if d == day {
			return true
		}
	}
	return false
}

// InWindow reports whether the wall-clock time t (in t's location) lies in w.
func InWindow(t time.Time, w Window) bool {
	minute := minutesOfDay(t)
	today := t.Weekday()
	if w.From < w.To {
		return windowHasDay(w, today) && minute >= w.From && minute < w.To
	}
	yesterday := (today + 6) % 7
	return (windowHasDay(w, today) && minute >= w.From) || (windowHasDay(w, yesterday) && minute < w.To)
}

// ScheduleArmed reports whether t lies in any window.
func ScheduleArmed(t time.Time, windows []Window) bool {
	for _, w := range windows {
		if InWindow(t, w) {
			return true
		}
	}
	return false
}

// EffectiveArmed returns whether alerts are armed at now (wall clock of now's
// location) and the next instant this changes. The zero time means no change:
// modes on and off never change by themselves, and a schedule may not change
// within the next eight days.
func EffectiveArmed(now time.Time, mode ArmMode, windows []Window) (bool, time.Time) {
	switch mode {
	case ArmOn:
		return true, time.Time{}
	case ArmSchedule:
	default:
		return false, time.Time{}
	}
	armed := ScheduleArmed(now, windows)
	if len(windows) == 0 {
		return armed, time.Time{}
	}
	loc := now.Location()
	year, month, day := now.Date()
	var candidates []time.Time
	for offset := -1; offset <= armingHorizonDays; offset++ {
		weekday := time.Date(year, month, day+offset, 12, 0, 0, 0, loc).Weekday()
		for _, w := range windows {
			if !windowHasDay(w, weekday) {
				continue
			}
			endDay := day + offset
			if w.To <= w.From {
				endDay++
			}
			candidates = append(candidates,
				time.Date(year, month, day+offset, w.From/60, w.From%60, 0, 0, loc),
				time.Date(year, month, endDay, w.To/60, w.To%60, 0, 0, loc))
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	for _, candidate := range candidates {
		if candidate.After(now) && ScheduleArmed(candidate, windows) != armed {
			return armed, candidate
		}
	}
	return armed, time.Time{}
}
