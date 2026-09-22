package agent

import (
	"time"

	"aurago/internal/config"
)

// computeNextMaintenanceRunCalendar computes the next local calendar
// occurrence of the configured maintenance wall clock. A disabled controller
// has no schedule. Date arithmetic uses AddDate so a DST transition does not
// turn a daily wall-clock schedule into a 23/25-hour interval.
//
// During a spring-forward gap, choose the first valid wall-clock minute after
// the requested time (02:30 becomes 03:00 in a one-hour gap). During a
// fall-back fold, choose the earlier occurrence when both instants exist.
func computeNextMaintenanceRunCalendar(cfg *config.Config, now time.Time) time.Time {
	if cfg == nil || !cfg.Maintenance.Enabled {
		return time.Time{}
	}
	hour, minute, err := parseTime(cfg.Maintenance.Time)
	if err != nil {
		hour, minute = 4, 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	location := now.Location()
	target := firstMaintenanceWallClock(now, hour, minute, location)
	if !now.Before(target) {
		tomorrow := now.AddDate(0, 0, 1)
		target = firstMaintenanceWallClock(tomorrow, hour, minute, location)
	}
	return target
}

// nextMaintenanceRunAfterDay returns the configured wall clock on the next
// local calendar day. It is used after a durable same-day claim rejects a
// rescheduled timer, so hot reloads cannot create another attempt later that
// day.
func nextMaintenanceRunAfterDay(cfg *config.Config, now time.Time) time.Time {
	if cfg == nil || !cfg.Maintenance.Enabled {
		return time.Time{}
	}
	hour, minute, err := parseTime(cfg.Maintenance.Time)
	if err != nil {
		hour, minute = 4, 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	return firstMaintenanceWallClock(now.AddDate(0, 0, 1), hour, minute, now.Location())
}

func firstMaintenanceWallClock(day time.Time, hour, minute int, location *time.Location) time.Time {
	year, month, date := day.Date()
	targetMinute := hour*60 + minute
	// Search instants around the local date instead of relying on time.Date's
	// normalization. This finds the first exact instant in a fold and the
	// first valid wall minute after a spring gap.
	center := time.Date(year, month, date, 12, 0, 0, 0, time.UTC)
	searchStart := center.Add(-36 * time.Hour)
	searchEnd := center.Add(36 * time.Hour)
	var exact, firstAfter time.Time
	firstAfterMinute := 24 * 60
	for instant := searchStart; instant.Before(searchEnd); instant = instant.Add(time.Minute) {
		local := instant.In(location)
		localYear, localMonth, localDate := local.Date()
		if localYear != year || localMonth != month || localDate != date || local.Second() != 0 || local.Nanosecond() != 0 {
			continue
		}
		wallMinute := local.Hour()*60 + local.Minute()
		switch {
		case wallMinute == targetMinute && exact.IsZero():
			exact = local
		case wallMinute > targetMinute && wallMinute < firstAfterMinute:
			firstAfter = local
			firstAfterMinute = wallMinute
		}
	}
	if !exact.IsZero() {
		return exact
	}
	if !firstAfter.IsZero() {
		return firstAfter
	}
	return time.Date(year, month, date, hour, minute, 0, 0, location)
}
