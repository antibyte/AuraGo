package rtlsdr

import "time"

// Next uses local wall-clock dates instead of adding 24 hours across DST.
// A missing spring-forward time is skipped; the repeated autumn hour runs once.
func (v Schedule) Next(after time.Time) time.Time {
	if v.Repeat == "once" {
		if v.Start.After(after) {
			return v.Start
		}
		return time.Time{}
	}
	loc, err := time.LoadLocation(v.Timezone)
	if err != nil {
		return time.Time{}
	}
	anchor := v.Start.In(loc)
	day := after.In(loc)
	if after.IsZero() || day.Before(anchor) {
		day = anchor
	}
	for i := 0; i < 9; i++ {
		date := day.AddDate(0, 0, i)
		if v.Repeat == "weekly" && date.Weekday() != anchor.Weekday() {
			continue
		}
		candidate := time.Date(date.Year(), date.Month(), date.Day(), anchor.Hour(), anchor.Minute(), anchor.Second(), 0, loc)
		if candidate.Hour() != anchor.Hour() || candidate.Minute() != anchor.Minute() {
			continue
		}
		if !candidate.Before(v.Start) && candidate.After(after) {
			return candidate
		}
	}
	return time.Time{}
}

func scheduleConflict(a, b Schedule) bool {
	if !a.Enabled || !b.Enabled {
		return false
	}
	base := a.Start
	if b.Start.After(base) {
		base = b.Start
	}
	// Include a complete DST cycle and the initial boundary of a later plan.
	end := base.AddDate(1, 0, 8)
	x := a.Next(base.Add(-MaxDuration - Warmup))
	y := b.Next(base.Add(-MaxDuration - Warmup))
	for !x.IsZero() && !y.IsZero() && !x.After(end) && !y.After(end) {
		xEnd := x.Add(time.Duration(a.Duration) * time.Second)
		yEnd := y.Add(time.Duration(b.Duration) * time.Second)
		if x.Add(-Warmup).Before(yEnd) && y.Add(-Warmup).Before(xEnd) {
			return true
		}
		if xEnd.Before(yEnd) {
			x = a.Next(x)
		} else {
			y = b.Next(y)
		}
	}
	return false
}

func (s *Service) SaveSchedule(v Schedule) (Schedule, error) {
	if err := s.writable(); err != nil {
		return v, err
	}
	if v.Agent && !s.policy().AllowAgent {
		return v, ErrDisabled
	}
	if v.Duration == 0 {
		v.Duration = 600
	}
	if err := v.Tuning.Validate(); err != nil {
		return v, err
	}
	if v.Timezone == "" {
		v.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(v.Timezone); err != nil {
		return v, ErrInvalid
	}
	if v.Repeat != "once" && v.Repeat != "daily" && v.Repeat != "weekly" {
		return v, ErrInvalid
	}
	if v.Start.IsZero() || v.Start.Before(s.opts.Now()) || v.Duration < 5 || v.Duration > int(MaxDuration/time.Second) || len(v.Name) > 120 || (v.Tuning.Mode == "dab" && v.Tuning.ServiceID == "") {
		return v, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return v, err
	}
	if v.Agent && !s.policy().AllowAgent {
		return v, ErrDisabled
	}
	if len(s.state.Schedules) >= 100 {
		return v, ErrQuota
	}
	for _, old := range s.state.Schedules {
		if scheduleConflict(v, old) {
			return v, ErrBusy
		}
	}
	for _, active := range s.state.Recordings {
		if active.ID == s.active && scheduleConflict(v, Schedule{Start: active.Start, Repeat: "once", Duration: active.Duration, Enabled: true}) {
			return v, ErrBusy
		}
	}
	v.ID = newID()
	v.LastSlot = time.Time{}
	v.NextStart = time.Time{}
	s.state.Schedules = append(s.state.Schedules, v)
	err := s.saveLocked()
	if err != nil {
		s.state.Schedules = s.state.Schedules[:len(s.state.Schedules)-1]
	}
	return v, err
}

func (s *Service) DeleteSchedule(id string) error {
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.state.Schedules {
		if v.ID == id {
			previous := append([]Schedule{}, s.state.Schedules...)
			s.state.Schedules = append(s.state.Schedules[:i], s.state.Schedules[i+1:]...)
			err := s.saveLocked()
			if err != nil {
				s.state.Schedules = previous
			}
			return err
		}
	}
	return ErrNotFound
}
