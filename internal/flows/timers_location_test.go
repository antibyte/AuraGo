package flows

import (
	"context"
	"testing"
	"time"
)

// A yearly timer recurs at its local time of day in the service's zone. 2026-03-28 09:00
// in Berlin is still CET (08:00 UTC); 2027-03-28 is the day of the spring switch, so
// 09:00 that day is CEST (07:00 UTC). Without a zone the service keeps the UTC time of
// day, 08:00 UTC, which is 10:00 in Berlin.
func TestTimerServiceYearlyKeepsLocalTimeAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	fireAt := time.Date(2026, 3, 28, 9, 0, 0, 0, berlin)
	local := time.Date(2027, 3, 28, 9, 0, 0, 0, berlin)
	utcKept := fireAt.UTC().AddDate(1, 0, 0)
	if local.Sub(utcKept) != -time.Hour {
		t.Fatalf("test dates do not straddle a switch: %s vs %s", local.UTC(), utcKept)
	}
	cases := []struct {
		name string
		loc  *time.Location
		want time.Time
	}{
		{"Europe/Berlin", berlin, local},
		{"no zone keeps UTC", nil, utcKept},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Five minutes late at start-up is within MissedTimerGrace, so Start fires the
			// timer on its own goroutine and settles it before it returns.
			h := newHardTimers(t, fireAt.Add(5*time.Minute))
			h.arm(t, TimerRecord{NodeID: "n_aaaaaaaa", FireAt: fireAt, Repeat: RepeatYearly})
			fired := make(chan time.Time, 1)
			svc := h.service(t, func(_, _ string, at time.Time) { fired <- at }, nil, nil)
			if tc.loc != nil {
				svc.SetLocation(tc.loc)
			}
			if err := svc.Start(context.Background()); err != nil {
				t.Fatalf("Start: %v", err)
			}
			select {
			case at := <-fired:
				if !at.Equal(fireAt) {
					t.Fatalf("fired for %s, want %s", at, fireAt)
				}
			default:
				t.Fatal("the due timer did not fire during Start")
			}
			list := h.timers(t)
			if len(list) != 1 || !list[0].FireAt.Equal(tc.want) || list[0].Repeat != RepeatYearly {
				t.Fatalf("re-armed timers = %+v, want one at %s (%s)", list, tc.want.UTC(), tc.want.In(berlin))
			}
		})
	}
}

// tzRearm arms one yearly timer at fireAt, lets a timer service in loc fire it during
// Start (five minutes late, within the grace) and returns the time it re-armed it for.
func tzRearm(t *testing.T, loc *time.Location, fireAt time.Time) time.Time {
	t.Helper()
	h := newHardTimers(t, fireAt.Add(5*time.Minute))
	h.arm(t, TimerRecord{NodeID: "n_aaaaaaaa", FireAt: fireAt, Repeat: RepeatYearly})
	svc := h.service(t, func(string, string, time.Time) {}, nil, nil)
	svc.SetLocation(loc)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	list := h.timers(t)
	if len(list) != 1 || list[0].Repeat != RepeatYearly {
		t.Fatalf("re-armed timers = %+v", list)
	}
	return list[0].FireAt
}

// The edges of yearly timers in a zone with daylight saving time (Europe/Berlin).
func TestTimerServiceYearlyZoneEdges(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("tzdata missing")
	}
	local := func(tm time.Time) string { return tm.In(berlin).Format("2006-01-02 15:04 MST") }

	// Feb 29 00:30 local is Feb 28 in UTC; it recurs on the next local Feb 29.
	leap := time.Date(2028, 2, 29, 0, 30, 0, 0, berlin)
	if got := tzRearm(t, berlin, leap); local(got) != "2032-02-29 00:30 CET" {
		t.Fatalf("Feb 29 re-armed for %s", local(got))
	}

	// The spring-forward gap, the documented limit: 02:30 does not exist on 2027-03-28,
	// the timer moves to 03:30 and stays there the year after.
	gap := time.Date(2026, 3, 28, 2, 30, 0, 0, berlin)
	y2027 := tzRearm(t, berlin, gap)
	if local(y2027) != "2027-03-28 03:30 CEST" {
		t.Fatalf("gap year 2027 = %s", local(y2027))
	}
	if y2028 := tzRearm(t, berlin, y2027); local(y2028) != "2028-03-28 03:30 CEST" {
		t.Fatalf("gap year 2028 = %s", local(y2028))
	}

	// The autumn overlap keeps the wall-clock time: from an overlap day to an ordinary
	// day, and from an ordinary day to the next year's overlap day (2027-10-31), where
	// either of the two instants of 02:30 is acceptable.
	if got := tzRearm(t, berlin, time.Date(2026, 10, 25, 2, 30, 0, 0, berlin)); local(got) != "2027-10-25 02:30 CEST" {
		t.Fatalf("overlap day re-armed for %s", local(got))
	}
	got := tzRearm(t, berlin, time.Date(2026, 10, 31, 2, 30, 0, 0, berlin)).In(berlin)
	if got.Format("2006-01-02 15:04") != "2027-10-31 02:30" {
		t.Fatalf("re-armed into the overlap for %s", local(got))
	}
}
