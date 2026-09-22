package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

var ErrStalePersonalityObservation = errors.New("personality observation was superseded")

const personalityDynamicsSchema = `
CREATE TABLE IF NOT EXISTS personality_dynamics (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	version INTEGER NOT NULL,
	state_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS personality_observations (
	id TEXT PRIMARY KEY,
	primary_applied INTEGER NOT NULL DEFAULT 0,
	semantic_applied INTEGER NOT NULL DEFAULT 0,
	relationship_applied INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_personality_observations_time ON personality_observations(created_at);
`

type personalityReader interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}

// InitPersonalityDynamics migrates additively, backing up existing on-disk
// personality data before the first migration. No historical text is replayed.
func (s *SQLiteMemory) InitPersonalityDynamics() error {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='personality_dynamics')`).Scan(&exists); err != nil {
		return fmt.Errorf("check personality dynamics migration: %w", err)
	}
	if exists {
		var version int
		err := s.db.QueryRow(`SELECT version FROM personality_dynamics WHERE id=1`).Scan(&version)
		if err == nil && version == PersonalityDynamicsVersion {
			return nil
		}
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("read personality dynamics version: %w", err)
		}
		if err == nil {
			return fmt.Errorf("unsupported personality dynamics version %d", version)
		}
	}
	if !exists {
		if err := s.backupPersonalityMigration(); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin personality dynamics migration: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(personalityDynamicsSchema); err != nil {
		return fmt.Errorf("create personality dynamics tables: %w", err)
	}
	if err = savePersonalityDynamics(tx, newPersonalityDynamics()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit personality dynamics migration: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) backupPersonalityMigration() error {
	var populated bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM mood_log) OR EXISTS(SELECT 1 FROM emotion_history) OR EXISTS(SELECT 1 FROM affect_state WHERE cause_code <> '') OR EXISTS(SELECT 1 FROM personality_traits WHERE trait <> 'loneliness' AND value <> 0.5)`).Scan(&populated); err != nil {
		return fmt.Errorf("inspect personality data before migration: %w", err)
	}
	if !populated {
		return nil
	}
	rows, err := s.db.Query(`PRAGMA database_list`)
	if err != nil {
		return fmt.Errorf("locate personality database: %w", err)
	}
	path := ""
	for rows.Next() {
		var seq int
		var name, file string
		if err = rows.Scan(&seq, &name, &file); err != nil {
			rows.Close()
			return fmt.Errorf("read personality database path: %w", err)
		}
		if name == "main" {
			path = file
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("read personality database list: %w", err)
	}
	if path == "" {
		return nil
	} // in-memory stores have no previous deployment
	backup := path + ".personality-dynamics-v1-" + rand.Text() + ".bak"
	if _, err = s.db.Exec(`VACUUM main INTO ?`, backup); err != nil {
		return fmt.Errorf("back up personality database before migration: %w", err)
	}
	if err = os.Chmod(backup, 0600); err != nil {
		return fmt.Errorf("protect personality migration backup: %w", err)
	}
	return nil
}

func loadPersonalityDynamics(db personalityReader) (personalityDynamicsState, error) {
	var data string
	state := newPersonalityDynamics()
	if err := db.QueryRow(`SELECT state_json FROM personality_dynamics WHERE id=1`).Scan(&data); err != nil {
		return state, fmt.Errorf("load personality dynamics: %w", err)
	}
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return state, fmt.Errorf("decode personality dynamics: %w", err)
	}
	if state.Version != PersonalityDynamicsVersion {
		return state, fmt.Errorf("unsupported personality dynamics state version %d", state.Version)
	}
	if state.Stimuli == nil {
		state.Stimuli = map[string]personalityStimulus{}
	}
	return state, nil
}

func savePersonalityDynamics(db affectExecer, state personalityDynamicsState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode personality dynamics: %w", err)
	}
	_, err = db.Exec(`INSERT INTO personality_dynamics(id,version,state_json) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,state_json=excluded.state_json`, PersonalityDynamicsVersion, string(data))
	if err != nil {
		return fmt.Errorf("save personality dynamics: %w", err)
	}
	return nil
}

func loadPersonalityAffect(db personalityReader) (AffectState, error) {
	var state AffectState
	var stamp string
	err := db.QueryRow(`SELECT valence,arousal,mood,cause_code,updated_at FROM affect_state WHERE id=1`).Scan(&state.Valence, &state.Arousal, &state.Mood, &state.CauseCode, &stamp)
	if err != nil {
		return state, fmt.Errorf("load personality affect: %w", err)
	}
	state.UpdatedAt, err = parseAffectTimestamp(stamp)
	if err != nil {
		return state, err
	}
	if state.CauseCode == "" {
		var mood Mood
		if err := db.QueryRow(`SELECT mood FROM mood_log ORDER BY timestamp DESC,id DESC LIMIT 1`).Scan(&mood); err == nil {
			state.Mood = mood
		}
	}
	return state, nil
}

func loadPersonalityTraits(db personalityReader) (PersonalityTraits, error) {
	rows, err := db.Query(`SELECT trait,value FROM personality_traits`)
	if err != nil {
		return nil, fmt.Errorf("load personality snapshot traits: %w", err)
	}
	defer rows.Close()
	traits := PersonalityTraits{}
	for rows.Next() {
		var key string
		var value float64
		if err = rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan personality trait: %w", err)
		}
		traits[key] = value
	}
	return traits, rows.Err()
}

func makePersonalitySnapshot(affect AffectState, state personalityDynamicsState, traits PersonalityTraits, now time.Time) PersonalitySnapshot {
	return PersonalitySnapshot{Affect: DecayAffect(affect, now), Traits: traits, Dynamics: projectPersonalityDynamics(state, now).view(traits), Epoch: state.Epoch, Persona: state.Persona, TurnID: state.TurnID}
}

// GetPersonalitySnapshotAt is a pure read. Polling does not advance decay,
// habituation, revisions, confirmation counts, or last-interaction timestamps.
func (s *SQLiteMemory) GetPersonalitySnapshotAt(now time.Time) (PersonalitySnapshot, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	s.affectMu.Lock()
	defer s.affectMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("begin personality snapshot: %w", err)
	}
	defer tx.Rollback()
	state, err := loadPersonalityDynamics(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	affect, err := loadPersonalityAffect(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	traits, err := loadPersonalityTraits(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	return makePersonalitySnapshot(affect, state, traits, now), nil
}

func (s *SQLiteMemory) invalidatePersonalityCaches() {
	s.personalityCacheMu.Lock()
	defer s.personalityCacheMu.Unlock()
	s.traitsCacheAt = time.Time{}
	s.moodCacheAt = time.Time{}
}

// ApplyPersonalityObservation is the sole transactional update path for
// observed affect, dynamics, bounded traits, mood and synthesized emotion.
func (s *SQLiteMemory) ApplyPersonalityObservation(observation PersonalityObservation) (PersonalitySnapshot, error) {
	s.affectMu.Lock()
	defer s.affectMu.Unlock()
	now := observation.At
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	if observation.ID == "" {
		observation.ID = rand.Text()
	}
	key := personalityObservationKey("observation", observation.ID)
	tx, err := s.db.Begin()
	if err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("begin personality observation: %w", err)
	}
	defer tx.Rollback()
	state, err := loadPersonalityDynamics(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	current, err := loadPersonalityAffect(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	traits, err := loadPersonalityTraits(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	if basis := observation.Basis; basis != nil && (basis.Dynamics.Revision != state.Revision || basis.Epoch != state.Epoch || basis.Persona != state.Persona || basis.TurnID != state.TurnID) {
		return PersonalitySnapshot{}, ErrStalePersonalityObservation
	}
	var primary, semantic, relational bool
	err = tx.QueryRow(`SELECT primary_applied,semantic_applied,relationship_applied FROM personality_observations WHERE id=?`, key).Scan(&primary, &semantic, &relational)
	if err != nil && err != sql.ErrNoRows {
		return PersonalitySnapshot{}, fmt.Errorf("read personality observation receipt: %w", err)
	}
	if (observation.Semantic && semantic) || (!observation.Semantic && primary) {
		return makePersonalitySnapshot(current, state, traits, now), nil
	}
	if now.Before(state.UpdatedAt) {
		now = state.UpdatedAt
	}
	next, updated, affinityDelta := integratePersonalityObservation(current, state, observation, relational, now)
	updated.Revision++
	if observation.BeginTurn && observation.Human {
		updated.TurnID = observation.ID
	}
	for trait, delta := range normalizeTraitMap(observation.TraitDeltas, -0.1, 0.1) {
		if trait == TraitAffinity || trait == TraitLoneliness {
			continue
		}
		if err = updateObservedTrait(tx, traits, trait, delta); err != nil {
			return PersonalitySnapshot{}, err
		}
	}
	if affinityDelta != 0 {
		if err = updateObservedTrait(tx, traits, TraitAffinity, affinityDelta); err != nil {
			return PersonalitySnapshot{}, err
		}
	}
	if err = saveAffectStateWith(tx, next); err != nil {
		return PersonalitySnapshot{}, err
	}
	if err = savePersonalityDynamics(tx, updated); err != nil {
		return PersonalitySnapshot{}, err
	}
	if observation.Event != nil && !observation.Semantic {
		event := *observation.Event
		// The new ledger stores codes and numeric state only, never conversation
		// fragments. The existing public event timeline remains compatible.
		event.Detail = ""
		if err = insertAffectEventWith(tx, event, now); err != nil {
			return PersonalitySnapshot{}, fmt.Errorf("record personality affect event: %w", err)
		}
	}
	if err = insertMoodLogWith(tx, next.Mood, next.CauseCode); err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("record personality mood: %w", err)
	}
	var emotion EmotionState
	if observation.Emotion != nil {
		emotion = *observation.Emotion
		emotion.Valence, emotion.Arousal, emotion.PrimaryMood, emotion.Timestamp = next.Valence, next.Arousal, next.Mood, now
		if err = insertEmotionStateHistoryWith(tx, emotion, next.CauseCode); err != nil {
			return PersonalitySnapshot{}, fmt.Errorf("record personality emotion: %w", err)
		}
	}
	primary = primary || !observation.Semantic
	semantic = semantic || observation.Semantic
	relational = relational || relationshipObservation(observation)
	_, err = tx.Exec(`INSERT INTO personality_observations(id,primary_applied,semantic_applied,relationship_applied,created_at) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET primary_applied=excluded.primary_applied,semantic_applied=excluded.semantic_applied,relationship_applied=excluded.relationship_applied`, key, primary, semantic, relational, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("save personality observation receipt: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("commit personality observation: %w", err)
	}
	s.invalidatePersonalityCaches()
	if observation.Emotion != nil {
		*observation.Emotion = emotion
	}
	return makePersonalitySnapshot(next, updated, traits, now), nil
}

func updateObservedTrait(db *sql.Tx, traits PersonalityTraits, trait string, delta float64) error {
	current := traits[trait]
	// Dampen only movements away from the midpoint, preserving recovery.
	if delta > 0 && current >= 0.5 {
		delta *= clampFinite((1-current)/0.5, 0, 1, 0)
	}
	if delta < 0 && current <= 0.5 {
		delta *= clampFinite(current/0.5, 0, 1, 0)
	}
	_, err := db.Exec(`UPDATE personality_traits SET value=MIN(COALESCE((SELECT ceiling FROM personality_trait_bounds WHERE trait=personality_traits.trait),1),MAX(COALESCE((SELECT floor FROM personality_trait_bounds WHERE trait=personality_traits.trait),0),value+?)),updated_at=CURRENT_TIMESTAMP WHERE trait=?`, delta, trait)
	if err != nil {
		return fmt.Errorf("update observed personality trait: %w", err)
	}
	if err = db.QueryRow(`SELECT value FROM personality_traits WHERE trait=?`, trait).Scan(&current); err != nil {
		return fmt.Errorf("read updated personality trait: %w", err)
	}
	traits[trait] = current
	return nil
}

// SetPersonalityContext invalidates outstanding semantic work when the active
// persona changes. The shared experience and familiarity intentionally survive.
func (s *SQLiteMemory) SetPersonalityContext(persona string, meta PersonalityMeta) error {
	s.affectMu.Lock()
	defer s.affectMu.Unlock()
	state, err := loadPersonalityDynamics(s.db)
	if err != nil {
		return err
	}
	persona = strings.TrimSpace(persona)
	if persona == "" {
		persona = "neutral"
	}
	if state.Persona != persona {
		state.Persona = persona
		state.Epoch++
		state.Revision++
		state.TurnID = ""
		state.PendingMood = ""
		state.PendingCount = 0
	}
	state.Meta = meta.Normalized()
	return savePersonalityDynamics(s.db, state)
}

// ResetPersonalityDynamics retains all traits, notes, history and the active
// persona, while invalidating in-flight results with a new epoch and revision.
func (s *SQLiteMemory) ResetPersonalityDynamics(now time.Time) (PersonalitySnapshot, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	s.affectMu.Lock()
	defer s.affectMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("begin personality reset: %w", err)
	}
	defer tx.Rollback()
	old, err := loadPersonalityDynamics(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	state := newPersonalityDynamics()
	state.Persona, state.Meta, state.Epoch, state.Revision, state.UpdatedAt = old.Persona, old.Meta, old.Epoch+1, old.Revision+1, now
	state.Recoveries, state.HumanEvents = old.Recoveries, old.HumanEvents
	affect := RestAffect(now)
	if err = savePersonalityDynamics(tx, state); err != nil {
		return PersonalitySnapshot{}, err
	}
	if err = saveAffectStateWith(tx, affect); err != nil {
		return PersonalitySnapshot{}, err
	}
	if err = insertMoodLogWith(tx, affect.Mood, "dynamics_reset"); err != nil {
		return PersonalitySnapshot{}, err
	}
	traits, err := loadPersonalityTraits(tx)
	if err != nil {
		return PersonalitySnapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("commit personality reset: %w", err)
	}
	s.invalidatePersonalityCaches()
	return makePersonalitySnapshot(affect, state, traits, now), nil
}
