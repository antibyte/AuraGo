package memory

import (
	"context"
	"fmt"
	"time"
)

func checkCorrectionProfileCapacity(ctx context.Context, q correctionQueryer, category string) error {
	var total, inCategory int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(category=?),0) FROM user_profile`, category).Scan(&total, &inCategory); err != nil {
		return err
	}
	if total >= maxTotalProfileEntries || inCategory >= maxProfileEntriesPerCategory {
		return fmt.Errorf("profile capacity reached; review entries before restoring an archived field")
	}
	return nil
}

func (s *SQLiteMemory) deleteProfileEntries(category, key string, all bool) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin profile deletion: %w", err)
	}
	defer tx.Rollback()
	// Serialize deletion with analysis and corrections before retaining the last value.
	if _, err = tx.Exec(`UPDATE user_profile SET key=key WHERE ? OR (category=? AND key=?)`, all, category, key); err != nil {
		return 0, fmt.Errorf("lock profile deletion: %w", err)
	}
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	if _, err = tx.Exec(`INSERT INTO personality_profile_deletions(category,key,deleted_at) SELECT category,key,? FROM user_profile WHERE ? OR (category=? AND key=?) ON CONFLICT(category,key) DO UPDATE SET deleted_at=excluded.deleted_at`, stamp, all, category, key); err != nil {
		return 0, err
	}
	if all {
		if _, err = tx.Exec(`INSERT INTO personality_profile_deletions(category,key,deleted_at) VALUES('*','*',?) ON CONFLICT(category,key) DO UPDATE SET deleted_at=excluded.deleted_at`, stamp); err != nil {
			return 0, err
		}
	}
	if _, err = tx.Exec(`INSERT INTO memory_correction_tombstones(reference,correction_id,previous_value)
		SELECT json_object('store','profile','category',p.category,'key',p.key),c.correction_id,p.value
		FROM user_profile p JOIN memory_profile_corrections c ON c.category=p.category AND c.key=p.key
		WHERE ? OR (p.category=? AND p.key=?)
		ON CONFLICT(reference) DO UPDATE SET correction_id=excluded.correction_id,previous_value=excluded.previous_value`, all, category, key); err != nil {
		return 0, fmt.Errorf("retain corrected profile field: %w", err)
	}
	result, err := tx.Exec(`DELETE FROM user_profile WHERE ? OR (category=? AND key=?)`, all, category, key)
	if err != nil {
		return 0, fmt.Errorf("delete profile entries: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM personality_profile_evidence WHERE ? OR (category=? AND key=?)`, all, category, key); err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted profile entries: %w", err)
	}
	return count, tx.Commit()
}
