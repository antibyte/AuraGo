package memory

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const PersonalityObservationLifetime = 90 * 24 * time.Hour

var (
	ErrPersonalityEventExpired  = errors.New("personality event expired")
	ErrPersonalityEventConflict = errors.New("personality event conflicts with an accepted event")
)

// HumanInput is captured by a trusted ingress before enrichment or tool execution.
// Only its fingerprint, session marker and turn identity are persisted here.
type HumanInput struct {
	Text       string
	SessionID  string
	TurnID     string
	OccurredAt time.Time
}

// NewHumanInput is for authenticated human ingress only, before augmentation.
func NewHumanInput(text, sessionID string) *HumanInput {
	return &HumanInput{Text: text, SessionID: sessionID, TurnID: rand.Text(), OccurredAt: time.Now().UTC()}
}

// NewHumanInputEvent preserves a transport event's identity and original time.
// Event identifiers are opaque before they enter any persistent snapshot.
func NewHumanInputEvent(text, sessionID, eventID string, occurredAt time.Time) *HumanInput {
	input := NewHumanInput(text, sessionID)
	if eventID != "" {
		// Session rotation after deletion must not make a transport retry new evidence.
		input.TurnID = personalityObservationKey("human", eventID)
	}
	if !occurredAt.IsZero() {
		input.OccurredAt = occurredAt.UTC()
	}
	return input
}

func (h *HumanInput) Valid() bool {
	return h != nil && h.SessionID != "" && h.TurnID != "" && !h.OccurredAt.IsZero()
}

const personalityEvidenceSchema = `
CREATE TABLE IF NOT EXISTS deleted_chat_sessions (session_id TEXT PRIMARY KEY,deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,privacy_job_id TEXT NOT NULL DEFAULT '');
CREATE TABLE personality_profile_evidence (
 session_hash TEXT NOT NULL, turn_id TEXT NOT NULL, category TEXT NOT NULL,
 key TEXT NOT NULL, value TEXT NOT NULL, created_at TEXT NOT NULL,
 confirmations INTEGER NOT NULL DEFAULT 1 CHECK(confirmations=1),
 PRIMARY KEY(session_hash,turn_id,category,key)
);
CREATE INDEX personality_profile_evidence_key ON personality_profile_evidence(category,key,created_at);
CREATE TABLE personality_profile_deletions (category TEXT NOT NULL,key TEXT NOT NULL,deleted_at TEXT NOT NULL,PRIMARY KEY(category,key));
CREATE TRIGGER personality_profile_evidence_delete AFTER DELETE ON user_profile BEGIN DELETE FROM personality_profile_evidence WHERE category=OLD.category AND key=OLD.key; END;
CREATE TABLE personality_text_sources (
 kind TEXT NOT NULL, item_id TEXT NOT NULL, session_hash TEXT NOT NULL,
 turn_id TEXT NOT NULL, PRIMARY KEY(kind,item_id,session_hash,turn_id)
);
CREATE INDEX personality_text_sources_session ON personality_text_sources(session_hash);
CREATE TRIGGER personality_character_sources_delete AFTER DELETE ON character_notes BEGIN DELETE FROM personality_text_sources WHERE kind='character' AND item_id=CAST(OLD.id AS TEXT); END;
CREATE TRIGGER personality_emotion_sources_delete AFTER DELETE ON emotion_history BEGIN DELETE FROM personality_text_sources WHERE kind='emotion' AND item_id=CAST(OLD.id AS TEXT); END;
CREATE TABLE personality_legacy_review (
 id INTEGER PRIMARY KEY, kind TEXT NOT NULL, payload TEXT NOT NULL,
 fingerprint TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE personality_privacy_counts (
 job_id TEXT PRIMARY KEY REFERENCES privacy_deletion_jobs(deletion_job_id) ON DELETE CASCADE, deleted INTEGER NOT NULL DEFAULT 0,
 retained INTEGER NOT NULL DEFAULT 0, legacy_unknown INTEGER NOT NULL DEFAULT 0
);
`

// initPersonalityEvidence quarantines unproven text atomically, once per store.
func (s *SQLiteMemory) initPersonalityEvidence() error {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='personality_profile_evidence')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, column := range []string{"occurred_at", "primary_hash", "semantic_hash"} {
			var found bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('personality_observations') WHERE name=?)`, column).Scan(&found); err != nil {
				return err
			}
			if !found {
				if _, err := tx.Exec(`ALTER TABLE personality_observations ADD COLUMN ` + column + ` TEXT NOT NULL DEFAULT ''`); err != nil {
					return err
				}
			}
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		_, err = s.CleanupPersonalityObservations(time.Now())
		return err
	}
	if _, err := backupMainDatabase(s.db, "personality-evidence-v1"); err != nil {
		return fmt.Errorf("back up personality migration: %w", err)
	}
	// Rehearse the exact transactional migration on a protected consistent copy.
	probePath, err := backupMainDatabase(s.db, "personality-evidence-probe")
	if err != nil {
		return err
	}
	if probePath != "" {
		probe, openErr := sql.Open("sqlite", probePath)
		if openErr != nil {
			_ = os.Remove(probePath)
			return fmt.Errorf("open migration probe: %w", openErr)
		}
		probeErr := migratePersonalityEvidence(probe)
		closeErr := probe.Close()
		removeErr := os.Remove(probePath)
		if err := errors.Join(probeErr, closeErr, removeErr); err != nil {
			return fmt.Errorf("personality migration probe: %w", err)
		}
	}
	if err := migratePersonalityEvidence(s.db); err != nil {
		return err
	}
	_, err = s.CleanupPersonalityObservations(time.Now())
	return err
}

func migratePersonalityEvidence(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(personalityEvidenceSchema); err != nil {
		return fmt.Errorf("create personality evidence schema: %w", err)
	}
	for _, statement := range []string{
		`ALTER TABLE personality_observations ADD COLUMN occurred_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personality_observations ADD COLUMN primary_hash TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE personality_observations ADD COLUMN semantic_hash TEXT NOT NULL DEFAULT ''`,
		`INSERT INTO personality_legacy_review(kind,payload,fingerprint)
	 SELECT 'profile',json_object('category',category,'key',key,'value',value,'confidence',confidence,'source',source,'updated_at',updated_at),lower(hex(randomblob(32))) FROM user_profile WHERE source NOT IN ('manual','memory_correction')`,
		`DELETE FROM user_profile WHERE source NOT IN ('manual','memory_correction')`,
		`INSERT INTO personality_legacy_review(kind,payload,fingerprint)
	 SELECT 'character',json_object('category',category,'text',text,'confidence',confidence,'protected',protected,'source',source,'created_at',created_at,'updated_at',updated_at),lower(hex(randomblob(32))) FROM character_notes WHERE source<>'manual'`,
		`DELETE FROM character_notes WHERE source<>'manual'`,
		`INSERT INTO personality_legacy_review(kind,payload,fingerprint)
	 SELECT 'emotion',json_object('description',description,'cause',cause,'inner_thought',inner_thought,'trigger_summary',trigger_summary,'nudge_category',nudge_category,'source',source,'timestamp',timestamp,'primary_mood',primary_mood,'secondary_mood',secondary_mood,'valence',valence,'arousal',arousal,'confidence',confidence),lower(hex(randomblob(32))) FROM emotion_history WHERE COALESCE(description,'')<>'' OR COALESCE(inner_thought,'')<>''`,
		`DELETE FROM emotion_history`,
		`INSERT INTO personality_legacy_review(kind,payload,fingerprint)
	 SELECT 'mood',json_object('trigger_text',trigger_text),lower(hex(randomblob(32))) FROM mood_log WHERE COALESCE(trigger_text,'')<>''`,
		`UPDATE mood_log SET trigger_text=''`,
	} {
		if _, err = tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate personality evidence: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit personality evidence migration: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) CleanupPersonalityObservations(now time.Time) (int64, error) {
	// SQL narrows the candidates; Go checks the precise boundary before deletion.
	rows, err := s.db.Query(`SELECT id,COALESCE(NULLIF(occurred_at,''),created_at) FROM personality_observations WHERE julianday(COALESCE(NULLIF(occurred_at,''),created_at))<=julianday(?) ORDER BY created_at LIMIT 1000`, now.UTC().Add(-PersonalityObservationLifetime).Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("list expired personality receipts: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id, stamp string
		if err = rows.Scan(&id, &stamp); err != nil {
			break
		}
		at, e := parsePersonalityEventTime(stamp)
		if e != nil {
			err = e
			break
		}
		if !now.Before(at.Add(PersonalityObservationLifetime)) {
			ids = append(ids, id)
		}
	}
	err = errors.Join(err, rows.Err(), rows.Close())
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	result, err := s.db.Exec("DELETE FROM personality_observations WHERE id IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+")", args...)
	if err != nil {
		return 0, fmt.Errorf("clean personality observations: %w", err)
	}
	return result.RowsAffected()
}

func observationFingerprint(o PersonalityObservation) string {
	// Hash the submitted phase, before projection, cooldown or receipt defaults.
	if o.Input.Valid() {
		o.OccurredAt = o.Input.OccurredAt
		if strings.TrimSpace(o.Input.Text) == "" {
			o.Human = false
			o.ProfileUpdates = nil
		}
	} else {
		o.Human = false
		o.ProfileUpdates = nil
	}
	if o.Source == "feedback" && o.Input.Valid() {
		return privacyFingerprint(o.Source, o.Input.Text, o.Input.TurnID, o.Input.OccurredAt.UTC().Format(time.RFC3339Nano))
	}
	var emotion any
	if e := o.Emotion; e != nil {
		emotion = struct {
			Description, Secondary, Cause, Style, Source string
			Confidence                                   float64
		}{e.Description, e.SecondaryMood, e.Cause, e.RecommendedResponseStyle, e.Source, e.Confidence}
	}
	input := ""
	if o.Input.Valid() {
		input = privacyFingerprint(o.Input.Text + "\x00" + o.Input.SessionID + "\x00" + o.Input.TurnID)
	}
	data, _ := json.Marshal(struct {
		Source, Target, Signal, Mood, Input string
		Confidence, Affinity, Loneliness    float64
		Deltas                              map[string]float64
		Profiles                            []ProfileUpdate
		Emotion                             any
		Thought, Category                   string
		Event                               *AffectEvent
		Explicit, Human, BeginTurn          bool
		OperationalIssueOpen                *bool
		Occurred                            time.Time
	}{o.Source, o.Target, o.Signal, string(o.Mood), input, o.Confidence, o.AffinityDelta, o.LonelinessDelta, o.TraitDeltas, o.ProfileUpdates, emotion, o.InnerThought, o.NudgeCategory, o.Event, o.Explicit, o.Human, o.BeginTurn, o.OperationalIssueOpen, o.OccurredAt.UTC()})
	return privacyFingerprint(string(data))
}

func checkPersonalityInputTx(tx *sql.Tx, o PersonalityObservation) error {
	if !o.Input.Valid() {
		return nil
	}
	var deleted bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=?)`, o.Input.SessionID).Scan(&deleted); err != nil {
		return err
	}
	if deleted {
		return ErrSessionDeleted
	}
	return nil
}

func recordPersonalityTextSource(tx *sql.Tx, kind, id string, input *HumanInput) error {
	if !input.Valid() {
		return nil
	}
	_, err := tx.Exec(`INSERT OR IGNORE INTO personality_text_sources(kind,item_id,session_hash,turn_id) VALUES(?,?,?,?)`, kind, id, sessionPrivacyMarker(input.SessionID), privacyFingerprint(input.TurnID))
	return err
}

// PersonalityEventReceipt supports old clients only for already accepted IDs.
func (s *SQLiteMemory) PersonalityEventReceipt(id string, now time.Time) (bool, error) {
	var at, hash string
	err := s.db.QueryRow(`SELECT COALESCE(NULLIF(occurred_at,''),created_at),primary_hash FROM personality_observations WHERE id=?`, personalityObservationKey("observation", id)).Scan(&at, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	t, err := parsePersonalityEventTime(at)
	if err != nil {
		return false, err
	}
	if !now.Before(t.Add(PersonalityObservationLifetime)) {
		return false, ErrPersonalityEventExpired
	}
	return hash == "", nil
}

func parsePersonalityEventTime(value string) (time.Time, error) {
	for _, format := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if at, err := time.Parse(format, value); err == nil {
			return at, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid personality event timestamp")
}

// Keep administrative review separate from normal memory and model readers.
type PersonalityLegacyEntry struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Fingerprint string          `json:"fingerprint"`
}

func (s *SQLiteMemory) ListPersonalityLegacy(ctx context.Context, after int64, limit int) ([]PersonalityLegacyEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,kind,payload,fingerprint FROM personality_legacy_review WHERE id>? ORDER BY id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []PersonalityLegacyEntry{}
	for rows.Next() {
		var e PersonalityLegacyEntry
		var payload string
		if err = rows.Scan(&e.ID, &e.Kind, &payload, &e.Fingerprint); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		e.Fingerprint = privacyFingerprint(e.Kind, payload, e.Fingerprint)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
