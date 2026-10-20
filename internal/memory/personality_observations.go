package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
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
	var evidenceExists bool
	if err = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='personality_profile_evidence')`).Scan(&evidenceExists); err != nil {
		return err
	}
	if evidenceExists {
		return s.initPersonalityEvidence()
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
	return PersonalitySnapshot{Affect: projectPersonalityAffect(affect, now), Traits: traits, Dynamics: projectPersonalityDynamics(state, now).view(traits), Epoch: state.Epoch, Persona: state.Persona, TurnID: state.TurnID}
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
	if observation.Input.Valid() {
		observation.OccurredAt = observation.Input.OccurredAt
		if strings.TrimSpace(observation.Input.Text) == "" {
			observation.Human = false
			observation.ProfileUpdates = nil
		}
	} else {
		observation.Human = false
		observation.ProfileUpdates = nil
	}
	fingerprint := observation.requestHash
	if fingerprint == "" {
		fingerprint = observationFingerprint(observation)
	}
	if observation.OccurredAt.IsZero() {
		observation.OccurredAt = now
	}
	if !now.Before(observation.OccurredAt.Add(PersonalityObservationLifetime)) {
		return PersonalitySnapshot{}, ErrPersonalityEventExpired
	}
	if observation.OccurredAt.After(now.Add(5 * time.Minute)) {
		return PersonalitySnapshot{}, fmt.Errorf("personality event is in the future")
	}
	if observation.ID == "" {
		observation.ID = rand.Text()
	}
	key := personalityObservationKey("observation", observation.ID)
	tx, err := s.db.Begin()
	if err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("begin personality observation: %w", err)
	}
	defer tx.Rollback()
	if err = checkPersonalityInputTx(tx, observation); err != nil {
		return PersonalitySnapshot{}, err
	}
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
	var primary, semantic, relational bool
	var primaryHash, semanticHash string
	err = tx.QueryRow(`SELECT primary_applied,semantic_applied,relationship_applied,primary_hash,semantic_hash FROM personality_observations WHERE id=?`, key).Scan(&primary, &semantic, &relational, &primaryHash, &semanticHash)
	if err != nil && err != sql.ErrNoRows {
		return PersonalitySnapshot{}, fmt.Errorf("read personality observation receipt: %w", err)
	}
	if (observation.Semantic && semantic) || (!observation.Semantic && primary) {
		previousHash := primaryHash
		if observation.Semantic {
			previousHash = semanticHash
		}
		if previousHash != "" && previousHash != fingerprint {
			return PersonalitySnapshot{}, ErrPersonalityEventConflict
		}
		return makePersonalitySnapshot(current, state, traits, now), nil
	}
	if basis := observation.Basis; basis != nil {
		// Primary events integrate into the serialized current state so parallel
		// channels do not lose evidence. Only model-derived enrichment requires
		// the exact revision/turn it analyzed. Both reject reset/persona changes.
		if basis.Epoch != state.Epoch || basis.Persona != state.Persona ||
			(observation.Semantic && (basis.Dynamics.Revision != state.Revision || basis.TurnID != state.TurnID)) {
			return PersonalitySnapshot{}, ErrStalePersonalityObservation
		}
	}
	if observation.OperationalIssueOpen != nil && *observation.OperationalIssueOpen == state.OperationalIssueOpen {
		return makePersonalitySnapshot(current, state, traits, now), nil
	}
	if observation.Semantic {
		semanticHash = fingerprint
	} else {
		primaryHash = fingerprint
	}
	if now.Before(state.UpdatedAt) {
		now = state.UpdatedAt
	}
	next, updated, affinityDelta := integratePersonalityObservation(current, state, observation, relational, now)
	if observation.OperationalIssueOpen != nil {
		updated.OperationalIssueOpen = *observation.OperationalIssueOpen
	}
	updated.Revision++
	if observation.BeginTurn && observation.Human {
		updated.TurnID = observation.ID
		updated.TurnFamily = personalityStimulusKey(observation)
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
	if observation.LonelinessDelta != 0 && observation.Source == "inactivity" {
		if err = updateObservedTrait(tx, traits, TraitLoneliness, clampFinite(observation.LonelinessDelta, -1, 1, 0)); err != nil {
			return PersonalitySnapshot{}, err
		}
	}
	if err = applyPersonalityProfilesTx(tx, observation.Input, observation.ProfileUpdates); err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("apply personality profile evidence: %w", err)
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
		event.Valence = clampFinite(event.Valence, -1, 1, 0)
		event.Arousal = clampFinite(event.Arousal, 0, 1, AffectRestArousal)
		event.Weight = clampFinite(event.Weight, 0, 0.6, AffectDefaultWeight)
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
		emotion.DynamicsRevision, emotion.DynamicsEpoch = updated.Revision, updated.Epoch
		if err = insertEmotionStateHistoryWith(tx, emotion, next.CauseCode); err != nil {
			return PersonalitySnapshot{}, fmt.Errorf("record personality emotion: %w", err)
		}
		if observation.InnerThought != "" {
			if _, err = tx.Exec(`UPDATE emotion_history SET inner_thought=?,nudge_category=? WHERE id=last_insert_rowid()`, observation.InnerThought, observation.NudgeCategory); err != nil {
				return PersonalitySnapshot{}, fmt.Errorf("record personality inner voice: %w", err)
			}
		}
		var emotionID int64
		if err = tx.QueryRow(`SELECT last_insert_rowid()`).Scan(&emotionID); err != nil {
			return PersonalitySnapshot{}, err
		}
		if err = recordPersonalityTextSource(tx, "emotion", fmt.Sprint(emotionID), observation.Input); err != nil {
			return PersonalitySnapshot{}, err
		}
	}
	primary = primary || !observation.Semantic
	semantic = semantic || observation.Semantic
	relational = relational || relationshipObservation(observation)
	_, err = tx.Exec(`INSERT INTO personality_observations(id,primary_applied,semantic_applied,relationship_applied,created_at,occurred_at,primary_hash,semantic_hash) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET primary_applied=excluded.primary_applied,semantic_applied=excluded.semantic_applied,relationship_applied=excluded.relationship_applied,primary_hash=excluded.primary_hash,semantic_hash=excluded.semantic_hash`, key, primary, semantic, relational, now.UTC().Format(time.RFC3339Nano), observation.OccurredAt.UTC().Format(time.RFC3339Nano), primaryHash, semanticHash)
	if err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("save personality observation receipt: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return PersonalitySnapshot{}, fmt.Errorf("commit personality observation: %w", err)
	}
	s.invalidatePersonalityCaches()
	if observation.applied != nil {
		*observation.applied = true
	}
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
func (s *SQLiteMemory) SetPersonalityContext(persona string, meta PersonalityMeta, invalidate ...bool) error {
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
	if state.Persona != persona || !reflect.DeepEqual(state.Meta, meta.Normalized()) || (len(invalidate) > 0 && invalidate[0]) {
		if state.Persona != "" {
			state.Epoch++
			state.TurnID = ""
			state.PendingMood, state.PendingCount = "", 0
		}
		state.Persona = persona
		state.Revision++
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
