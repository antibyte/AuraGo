package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (s *SQLiteMemory) preparePersonalityPrivacyTargets(ctx context.Context, sessionID string, deleteMemories bool) ([]privacyDeletionTarget, error) {
	marker := sessionPrivacyMarker(sessionID)
	targets := []privacyDeletionTarget{}
	if !deleteMemories {
		return targets, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT category,key FROM personality_profile_evidence WHERE session_hash=?`, marker)
	if err != nil {
		return nil, err
	}
	var keys [][2]string
	for rows.Next() {
		var key [2]string
		if err = rows.Scan(&key[0], &key[1]); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		var value, source string
		var confidence int
		err = s.db.QueryRowContext(ctx, `SELECT value,source,confidence FROM user_profile WHERE category=? AND key=?`, key[0], key[1]).Scan(&value, &source, &confidence)
		if err == sql.ErrNoRows {
			value, source = "", ""
		} else if err != nil {
			return nil, err
		}
		id, _ := json.Marshal(key)
		targets = append(targets, privacyDeletionTarget{Store: "personality_profile", TargetID: string(id), ExpectedFingerprint: privacyFingerprint(value, source, fmt.Sprint(confidence)), SourceMarker: marker, State: "pending"})
	}
	rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT kind,item_id FROM personality_text_sources WHERE session_hash=? AND kind IN ('emotion','character')`, marker)
	if err != nil {
		return nil, err
	}
	var textTargets []privacyDeletionTarget
	for rows.Next() {
		var kind, id string
		if err = rows.Scan(&kind, &id); err != nil {
			rows.Close()
			return nil, err
		}
		textTargets = append(textTargets, privacyDeletionTarget{Store: "personality_" + kind, TargetID: id, SourceMarker: marker, State: "pending"})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, target := range textTargets {
		target.ExpectedFingerprint, err = personalityTextFingerprint(s.db, target.Store, target.TargetID)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func personalityTextFingerprint(db personalityReader, store, id string) (string, error) {
	query := `SELECT json_object('text',text,'category',category,'confidence',confidence,'source',source,'protected',protected,'updated_at',updated_at) FROM character_notes WHERE id=?`
	if store == "personality_emotion" {
		query = `SELECT json_object('description',description,'cause',cause,'thought',inner_thought,'category',nudge_category,'source',source,'confidence',confidence,'timestamp',timestamp) FROM emotion_history WHERE id=?`
	}
	var content string
	err := db.QueryRow(query, id).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return privacyFingerprint(content), nil
}

func (s *SQLiteMemory) applyPersonalityPrivacyTarget(ctx context.Context, jobID string, target privacyDeletionTarget) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	retained := false
	switch target.Store {
	case "personality_profile":
		var key [2]string
		if err = json.Unmarshal([]byte(target.TargetID), &key); err != nil {
			return false, err
		}
		var value, source string
		var confidence int
		err = tx.QueryRowContext(ctx, `SELECT value,source,confidence FROM user_profile WHERE category=? AND key=?`, key[0], key[1]).Scan(&value, &source, &confidence)
		if err != nil && err != sql.ErrNoRows {
			return false, err
		}
		if err == nil && (source == "manual" || source == "memory_correction" || privacyFingerprint(value, source, fmt.Sprint(confidence)) != target.ExpectedFingerprint) {
			retained = true
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM personality_profile_evidence WHERE session_hash=? AND category=? AND key=?`, target.SourceMarker, key[0], key[1]); err != nil {
			return false, err
		}
		if err = rebuildPersonalityProfileTx(tx, key[0], key[1]); err != nil {
			return false, err
		}
		var remains bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM user_profile WHERE category=? AND key=?)`, key[0], key[1]).Scan(&remains); err != nil {
			return false, err
		}
		retained = retained || remains
	case "personality_emotion", "personality_character":
		fingerprint, err := personalityTextFingerprint(tx, target.Store, target.TargetID)
		if err != nil {
			return false, err
		}
		kind := "emotion"
		if target.Store == "personality_character" {
			kind = "character"
		}
		var shared bool
		independent := false
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM personality_text_sources WHERE kind=? AND item_id=? AND session_hash<>?)`, kind, target.TargetID, target.SourceMarker).Scan(&shared); err != nil {
			return false, err
		}
		if kind == "character" {
			var protected bool
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM character_notes WHERE id=? AND (protected=1 OR source='manual'))`, target.TargetID).Scan(&protected); err != nil {
				return false, err
			}
			independent = protected
		}
		independent = independent || (fingerprint != "" && fingerprint != target.ExpectedFingerprint)
		shared = shared || independent
		retained = shared
		if !shared {
			if kind == "emotion" {
				_, err = tx.ExecContext(ctx, `DELETE FROM emotion_history WHERE id=?`, target.TargetID)
			} else {
				_, err = tx.ExecContext(ctx, `DELETE FROM character_notes WHERE id=? AND protected=0 AND source<>'manual'`, target.TargetID)
			}
			if err != nil {
				return false, err
			}
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM personality_text_sources WHERE kind=? AND item_id=? AND session_hash=?`, kind, target.TargetID, target.SourceMarker); err != nil {
			return false, err
		}
		if independent {
			if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO personality_text_sources(kind,item_id,session_hash,turn_id) VALUES(?,?,'retained','')`, kind, target.TargetID); err != nil {
				return false, err
			}
		}
	default:
		return false, fmt.Errorf("unknown personality privacy store")
	}
	state, reason := "deleted", ""
	if retained {
		state, reason = PrivacyDeletionRetainedReview, "candidate_changed_or_shared"
	}
	if err = markPrivacyTargetStateTx(ctx, tx, jobID, target, state, reason); err != nil {
		return false, err
	}
	return !retained, tx.Commit()
}

func (s *SQLiteMemory) personalityPrivacyCounts(ctx context.Context, id string) (deleted, retained, legacy int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT deleted,retained,legacy_unknown FROM personality_privacy_counts WHERE job_id=?`, id).Scan(&deleted, &retained, &legacy)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}
