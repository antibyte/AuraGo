package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

func applyPersonalityProfilesTx(tx *sql.Tx, input *HumanInput, updates []ProfileUpdate) error {
	if !input.Valid() || strings.TrimSpace(input.Text) == "" {
		return nil
	}
	for _, update := range updates {
		category, key, value := strings.ToLower(strings.TrimSpace(update.Category)), normalizeProfileKey(update.Key), normalizeProfileValue(update.Value)
		if !isValidProfileCategory(category) || !isValidProfileKey(key) || isRejectedProfileKey(key) || value == "" || containsProfilePII(value) || HasDurableCredential(key+"="+value) {
			continue
		}
		// The helper model writes the value and may have read a web page or mail
		// in the same turn. A profile value is a fact about the user, so a
		// standing order (StandingOrder) is no profile value, and the value has
		// to say what the human said this turn: a valid human turn proves
		// nothing about words the human never wrote.
		if _, directive := StandingOrder(strings.ReplaceAll(key, "_", " ") + ": " + value); directive || !DerivedFrom(value, input.Text) {
			continue
		}
		if utf8.RuneCountInString(value) > 200 {
			value = string([]rune(value)[:200])
		}
		var locked bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_profile_corrections WHERE category=? AND key=?) OR EXISTS(SELECT 1 FROM user_profile WHERE category=? AND key=? AND source IN ('manual','memory_correction'))`, category, key, category, key).Scan(&locked); err != nil {
			return err
		}
		if locked {
			continue
		}
		var deletion sql.NullString
		if err := tx.QueryRow(`SELECT MAX(deleted_at) FROM personality_profile_deletions WHERE (category=? AND key=?) OR (category='*' AND key='*')`, category, key).Scan(&deletion); err != nil {
			return err
		}
		if deletion.Valid {
			at, err := parsePersonalityEventTime(deletion.String)
			if err != nil {
				return err
			}
			if !input.OccurredAt.After(at) {
				continue
			}
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO personality_profile_evidence(session_hash,turn_id,category,key,value,created_at) VALUES(?,?,?,?,?,?)`, sessionPrivacyMarker(input.SessionID), privacyFingerprint(input.TurnID), category, key, value, input.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z")); err != nil {
			return err
		}
		if err := rebuildPersonalityProfileTx(tx, category, key); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM user_profile WHERE category=? AND source NOT IN ('manual','memory_correction') AND key NOT IN (SELECT key FROM user_profile WHERE category=? ORDER BY CASE WHEN source IN ('manual','memory_correction') THEN 0 ELSE 1 END,confidence DESC,updated_at DESC,key LIMIT ?)`, category, category, maxProfileEntriesPerCategory); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM user_profile WHERE source NOT IN ('manual','memory_correction') AND (category,key) NOT IN (SELECT category,key FROM user_profile ORDER BY CASE WHEN source IN ('manual','memory_correction') THEN 0 ELSE 1 END,confidence DESC,updated_at DESC,category,key LIMIT 50)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM personality_profile_evidence WHERE NOT EXISTS(SELECT 1 FROM user_profile p WHERE p.category=personality_profile_evidence.category AND p.key=personality_profile_evidence.key)`); err != nil {
			return err
		}
		// The extraction contract permits at most one durable update per turn.
		break
	}
	return nil
}

func rebuildPersonalityProfileTx(tx *sql.Tx, category, key string) error {
	var locked bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_profile_corrections WHERE category=? AND key=?) OR EXISTS(SELECT 1 FROM user_profile WHERE category=? AND key=? AND source IN ('manual','memory_correction'))`, category, key, category, key).Scan(&locked); err != nil {
		return err
	}
	if locked {
		return nil
	}
	var value string
	err := tx.QueryRow(`SELECT value FROM personality_profile_evidence WHERE category=? AND key=? ORDER BY created_at DESC,turn_id DESC LIMIT 1`, category, key).Scan(&value)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`DELETE FROM user_profile WHERE category=? AND key=? AND source NOT IN ('manual','memory_correction')`, category, key)
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO user_profile(category,key,value,confidence,source) VALUES(?,?,?,(SELECT SUM(confirmations) FROM personality_profile_evidence WHERE category=? AND key=? AND value=?),'v2')
	 ON CONFLICT(category,key) DO UPDATE SET value=excluded.value,confidence=excluded.confidence,source='v2',updated_at=CURRENT_TIMESTAMP`, category, key, value, category, key, value)
	return err
}

func (s *SQLiteMemory) ReviewPersonalityLegacy(id int64, fingerprint, action string) error {
	s.correctionMu.Lock()
	defer s.correctionMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var kind, payload, current string
	if err = tx.QueryRow(`SELECT kind,payload,fingerprint FROM personality_legacy_review WHERE id=?`, id).Scan(&kind, &payload, &current); err != nil {
		return fmt.Errorf("legacy entry unavailable: %w", err)
	}
	if fingerprint == "" || privacyFingerprint(kind, payload, current) != fingerprint {
		return ErrPersonalityEventConflict
	}
	var protection struct {
		Protected bool `json:"protected"`
	}
	// Legacy JSON uses SQLite's numeric boolean representation.
	var raw map[string]json.RawMessage
	if err = json.Unmarshal([]byte(payload), &raw); err != nil {
		return err
	}
	if value := raw["protected"]; string(value) == "1" || string(value) == "true" {
		protection.Protected = true
	}
	switch action {
	case "discard":
		if protection.Protected {
			return fmt.Errorf("protected legacy note must be adopted before it can be unprotected")
		}
	case "adopt":
		if err = adoptPersonalityLegacyTx(tx, kind, payload); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid legacy review action")
	}
	if _, err = tx.Exec(`DELETE FROM personality_legacy_review WHERE id=? AND fingerprint=?`, id, current); err != nil {
		return err
	}
	return tx.Commit()
}

func adoptPersonalityLegacyTx(tx *sql.Tx, kind, payload string) error {
	switch kind {
	case "profile":
		var p ProfileUpdate
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return err
		}
		p.Category, p.Key, p.Value = strings.ToLower(strings.TrimSpace(p.Category)), normalizeProfileKey(p.Key), normalizeProfileValue(p.Value)
		if !isValidProfileCategory(p.Category) || !isValidProfileKey(p.Key) || isRejectedProfileKey(p.Key) || p.Value == "" || containsProfilePII(p.Value) || HasDurableCredential(p.Key+"="+p.Value) || utf8.RuneCountInString(p.Value) > 200 {
			return fmt.Errorf("invalid legacy profile value")
		}
		var locked bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_profile_corrections WHERE category=? AND key=?)`, p.Category, p.Key).Scan(&locked); err != nil {
			return err
		}
		if locked {
			return ErrPersonalityEventConflict
		}
		var manualConflict bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_profile WHERE category=? AND key=? AND source IN ('manual','memory_correction') AND value<>?)`, p.Category, p.Key, p.Value).Scan(&manualConflict); err != nil {
			return err
		}
		if manualConflict {
			return ErrPersonalityEventConflict
		}
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_profile WHERE category=? AND key=?)`, p.Category, p.Key).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if err := checkCorrectionProfileCapacity(context.Background(), tx, p.Category); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO user_profile(category,key,value,confidence,source) VALUES(?,?,?,3,'manual') ON CONFLICT(category,key) DO UPDATE SET value=excluded.value,confidence=MAX(user_profile.confidence,3),source='manual',updated_at=CURRENT_TIMESTAMP`, p.Category, p.Key, p.Value)
		return err
	case "character":
		var p struct {
			Category, Text string
			Confidence     float64
			Protected      json.RawMessage
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return err
		}
		n := NormalizeCharacterNote(CharacterNote{Category: p.Category, Text: p.Text, Confidence: p.Confidence, Source: CharacterNoteSourceManual})
		if RejectCharacterNote(n.Text, "") {
			return fmt.Errorf("invalid legacy character note")
		}
		var rejected bool
		hash := characterNoteHash(n.Text)
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_note_rejections WHERE text_hash=?)`, hash).Scan(&rejected); err != nil {
			return err
		}
		if rejected {
			return ErrPersonalityEventConflict
		}
		protected := string(p.Protected) == "1" || string(p.Protected) == "true"
		_, err := tx.Exec(`INSERT INTO character_notes(category,text,text_hash,confidence,source,protected) VALUES(?,?,?,?,?,?) ON CONFLICT(text_hash) DO UPDATE SET source='manual',confidence=MAX(character_notes.confidence,excluded.confidence),protected=MAX(character_notes.protected,excluded.protected)`, n.Category, n.Text, hash, n.Confidence, n.Source, protected)
		return err
	default:
		return fmt.Errorf("only profile and character entries can be adopted")
	}
}
