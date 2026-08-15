package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const affectSchema = `
CREATE TABLE IF NOT EXISTS affect_state (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	valence REAL NOT NULL DEFAULT 0,
	arousal REAL NOT NULL DEFAULT 0.35,
	mood TEXT NOT NULL DEFAULT 'curious',
	cause_code TEXT NOT NULL DEFAULT '',
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS affect_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_type TEXT NOT NULL,
	cause_code TEXT NOT NULL,
	valence REAL,
	arousal REAL,
	weight REAL,
	source TEXT,
	detail TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_affect_events_time ON affect_events(created_at);
`

// InitAffectTables creates the singleton affect state and event log.
func (s *SQLiteMemory) InitAffectTables() error {
	if _, err := s.db.Exec(affectSchema); err != nil {
		return fmt.Errorf("affect schema: %w", err)
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO affect_state (id, valence, arousal, mood, cause_code) VALUES (1, ?, ?, ?, '')`,
		AffectRestValence, AffectRestArousal, string(DeriveMoodFromAffect(AffectRestValence, AffectRestArousal)),
	)
	if err != nil {
		return fmt.Errorf("seed affect state: %w", err)
	}
	return nil
}

// GetAffectState returns the persisted affect, decayed to now.
func (s *SQLiteMemory) GetAffectState() (AffectState, error) {
	return s.GetAffectStateAt(time.Now())
}

// GetAffectStateAt returns the persisted affect decayed to the given time.
func (s *SQLiteMemory) GetAffectStateAt(now time.Time) (AffectState, error) {
	if now.IsZero() {
		now = time.Now()
	}
	var (
		state     AffectState
		mood      string
		updatedAt string
	)
	err := s.db.QueryRow(
		`SELECT valence, arousal, mood, cause_code, updated_at FROM affect_state WHERE id = 1`,
	).Scan(&state.Valence, &state.Arousal, &mood, &state.CauseCode, &updatedAt)
	if err == sql.ErrNoRows {
		return RestAffect(now), nil
	}
	if err != nil {
		return AffectState{}, fmt.Errorf("get affect state: %w", err)
	}
	state.Mood = Mood(mood)
	if parsed, parseErr := parseAffectTimestamp(updatedAt); parseErr == nil {
		state.UpdatedAt = parsed
	}
	return DecayAffect(state, now), nil
}

// ApplyAffectEvent integrates one world/conversation event and persists the result.
func (s *SQLiteMemory) ApplyAffectEvent(event AffectEvent, now time.Time) (AffectState, error) {
	if now.IsZero() {
		if !event.At.IsZero() {
			now = event.At
		} else {
			now = time.Now()
		}
	}
	if strings.TrimSpace(event.CauseCode) == "" {
		return AffectState{}, fmt.Errorf("affect event cause is required")
	}

	current, err := s.loadRawAffectState()
	if err != nil {
		return AffectState{}, err
	}
	next := IntegrateAffect(current, event, now)
	if err := s.saveAffectState(next); err != nil {
		return AffectState{}, err
	}
	if err := s.insertAffectEvent(event, now); err != nil {
		return next, fmt.Errorf("persist affect event: %w", err)
	}
	if next.Mood != "" && next.Mood != current.Mood {
		_ = s.LogMood(next.Mood, event.CauseCode)
	}
	return next, nil
}

func (s *SQLiteMemory) loadRawAffectState() (AffectState, error) {
	var (
		state     AffectState
		mood      string
		updatedAt string
	)
	err := s.db.QueryRow(
		`SELECT valence, arousal, mood, cause_code, updated_at FROM affect_state WHERE id = 1`,
	).Scan(&state.Valence, &state.Arousal, &mood, &state.CauseCode, &updatedAt)
	if err == sql.ErrNoRows {
		return RestAffect(time.Now()), nil
	}
	if err != nil {
		return AffectState{}, fmt.Errorf("load affect state: %w", err)
	}
	state.Mood = Mood(mood)
	if parsed, parseErr := parseAffectTimestamp(updatedAt); parseErr == nil {
		state.UpdatedAt = parsed
	}
	return state, nil
}

func (s *SQLiteMemory) saveAffectState(state AffectState) error {
	_, err := s.db.Exec(
		`INSERT INTO affect_state (id, valence, arousal, mood, cause_code, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			valence = excluded.valence,
			arousal = excluded.arousal,
			mood = excluded.mood,
			cause_code = excluded.cause_code,
			updated_at = excluded.updated_at`,
		state.Valence,
		state.Arousal,
		string(state.Mood),
		state.CauseCode,
		state.UpdatedAt.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return fmt.Errorf("save affect state: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) insertAffectEvent(event AffectEvent, now time.Time) error {
	detail := strings.TrimSpace(event.Detail)
	if utf8.RuneCountInString(detail) > 200 {
		detail = string([]rune(detail)[:200])
	}
	_, err := s.db.Exec(
		`INSERT INTO affect_events (event_type, cause_code, valence, arousal, weight, source, detail, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.CauseCode,
		event.CauseCode,
		event.Valence,
		event.Arousal,
		event.Weight,
		strings.TrimSpace(event.Source),
		detail,
		now.UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// CleanupAffectEvents removes old event-log rows beyond a time or count cap.
func (s *SQLiteMemory) CleanupAffectEvents(maxAgeDays, maxEntries int) (int, error) {
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	if maxEntries <= 0 {
		maxEntries = 200
	}
	res, err := s.db.Exec(
		`DELETE FROM affect_events WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", maxAgeDays),
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup affect events by age: %w", err)
	}
	deleted, _ := res.RowsAffected()
	res, err = s.db.Exec(
		`DELETE FROM affect_events WHERE id NOT IN (
			SELECT id FROM affect_events ORDER BY created_at DESC, id DESC LIMIT ?
		)`,
		maxEntries,
	)
	if err != nil {
		return int(deleted), fmt.Errorf("cleanup affect events by count: %w", err)
	}
	extra, _ := res.RowsAffected()
	return int(deleted + extra), nil
}

func parseAffectTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsed affect timestamp %q", value)
}
