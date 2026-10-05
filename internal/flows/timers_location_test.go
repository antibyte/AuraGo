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
