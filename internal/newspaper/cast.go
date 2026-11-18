package newspaper

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const castGrace = 15 * time.Minute

type CastBooking struct {
	Key       string    `json:"key"`
	EditionID string    `json:"edition_id,omitempty"`
	Device    string    `json:"device"`
	Day       string    `json:"day"`
	Due       time.Time `json:"due"`
	Daily     bool      `json:"daily"`
	Status    string    `json:"status"`
}

type publicationWindow struct {
	clock string
	grace time.Duration
}

func (s *Service) publicationWindows(p Profile) []publicationWindow {
	var windows []publicationWindow
	if p.CastDaily && s.policy().Cast {
		windows = append(windows, publicationWindow{p.CastTime, castGrace})
	}
	if p.Daily || len(windows) == 0 {
		windows = append(windows, publicationWindow{p.ReadyTime, 3 * time.Hour})
	}
	return windows
}

// Empty date and time request immediate playback. A one-time booking always binds the selected revision.
func (s *Service) ScheduleCast(ctx context.Context, id, day, clock, key string) (CastBooking, error) {
	if !s.canWrite() || !s.policy().Cast || s.cast == nil {
		return CastBooking{}, ErrDisabled
	}
	p, err := s.store.Profile(ctx)
	if err != nil {
		return CastBooking{}, err
	}
	if p.CastDevice == "" || len(key) < 16 || len(key) > 128 {
		return CastBooking{}, errors.New("select a Chromecast and provide a request key")
	}
	e, err := s.store.Get(ctx, id)
	if err != nil {
		return CastBooking{}, err
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return CastBooking{}, err
	}
	now := s.now()
	due := now
	if day != "" || clock != "" {
		due, err = readyInstant(day, clock, loc)
		if err != nil {
			return CastBooking{}, errors.New("choose a future local playback time")
		}
	}
	key = "manual:" + key
	// A replay with the same key can arrive after the requested minute.
	if due.Before(now.Truncate(time.Minute)) {
		b, err := scanBooking(s.store.db.QueryRowContext(ctx, castSelect+" WHERE request_key=?", key))
		if err == nil && b.EditionID == id && b.Device == p.CastDevice && b.Due.Unix() == due.Unix() {
			return b, nil
		}
		return CastBooking{}, errors.New("choose a future local playback time")
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO newspaper_casts(request_key,edition_id,day,device,due,daily,status) VALUES(?,?,?,?,?,0,'pending') ON CONFLICT(request_key) DO NOTHING`, key, id, due.In(loc).Format("2006-01-02"), p.CastDevice, due.Unix())
	if err != nil {
		return CastBooking{}, err
	}
	b, err := scanBooking(s.store.db.QueryRowContext(ctx, castSelect+" WHERE request_key=?", key))
	if err != nil {
		return b, err
	}
	if b.EditionID != id || b.Device != p.CastDevice || (clock != "" && b.Due.Unix() != due.Unix()) {
		return b, ErrConflict
	}
	if b.Status == "pending" {
		_, _, _ = s.startAudio(ctx, e, true)
		s.wakeScheduler()
	}
	return b, nil
}

const castSelect = `SELECT request_key,COALESCE(edition_id,''),day,device,due,daily,status FROM newspaper_casts`

func scanBooking(row interface{ Scan(...any) error }) (CastBooking, error) {
	var b CastBooking
	var sec int64
	err := row.Scan(&b.Key, &b.EditionID, &b.Day, &b.Device, &sec, &b.Daily, &b.Status)
	b.Due = time.Unix(sec, 0).UTC()
	return b, err
}

func (s *Store) bookings(ctx context.Context, filter string, args ...any) ([]CastBooking, error) {
	rows, err := s.db.QueryContext(ctx, castSelect+filter, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CastBooking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DailyCast(ctx context.Context, day string) (*CastBooking, error) {
	b, err := scanBooking(s.db.QueryRowContext(ctx, castSelect+" WHERE request_key=?", "daily:"+day))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) CastBookings(ctx context.Context, id string) ([]CastBooking, error) {
	return s.bookings(ctx, ` WHERE edition_id=? OR (daily=1 AND edition_id IS NULL AND day=(SELECT local_date FROM newspaper_editions WHERE id=?)) ORDER BY due DESC LIMIT 20`, id, id)
}

func (s *Service) processCasts(p Profile) {
	now := s.now()
	ctx := s.ctx
	if p.CastDaily && s.policy().Cast && s.cast != nil {
		loc, err := time.LoadLocation(p.TimeZone)
		if err != nil {
			return
		}
		day := now.In(loc).Format("2006-01-02")
		due, err := readyInstant(day, p.CastTime, loc)
		if err != nil {
			return
		}
		_, err = s.store.db.ExecContext(ctx, `INSERT INTO newspaper_casts(request_key,day,device,due,daily,status) VALUES(?,?,?,?,1,'pending') ON CONFLICT(request_key) DO UPDATE SET device=excluded.device,due=excluded.due,status='pending' WHERE newspaper_casts.status='cancelled' OR (newspaper_casts.status='pending' AND (newspaper_casts.device<>excluded.device OR newspaper_casts.due<>excluded.due))`, "daily:"+day, day, p.CastDevice, due.Unix())
		if err != nil {
			return
		}
	} else {
		_, _ = s.store.db.ExecContext(ctx, `UPDATE newspaper_casts SET status='cancelled' WHERE daily=1 AND status='pending'`)
	}
	_, _ = s.store.db.ExecContext(ctx, `UPDATE newspaper_casts SET status='missed' WHERE status='pending' AND due<?`, now.Add(-castGrace).Unix())
	if !s.policy().Cast || s.cast == nil {
		return
	}
	bookings, err := s.store.bookings(ctx, ` WHERE status='pending' AND due<=? ORDER BY due LIMIT 16`, now.Unix())
	if err != nil {
		return
	}
	for _, b := range bookings {
		s.tryCast(b)
	}
}

func (s *Service) tryCast(b CastBooking) {
	ctx := s.ctx
	if b.Daily {
		err := s.store.db.QueryRowContext(ctx, `SELECT id FROM newspaper_editions WHERE local_date=? ORDER BY revision DESC LIMIT 1`, b.Day).Scan(&b.EditionID)
		if err != nil {
			return
		}
		_, err = s.store.db.ExecContext(ctx, `UPDATE newspaper_casts SET edition_id=? WHERE request_key=? AND status='pending'`, b.EditionID, b.Key)
		if err != nil {
			return
		}
	}
	e, err := s.store.Get(ctx, b.EditionID)
	if err != nil {
		return
	}
	_, a, err := s.startAudio(ctx, e, false)
	if err != nil || a.Status != "ready" {
		return
	}
	file, err := s.store.Recording(ctx, b.EditionID)
	if err != nil {
		return
	}
	work, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	started := false
	err = s.cast(work, b.Device, file, func() error {
		if work.Err() != nil || !s.canWrite() || !s.policy().Cast || s.now().After(b.Due.Add(castGrace)) {
			return ErrDisabled
		}
		profile, err := s.store.Profile(work)
		if err != nil {
			return err
		}
		if b.Daily && (!profile.CastDaily || profile.CastDevice != b.Device || profile.CastTime == "") {
			return ErrDisabled
		}
		if b.Daily {
			loc, err := time.LoadLocation(profile.TimeZone)
			if err != nil {
				return err
			}
			due, err := readyInstant(b.Day, profile.CastTime, loc)
			if err != nil || !due.Equal(b.Due) {
				return ErrConflict
			}
		}
		result, err := s.store.db.ExecContext(work, `UPDATE newspaper_casts SET status='starting' WHERE request_key=? AND status='pending' AND (daily=0 OR edition_id=(SELECT id FROM newspaper_editions WHERE local_date=? ORDER BY revision DESC LIMIT 1))`, b.Key, b.Day)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrConflict
		}
		started = true
		return nil
	})
	if !started {
		return
	} // Pre-LOAD errors can be retried inside the grace window.
	status := "played"
	if err != nil {
		status = "uncertain"
	}
	_, _ = s.store.db.Exec(`UPDATE newspaper_casts SET status=? WHERE request_key=? AND status='starting'`, status, b.Key)
}
