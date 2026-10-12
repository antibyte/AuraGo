package persondetect

import (
	"sort"
	"time"
)

// Arming is decided on instants, not on the wall clock. Each occurrence of a
// window (one per start day) becomes the half-open interval [start, end) between
// two instants, and a time is armed when it lies in any interval. A wall-clock
// time on a calendar day becomes an instant like this:
//
//   - a time that exists once is that instant;
//   - a repeated time (the autumn change shows an hour twice) is its first
//     occurrence, so a boundary inside the repeated hour switches once;
//   - a skipped time (the spring change jumps over an hour, or over midnight) is
//     the moment of the jump.
//
// Like the RTL-SDR scheduler (documentation/rtl-sdr.md), a repeated autumn hour
// counts once. A recording that falls into a skipped hour can be skipped, but a
// state cannot, so it takes effect at the jump. Evaluating the schedule every
// ArmCheckInterval therefore logs each boundary exactly once.

const (
	// armingHorizonDays is how far EffectiveArmed looks for the next change. A
	// schedule repeats every calendar week, so a state that can change at all
	// changes within seven days of now; the eighth day is slack for the hours a
	// clock change moves a boundary by. The one exception is a window that fits
	// entirely into a skipped hour: on that day it has no instants and the next
	// change is a week later, which may lie beyond the horizon and then reads as
	// "no change". The arming loop evaluates the schedule every ArmCheckInterval
	// anyway, so next_change is only an announcement.
	armingHorizonDays = 8

	// Occurrences are generated for start days from armingLookBackDays before to
	// armingLookAheadDays after the day of the evaluated time. An occurrence is at
	// most one calendar day long, so the day before already reaches into today;
	// the extra day on each side is slack for boundaries that a clock change
	// shifts across midnight.
	armingLookBackDays  = 2
	armingLookAheadDays = 1
)

// armingSpan is one occurrence of a window: armed in [start, end).
type armingSpan struct{ start, end time.Time }

func windowHasDay(w Window, day time.Weekday) bool {
	for _, d := range w.Days {
		if d == day {
			return true
		}
	}
	return false
}

// boundaryInstant maps the wall-clock time minute (minutes since midnight) on
// the calendar day year-month-day in loc to an instant, see the rules above.
func boundaryInstant(year int, month time.Month, day, minute int, loc *time.Location) time.Time {
	t := time.Date(year, month, day, minute/60, minute%60, 0, 0, loc)
	// The wall clock t shows, as seconds, against the wall clock that was asked for.
	_, offset := t.Zone()
	shown := t.Unix() + int64(offset)
	asked := time.Date(year, month, day, minute/60, minute%60, 0, 0, time.UTC).Unix()
	start, end := t.ZoneBounds()
	switch {
	case shown > asked:
		// Skipped and normalised forward, into the zone after the jump, which
		// starts at the jump.
		return start
	case shown < asked:
		// Skipped and normalised backward, into the zone before the jump, which
		// ends at the jump.
		return end
	}
	if !start.IsZero() {
		// time.Date does not say which occurrence of a repeated time it picked.
		// In the second one t is less than the offset difference behind the start
		// of its zone; the first occurrence is that much earlier.
		_, previous := start.Add(-time.Second).Zone()
		if delta := time.Duration(previous-offset) * time.Second; delta > 0 && t.Sub(start) < delta {
			return t.Add(-delta)
		}
	}
	return t
}

// armingSpans returns the occurrences of the windows that start on the calendar
// days firstDay..lastDay relative to the day of ref, in ref's location. An
// occurrence with To <= From ends on the following day, and To == From is one
// calendar day. Occurrences without instants (both boundaries fall into the same
// skipped hour) are left out.
func armingSpans(ref time.Time, windows []Window, firstDay, lastDay int) []armingSpan {
	loc := ref.Location()
	year, month, day := ref.Date()
	spans := make([]armingSpan, 0, (lastDay-firstDay+1)*len(windows))
	for offset := firstDay; offset <= lastDay; offset++ {
		weekday := time.Date(year, month, day+offset, 0, 0, 0, 0, time.UTC).Weekday()
		for _, w := range windows {
			if !windowHasDay(w, weekday) {
				continue
			}
			endOffset := offset
			if w.To <= w.From {
				endOffset++
			}
			start := boundaryInstant(year, month, day+offset, w.From, loc)
			end := boundaryInstant(year, month, day+endOffset, w.To, loc)
			if start.Before(end) {
				spans = append(spans, armingSpan{start: start, end: end})
			}
		}
	}
	return spans
}

// mergeArmingSpans sorts the spans and joins those that overlap or touch. The
// result is ordered and its spans are separated by gaps. It reuses spans.
func mergeArmingSpans(spans []armingSpan) []armingSpan {
	if len(spans) < 2 {
		return spans
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
	merged := spans[:1]
	for _, span := range spans[1:] {
		last := &merged[len(merged)-1]
		switch {
		case span.start.After(last.end):
			merged = append(merged, span)
		case span.end.After(last.end):
			last.end = span.end
		}
	}
	return merged
}

// InWindow reports whether the instant t lies in an occurrence of w, evaluated
// in t's location. A window's Days are the days it starts on; To < From
// continues into the next day, To == From covers 24 hours. A repeated autumn hour
// is armed or disarmed at its first occurrence only and a skipped spring time
// takes effect at the jump.
func InWindow(t time.Time, w Window) bool {
	return ScheduleArmed(t, []Window{w})
}

// ScheduleArmed reports whether the instant t lies in an occurrence of any
// window, with the rules of InWindow.
func ScheduleArmed(t time.Time, windows []Window) bool {
	for _, span := range armingSpans(t, windows, -armingLookBackDays, armingLookAheadDays) {
		if !t.Before(span.start) && t.Before(span.end) {
			return true
		}
	}
	return false
}

// EffectiveArmed returns whether alerts are armed at now (windows are read on the
// calendar of now's location) and the next instant this changes. The zero time
// means that no change happens within the next eight days: modes on and off never
// change by themselves, a schedule without windows never arms, and a schedule
// that covers or leaves out the whole week stays as it is. Windows that touch or
// overlap do not change the state where they meet.
func EffectiveArmed(now time.Time, mode ArmMode, windows []Window) (bool, time.Time) {
	if mode != ArmSchedule {
		return mode == ArmOn, time.Time{}
	}
	spans := mergeArmingSpans(armingSpans(now, windows, -armingLookBackDays, armingHorizonDays))
	horizon := now.AddDate(0, 0, armingHorizonDays)
	for _, span := range spans {
		switch {
		case now.Before(span.start):
			return false, changeBefore(span.start, horizon)
		case now.Before(span.end):
			return true, changeBefore(span.end, horizon)
		}
	}
	return false, time.Time{}
}

// changeBefore returns at when it lies before the horizon and the zero time
// otherwise. Spans are only generated up to the horizon, so a boundary beyond it
// may be an artifact of where the generation stopped.
func changeBefore(at, horizon time.Time) time.Time {
	if at.Before(horizon) {
		return at
	}
	return time.Time{}
}
