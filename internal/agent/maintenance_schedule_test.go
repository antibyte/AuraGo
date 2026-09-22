package agent

import (
	"testing"
	"time"

	"aurago/internal/config"
)

func TestComputeNextMaintenanceRunCalendarDisabledAndInvalid(t *testing.T) {
	cfg := &config.Config{}
	if got := computeNextMaintenanceRunCalendar(cfg, time.Date(2026, 6, 10, 3, 0, 0, 0, time.UTC)); !got.IsZero() {
		t.Fatalf("disabled schedule = %v, want zero", got)
	}
	cfg.Maintenance.Enabled = true
	cfg.Maintenance.Time = "invalid"
	got := computeNextMaintenanceRunCalendar(cfg, time.Date(2026, 6, 10, 3, 0, 0, 0, time.UTC))
	want := time.Date(2026, 6, 10, 4, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("invalid time schedule = %v, want %v", got, want)
	}
}

func TestComputeNextMaintenanceRunCalendarUsesCalendarDays(t *testing.T) {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("timezone unavailable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Maintenance.Enabled = true
	cfg.Maintenance.Time = "04:00"
	now := time.Date(2026, 3, 28, 5, 0, 0, 0, location)
	got := computeNextMaintenanceRunCalendar(cfg, now)
	want := time.Date(2026, 3, 29, 4, 0, 0, 0, location)
	if !got.Equal(want) || got.Format("2006-01-02 15:04 MST") != want.Format("2006-01-02 15:04 MST") {
		t.Fatalf("spring transition schedule = %v, want %v", got, want)
	}
	now = time.Date(2026, 10, 24, 5, 0, 0, 0, location)
	got = computeNextMaintenanceRunCalendar(cfg, now)
	want = time.Date(2026, 10, 25, 4, 0, 0, 0, location)
	if !got.Equal(want) || got.Format("2006-01-02 15:04 MST") != want.Format("2006-01-02 15:04 MST") {
		t.Fatalf("fall transition schedule = %v, want %v", got, want)
	}
}

func TestComputeNextMaintenanceRunCalendarUsesFirstValidSpringMinute(t *testing.T) {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("timezone unavailable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Maintenance.Enabled = true
	cfg.Maintenance.Time = "02:30"
	now := time.Date(2026, 3, 28, 5, 0, 0, 0, location)
	got := computeNextMaintenanceRunCalendar(cfg, now)
	want := time.Date(2026, 3, 29, 3, 0, 0, 0, location)
	if !got.Equal(want) || got.Format("2006-01-02 15:04 MST") != want.Format("2006-01-02 15:04 MST") {
		t.Fatalf("spring gap schedule = %v, want %v", got, want)
	}
}

func TestComputeNextMaintenanceRunCalendarUsesFirstValidNewYorkSpringMinute(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone unavailable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Maintenance.Enabled = true
	cfg.Maintenance.Time = "02:30"
	now := time.Date(2026, 3, 7, 5, 0, 0, 0, location)
	got := computeNextMaintenanceRunCalendar(cfg, now)
	want := time.Date(2026, 3, 8, 3, 0, 0, 0, location)
	if !got.Equal(want) || got.Format("2006-01-02 15:04 MST") != want.Format("2006-01-02 15:04 MST") {
		t.Fatalf("New York spring gap schedule = %v, want %v", got, want)
	}
}

func TestComputeNextMaintenanceRunCalendarChoosesDSTFoldFirstOccurrence(t *testing.T) {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("timezone unavailable: %v", err)
	}
	cfg := &config.Config{}
	cfg.Maintenance.Enabled = true
	cfg.Maintenance.Time = "02:30"
	now := time.Date(2026, 10, 24, 5, 0, 0, 0, location)
	got := computeNextMaintenanceRunCalendar(cfg, now)
	first := time.Date(2026, 10, 25, 2, 30, 0, 0, time.FixedZone("CEST", 2*60*60))
	if got.Format("2006-01-02 15:04") != "2026-10-25 02:30" {
		t.Fatalf("fold schedule wall clock = %v", got)
	}
	gotName, gotOffset := got.Zone()
	firstName, firstOffset := first.Zone()
	if !got.Equal(first) || gotName != firstName || gotOffset != firstOffset {
		t.Fatalf("fold schedule = %v (%s), want first occurrence %v (%s)", got, gotName, first, firstName)
	}
}
