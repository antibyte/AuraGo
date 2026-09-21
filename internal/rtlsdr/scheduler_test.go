package rtlsdr

import (
	"testing"
	"time"
)

func TestRTLSDRDSTWallTime(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	v := Schedule{Start: time.Date(2026, 3, 28, 2, 30, 0, 0, loc), Timezone: "Europe/Berlin", Repeat: "daily"}
	next := v.Next(v.Start)
	if next.Day() != 30 || next.Hour() != 2 || next.Minute() != 30 {
		t.Fatalf("spring gap not skipped: %v", next)
	}
	v.Start = time.Date(2026, 10, 24, 2, 30, 0, 0, loc)
	first := v.Next(v.Start)
	next = v.Next(first)
	if first.Day() != 25 || next.Day() != 26 {
		t.Fatalf("autumn duplicated: %v %v", first, next)
	}
	v.Start = time.Date(2026, 3, 22, 12, 0, 0, 0, loc)
	v.Repeat = "weekly"
	next = v.Next(v.Start)
	if next.Day() != 29 || next.Hour() != 12 || next.Sub(v.Start) != 167*time.Hour {
		t.Fatalf("weekly DST: %v", next)
	}
}
func TestRTLSDRScheduleConflictsIncludeWarmup(t *testing.T) {
	start := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	a := Schedule{Start: start, Duration: 600, Repeat: "daily", Timezone: "UTC", Enabled: true}
	b := a
	b.Start = start.Add(610 * time.Second)
	if !scheduleConflict(a, b) {
		t.Fatal("warmup collision missed")
	}
	b.Start = start.Add(630 * time.Second)
	if scheduleConflict(a, b) {
		t.Fatal("adjacent jobs rejected")
	}
	b.Repeat = "once"
	b.Start = start.AddDate(0, 0, 400).Add(20 * time.Second)
	if !scheduleConflict(a, b) {
		t.Fatal("distant once collision missed")
	}
	b.Enabled = false
	if scheduleConflict(a, b) {
		t.Fatal("disabled plan conflicts")
	}
}
