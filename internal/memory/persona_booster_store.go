package memory

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"aurago/internal/personabooster"
)

// personaBoosterSchema stores the booster state as one JSON row. Like the
// other personality tables it is created idempotently; the booster also runs
// with the personality engine off, so every access ensures the table.
const personaBoosterSchema = `
CREATE TABLE IF NOT EXISTS persona_booster_state (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	state_json TEXT NOT NULL,
	updated_at TEXT NOT NULL
);`

// personaBoosterMu serializes read-modify-write cycles of the turn tick and
// the background helper cue within this process.
var personaBoosterMu sync.Mutex

type personaBoosterDB interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) *sql.Row
}

func ensurePersonaBoosterTable(db personaBoosterDB) error {
	if _, err := db.Exec(personaBoosterSchema); err != nil {
		return fmt.Errorf("persona booster schema: %w", err)
	}
	return nil
}

// LoadPersonaBoosterState returns the stored state or a zero state.
func (s *SQLiteMemory) LoadPersonaBoosterState() (personabooster.State, error) {
	if err := ensurePersonaBoosterTable(s.db); err != nil {
		return personabooster.State{}, err
	}
	return loadPersonaBoosterStateWith(s.db)
}

// SavePersonaBoosterState replaces the stored state.
func (s *SQLiteMemory) SavePersonaBoosterState(state personabooster.State) error {
	if err := ensurePersonaBoosterTable(s.db); err != nil {
		return err
	}
	return savePersonaBoosterStateWith(s.db, state)
}

// UpdatePersonaBoosterState applies update to the stored state in one
// transaction and returns the saved result.
func (s *SQLiteMemory) UpdatePersonaBoosterState(update func(personabooster.State) personabooster.State) (personabooster.State, error) {
	if update == nil {
		return s.LoadPersonaBoosterState()
	}
	if err := ensurePersonaBoosterTable(s.db); err != nil {
		return personabooster.State{}, err
	}
	personaBoosterMu.Lock()
	defer personaBoosterMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return personabooster.State{}, fmt.Errorf("begin persona booster update: %w", err)
	}
	defer tx.Rollback()
	current, err := loadPersonaBoosterStateWith(tx)
	if err != nil {
		return personabooster.State{}, err
	}
	next := update(current)
	if err := savePersonaBoosterStateWith(tx, next); err != nil {
		return personabooster.State{}, err
	}
	if err := tx.Commit(); err != nil {
		return personabooster.State{}, fmt.Errorf("commit persona booster update: %w", err)
	}
	return next, nil
}

func loadPersonaBoosterStateWith(db personaBoosterDB) (personabooster.State, error) {
	var raw string
	err := db.QueryRow(`SELECT state_json FROM persona_booster_state WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return personabooster.State{}, nil
	}
	if err != nil {
		return personabooster.State{}, fmt.Errorf("load persona booster state: %w", err)
	}
	var state personabooster.State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		// A damaged row must not block turns; the booster starts fresh.
		return personabooster.State{}, nil
	}
	return state, nil
}

func savePersonaBoosterStateWith(db personaBoosterDB, state personabooster.State) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode persona booster state: %w", err)
	}
	_, err = db.Exec(
		`INSERT INTO persona_booster_state (id, state_json, updated_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET state_json = excluded.state_json, updated_at = excluded.updated_at`,
		string(raw), time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save persona booster state: %w", err)
	}
	return nil
}

func clearPersonaBoosterStateWith(db personaBoosterDB) error {
	if err := ensurePersonaBoosterTable(db); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM persona_booster_state`); err != nil {
		return fmt.Errorf("clear persona booster state: %w", err)
	}
	return nil
}
